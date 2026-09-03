package launchpad

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Probe interface {
	Check(context.Context, Profile) (Snapshot, error)
}

type SystemProbe struct{}

// probeOverallBudget bounds the wall clock of a whole Check: independent
// probe groups run concurrently, so one slow helper command (for example a
// powershell child suspended by antivirus software on a consumer Windows
// machine) degrades into a recorded probe error instead of stretching the
// check into minutes with only a spinner for feedback.
var probeOverallBudget = 75 * time.Second

func (SystemProbe) Check(ctx context.Context, profile Profile) (Snapshot, error) {
	platform := detectPlatform()
	host, _ := os.Hostname()
	s := Snapshot{
		Timestamp:        time.Now().UTC(),
		Platform:         platform,
		Arch:             runtime.GOARCH,
		Hostname:         host,
		SessionTransport: detectSessionTransport(),
		PlatformDetails:  map[string]any{},
	}
	s.TargetUser, s.TargetUserIsAdmin = targetUserIdentity()
	s.IsAdministrator = detectAdministrator(ctx, platform)
	s.PackageManager = detectPackageManager()
	s.SSHClient = probeCapability(ctx, "ssh", "-V")

	probeCtx, cancelProbe := context.WithTimeout(ctx, probeOverallBudget)
	defer cancelProbe()
	var probeMu sync.Mutex
	addProbeError := func(format string, args ...any) {
		probeMu.Lock()
		defer probeMu.Unlock()
		s.ProbeErrors = append(s.ProbeErrors, fmt.Sprintf(format, args...))
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.SSHServer, s.SSHService, s.SSHPort = probeSSHServer(probeCtx, platform, profile)
		s.SSHConfigValid = probeSSHConfig(probeCtx, platform, s.SSHServer)
		effective := probeSSHEffectiveConfig(probeCtx, platform, s.SSHServer)
		if effective.Checked {
			// sshd -T parsed the complete effective configuration successfully. It
			// is a sufficient syntax check when an unprivileged probe cannot read
			// host private keys required by sshd -t.
			s.SSHConfigValid = true
			if path, err := resolveEffectiveAuthorizedKeysPath(effective.AuthorizedKeysFile, s); err == nil {
				s.SSHAuthorizedKeysFileChecked = true
				s.SSHAuthorizedKeysFile = path
			}
		}
		s.SSHAuthenticationChecked = effective.Checked
		s.SSHPasswordAuthentication = effective.PasswordAuthentication
		s.SSHKbdInteractiveAuthentication = effective.KbdInteractiveAuthentication
		s.SSHPubkeyAuthentication = effective.PubkeyAuthentication
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Network = probeNetwork(probeCtx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Tailscale = probeTailscale(probeCtx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		firewall, err := probeFirewall(probeCtx, platform, profile.SSH.Port)
		s.Firewall = firewall
		if err != nil {
			addProbeError("firewall: %v", err)
		}
	}()
	wg.Wait()
	if ctx.Err() == nil && probeCtx.Err() != nil {
		addProbeError("overall: %v; helper commands may be suspended by antivirus or security software", probeCtx.Err())
	}
	s.AuthorizedKeysChecked, s.AuthorizedKeysMatch, s.AuthorizedKeysCount = probeAuthorizedKeys(s, profile)
	var firewallErr error
	s.Firewall, firewallErr = probeFirewall(ctx, platform, profile.SSH.Port)
	if firewallErr != nil {
		s.ProbeErrors = append(s.ProbeErrors, "firewall: "+firewallErr.Error())
	}
	if platform == PlatformWSL {
		s.PlatformDetails["hostLayer"] = "wsl"
		s.Warnings = append(s.Warnings, "WSL is treated as a separate target; Windows host state was not inferred.")
	}
	if profile.Target.Platform != PlatformAuto && profile.Target.Platform != platform {
		s.Warnings = append(s.Warnings, fmt.Sprintf("profile targets %s but this process detected %s", profile.Target.Platform, platform))
	}
	if profile.Transport.Mode == "tailnet" && !s.Tailscale.Online {
		s.Warnings = append(s.Warnings, "Tailnet exposure is requested but Tailscale is not currently online.")
	}
	return s, nil
}

func detectPlatform() Platform {
	switch runtime.GOOS {
	case "windows":
		return PlatformWindows
	case "darwin":
		return PlatformMacOS
	default:
		if os.Getenv("WSL_INTEROP") != "" || strings.Contains(strings.ToLower(readSmallFile("/proc/version")), "microsoft") {
			return PlatformWSL
		}
		return PlatformLinux
	}
}

func detectSessionTransport() string {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return "ssh"
	}
	if os.Getenv("WT_SESSION") != "" {
		return "terminal"
	}
	return "local"
}

func detectPackageManager() string {
	for _, name := range []string{"winget", "brew", "apt-get", "dnf", "yum", "zypper", "pacman", "apk"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

func probeCapability(ctx context.Context, name string, args ...string) Capability {
	path, err := exec.LookPath(name)
	if err != nil {
		return Capability{}
	}
	out, _ := runCommand(ctx, 5*time.Second, path, args...)
	return Capability{Installed: true, Path: path, Version: strings.TrimSpace(string(out))}
}

func probeSSHServer(ctx context.Context, platform Platform, profile Profile) (Capability, ServiceState, int) {
	service := ServiceState{Name: profile.Advanced.LinuxSSHService}
	port := 0
	switch platform {
	case PlatformWindows:
		service.Name = profile.Advanced.WindowsSSHService
		script := `$s=Get-CimInstance Win32_Service -Filter "Name='sshd'" -ErrorAction SilentlyContinue; $l=Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object {$_.OwningProcess -in (Get-Process sshd -ErrorAction SilentlyContinue).Id} | Select-Object -First 1; [pscustomobject]@{installed=[bool]$s;running=($s.State -eq 'Running');startPolicy=$s.StartMode;port=$l.LocalPort;path=(Get-Command sshd.exe -ErrorAction SilentlyContinue).Source}|ConvertTo-Json -Compress`
		var v struct {
			Installed   bool   `json:"installed"`
			Running     bool   `json:"running"`
			StartPolicy string `json:"startPolicy"`
			Port        int    `json:"port"`
			Path        string `json:"path"`
		}
		if runJSON(ctx, &v, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script) == nil {
			return Capability{Installed: v.Installed, Path: v.Path}, ServiceState{Name: "sshd", Installed: v.Installed, Running: v.Running, StartPolicy: v.StartPolicy}, v.Port
		}
	case PlatformMacOS:
		service.Name = profile.Advanced.MacOSSSHLabel
		_, err := runCommand(ctx, 8*time.Second, "launchctl", "print", "system/com.openssh.sshd")
		path, pathErr := exec.LookPath("sshd")
		return Capability{Installed: pathErr == nil, Path: path}, ServiceState{Name: service.Name, Installed: pathErr == nil, Running: err == nil}, configuredSSHPort(ctx, path)
	default:
		name := profile.Advanced.LinuxSSHService
		if name == "" || name == "auto" {
			name = firstExistingService(ctx, "sshd", "ssh")
		}
		service.Name = name
		path, pathErr := exec.LookPath("sshd")
		active := commandSuccess(ctx, "systemctl", "is-active", "--quiet", name)
		enabled := commandSuccess(ctx, "systemctl", "is-enabled", "--quiet", name)
		port := 0
		if active {
			port = configuredSSHPort(ctx, path)
		}
		return Capability{Installed: pathErr == nil, Path: path}, ServiceState{Name: name, Installed: pathErr == nil, Running: active, StartPolicy: map[bool]string{true: "enabled", false: "disabled"}[enabled]}, port
	}
	return Capability{}, service, port
}

func configuredSSHPort(ctx context.Context, sshdPath string) int {
	if sshdPath == "" {
		return 0
	}
	out, err := runCommand(ctx, 8*time.Second, sshdPath, "-T")
	if err != nil {
		return 0
	}
	return parseConfiguredSSHPort(out)
}

type effectiveSSHConfig struct {
	Checked                      bool
	PasswordAuthentication       bool
	KbdInteractiveAuthentication bool
	PubkeyAuthentication         bool
	AuthorizedKeysFile           string
}

// probeSSHEffectiveConfig dumps the global effective sshd configuration with
// a single sshd -T run. Per-connection Match blocks (sshd -T -C ...) are not
// evaluated: hosts whose authentication policy depends on Match criteria are
// treated as unchecked, which fails closed. See docs/platform-support.md.
func probeSSHEffectiveConfig(ctx context.Context, platform Platform, server Capability) effectiveSSHConfig {
	if !server.Installed {
		return effectiveSSHConfig{}
	}
	path := server.Path
	if path == "" {
		path, _ = exec.LookPath("sshd")
	}
	if path == "" {
		return effectiveSSHConfig{}
	}
	args := []string{"-T"}
	if platform == PlatformWindows {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		args = append(args, "-f", filepath.Join(programData, "ssh", "sshd_config"))
	}
	out, err := runCommand(ctx, 8*time.Second, path, args...)
	if err != nil {
		return effectiveSSHConfig{}
	}
	return parseEffectiveSSHConfig(out)
}

func parseEffectiveSSHConfig(out []byte) effectiveSSHConfig {
	values := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			values[strings.ToLower(fields[0])] = strings.Join(fields[1:], " ")
		}
	}
	password, hasPassword := values["passwordauthentication"]
	keyboard, hasKeyboard := values["kbdinteractiveauthentication"]
	pubkey, hasPubkey := values["pubkeyauthentication"]
	return effectiveSSHConfig{
		Checked:                      hasPassword && hasKeyboard && hasPubkey,
		PasswordAuthentication:       strings.EqualFold(password, "yes"),
		KbdInteractiveAuthentication: strings.EqualFold(keyboard, "yes"),
		PubkeyAuthentication:         strings.EqualFold(pubkey, "yes"),
		AuthorizedKeysFile:           values["authorizedkeysfile"],
	}
}

func parseConfiguredSSHPort(out []byte) int {
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "port" {
			if port, parseErr := strconv.Atoi(fields[1]); parseErr == nil {
				return port
			}
		}
	}
	return 0
}

func firstExistingService(ctx context.Context, names ...string) string {
	for _, name := range names {
		out, err := runCommand(ctx, 5*time.Second, "systemctl", "show", "-p", "LoadState", "--value", name)
		if err == nil && strings.TrimSpace(string(out)) != "not-found" {
			return name
		}
	}
	return names[0]
}

func probeTailscale(ctx context.Context) TransportState {
	path, err := exec.LookPath("tailscale")
	if err != nil {
		return TransportState{}
	}
	out, err := runCommand(ctx, 8*time.Second, path, "status", "--json")
	if err != nil {
		return TransportState{Installed: true, State: strings.TrimSpace(string(out))}
	}
	var raw struct {
		BackendState string   `json:"BackendState"`
		TailscaleIPs []string `json:"TailscaleIPs"`
		Self         struct {
			Online bool `json:"Online"`
		} `json:"Self"`
	}
	if json.Unmarshal(out, &raw) != nil {
		return TransportState{Installed: true, State: "unknown"}
	}
	ip := ""
	if len(raw.TailscaleIPs) > 0 {
		ip = raw.TailscaleIPs[0]
	}
	return TransportState{Installed: true, Online: raw.Self.Online || raw.BackendState == "Running", IP: ip, State: raw.BackendState}
}

func probeNetwork(ctx context.Context) NetworkState {
	lookup := func(host string) error {
		lookupCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		_, err := net.DefaultResolver.LookupHost(lookupCtx, host)
		return err
	}
	ghErr := lookup("github.com")
	tsErr := lookup("controlplane.tailscale.com")
	lanIPs, lanScopes := localNetworkAddresses()
	return NetworkState{
		GitHubDNS:    ghErr == nil,
		TailscaleDNS: tsErr == nil,
		ProxySet:     os.Getenv("HTTPS_PROXY") != "" || os.Getenv("https_proxy") != "",
		LANIPs:       lanIPs,
		LANScopes:    lanScopes,
	}
}

func localNetworkAddresses() ([]string, []string) {
	var ips []string
	var scopes []string
	seenIP := map[string]bool{}
	seenScope := map[string]bool{}
	tailnetV6 := &net.IPNet{IP: net.ParseIP("fd7a:115c:a1e0::"), Mask: net.CIDRMask(48, 128)}
	interfaces, _ := net.Interfaces()
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := networkInterface.Addrs()
		for _, address := range addresses {
			ip, network, err := net.ParseCIDR(address.String())
			if err != nil || ip.IsLoopback() || ip.IsUnspecified() || !ip.IsPrivate() || tailnetV6.Contains(ip) {
				continue
			}
			ipText := ip.String()
			scope := network.String()
			if !seenIP[ipText] {
				ips = append(ips, ipText)
				seenIP[ipText] = true
			}
			if !seenScope[scope] {
				scopes = append(scopes, scope)
				seenScope[scope] = true
			}
		}
	}
	return ips, scopes
}

func probeSSHConfig(ctx context.Context, platform Platform, server Capability) bool {
	if !server.Installed {
		return false
	}
	path := server.Path
	if path == "" {
		path, _ = exec.LookPath("sshd")
	}
	if path == "" {
		return false
	}
	args := []string{"-t"}
	if platform == PlatformWindows {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		args = append(args, "-f", filepath.Join(programData, "ssh", "sshd_config"))
	}
	_, err := runCommand(ctx, 8*time.Second, path, args...)
	return err == nil
}

func resolveEffectiveAuthorizedKeysPath(configured string, snapshot Snapshot) (string, error) {
	fields := strings.Fields(configured)
	if len(fields) == 0 {
		return "", errors.New("effective authorizedkeysfile is empty")
	}
	configured = fields[0]
	home, err := targetUserHome()
	if err != nil {
		return "", err
	}
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	configured = strings.ReplaceAll(configured, "%h", home)
	configured = strings.ReplaceAll(configured, "%u", snapshot.TargetUser)
	const windowsProgramDataMarker = "__PROGRAMDATA__"
	if strings.HasPrefix(strings.ToUpper(configured), windowsProgramDataMarker) {
		configured = programData + configured[len(windowsProgramDataMarker):]
	}
	configured = filepath.FromSlash(configured)
	if strings.Contains(configured, "%") {
		return "", errors.New("effective authorizedkeysfile contains an unresolved token")
	}
	if !filepath.IsAbs(configured) {
		configured = filepath.Join(home, configured)
	}
	configured = filepath.Clean(configured)
	expectedSnapshot := snapshot
	expectedSnapshot.SSHAuthorizedKeysFile = ""
	expected, err := authorizedKeysPath(expectedSnapshot)
	if err != nil {
		return "", err
	}
	samePath := filepath.Clean(expected) == configured
	if snapshot.Platform == PlatformWindows {
		samePath = strings.EqualFold(filepath.Clean(expected), configured)
	}
	if !samePath {
		return "", fmt.Errorf("effective authorizedkeysfile %q is outside the supported target path %q", configured, expected)
	}
	return configured, nil
}

func probeAuthorizedKeys(snapshot Snapshot, profile Profile) (bool, bool, int) {
	if snapshot.SSHServer.Installed && !snapshot.SSHAuthorizedKeysFileChecked {
		return false, false, 0
	}
	path, err := authorizedKeysPath(snapshot)
	if err != nil {
		return false, false, 0
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, len(profile.SSH.PublicKeys) == 0, 0
	}
	if err != nil {
		return false, false, 0
	}
	existing := map[string]bool{}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || bytes.HasPrefix(line, []byte{'#'}) {
			continue
		}
		key, _, _, rest, parseErr := ssh.ParseAuthorizedKey(line)
		if parseErr != nil || len(bytes.TrimSpace(rest)) != 0 {
			continue
		}
		existing[ssh.FingerprintSHA256(key)] = true
	}
	for _, declared := range profile.SSH.PublicKeys {
		fingerprint, fingerprintErr := publicKeyFingerprint(declared)
		if fingerprintErr != nil || !existing[fingerprint] {
			return true, false, len(existing)
		}
	}
	return true, true, len(existing)
}

func probeFirewall(ctx context.Context, platform Platform, port int) (FirewallState, error) {
	switch platform {
	case PlatformWindows:
		script := fmt.Sprintf(`$target=%d; function Test-Port($spec){foreach($part in @($spec)-split ','){$part=$part.Trim(); if($part -in @('Any','*')){return $true}; if($part -match '^(\d+)-(\d+)$' -and $target -ge [int]$Matches[1] -and $target -le [int]$Matches[2]){return $true}; if($part -match '^\d+$' -and [int]$part -eq $target){return $true}}; return $false}; $r=Get-NetFirewallPortFilter -Protocol TCP -ErrorAction SilentlyContinue | Where-Object {Test-Port $_.LocalPort} | ForEach-Object {$filter=$_; $rule=Get-NetFirewallRule -AssociatedNetFirewallPortFilter $filter -ErrorAction SilentlyContinue | Where-Object {$_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow'}; foreach($item in $rule){$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $item -ErrorAction SilentlyContinue; [pscustomobject]@{port=$target;name=$item.Name;displayName=$item.DisplayName;scope=($a.RemoteAddress -join ',');exactPort=([string]$filter.LocalPort -eq [string]$target)}}}; ConvertTo-Json -InputObject @($r) -Compress`, port)
		// The inventory sweep enumerates every TCP port filter and its rule
		// associations; measured at 14.4s on an idle, nearly rule-free Windows
		// Server VM without elevation, so real consumer machines with many
		// rules routinely exceed the old 12s limit and get killed mid-sweep.
		out, err := runCommand(ctx, 45*time.Second, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		if err != nil {
			return FirewallState{Provider: "windows-firewall"}, err
		}
		var rules []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Scope       string `json:"scope"`
			ExactPort   bool   `json:"exactPort"`
		}
		if len(strings.TrimSpace(string(out))) > 2 && json.Unmarshal(out, &rules) != nil {
			return FirewallState{Provider: "windows-firewall"}, errors.New("could not parse Windows firewall rule inventory")
		}
		profileScript := `if(@(Get-NetFirewallProfile -ErrorAction Stop | Where-Object {-not $_.Enabled}).Count -gt 0){exit 1}`
		if _, err := runCommand(ctx, 8*time.Second, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", profileScript); err != nil {
			return FirewallState{Provider: "windows-firewall"}, errors.New("one or more Windows Firewall profiles are disabled or unreadable")
		}
		state := FirewallState{Checked: true, Enabled: true, Provider: "windows-firewall"}
		for _, rule := range rules {
			state.Ports = []int{port}
			state.Scopes = append(state.Scopes, rule.Scope)
			name := rule.Name
			if name == "" {
				name = rule.DisplayName
			}
			if !rule.ExactPort {
				if name == "" {
					name = "unnamed port-range rule"
				}
				state.PortRangeRules = append(state.PortRangeRules, name)
			}
			if broadFirewallScope(rule.Scope) {
				state.BroadExposure = true
				if name != "" {
					if rule.ExactPort {
						state.ConflictingRules = append(state.ConflictingRules, name)
					} else {
						state.UnresolvedBroadRules = append(state.UnresolvedBroadRules, name)
					}
				}
			}
		}
		return state, nil
	case PlatformMacOS:
		return FirewallState{Checked: true, Enabled: true, Provider: "application-firewall"}, nil
	default:
		if _, err := exec.LookPath("ufw"); err == nil {
			out, runErr := runCommand(ctx, 8*time.Second, "ufw", "status")
			if runErr != nil {
				return FirewallState{Provider: "ufw"}, runErr
			}
			state := FirewallState{
				Checked:  true,
				Enabled:  strings.Contains(strings.ToLower(string(out)), "status: active"),
				Provider: "ufw",
			}
			linePattern := regexp.MustCompile(`^\s*(\d+)(?:-(\d+))?(?:/tcp)?(?:\s+\(v6\))?\s+ALLOW(?:\s+IN)?\s+(.+?)\s*$`)
			for _, line := range strings.Split(string(out), "\n") {
				match := linePattern.FindStringSubmatch(line)
				if len(match) != 4 || !portSpecIncludes(match[1], match[2], port) {
					continue
				}
				scope := strings.TrimSpace(match[3])
				state.Ports = []int{port}
				state.Scopes = append(state.Scopes, scope)
				if match[2] != "" {
					state.PortRangeRules = append(state.PortRangeRules, strings.TrimSpace(line))
				}
				if broadFirewallScope(scope) || strings.EqualFold(scope, "Anywhere") || strings.HasPrefix(strings.ToLower(scope), "anywhere ") {
					state.BroadExposure = true
				}
			}
			return state, nil
		}
		if _, err := exec.LookPath("firewall-cmd"); err == nil {
			out, runErr := runCommand(ctx, 8*time.Second, "firewall-cmd", "--list-rich-rules")
			if runErr != nil {
				return FirewallState{Provider: "firewall-cmd"}, runErr
			}
			state := FirewallState{Checked: true, Enabled: true, Provider: "firewall-cmd"}
			portsOut, _ := runCommand(ctx, 8*time.Second, "firewall-cmd", "--list-ports")
			for _, declared := range strings.Fields(string(portsOut)) {
				portPart, ok := strings.CutSuffix(declared, "/tcp")
				if !ok {
					continue
				}
				start, end, exact, ok := parsePortSpec(portPart)
				if !ok || port < start || port > end {
					continue
				}
				state.Ports = []int{port}
				state.Scopes = append(state.Scopes, "")
				if !exact {
					state.PortRangeRules = append(state.PortRangeRules, declared)
				} else {
					state.BroadExposure = true
				}
			}
			portPattern := regexp.MustCompile(`port port="([^"]+)" protocol="tcp"`)
			sourcePattern := regexp.MustCompile(`source address="([^"]+)"`)
			for _, line := range strings.Split(string(out), "\n") {
				match := portPattern.FindStringSubmatch(line)
				if len(match) != 2 {
					continue
				}
				start, end, exact, ok := parsePortSpec(match[1])
				if !ok || port < start || port > end {
					continue
				}
				scope := ""
				if sourceMatch := sourcePattern.FindStringSubmatch(line); len(sourceMatch) == 2 {
					scope = sourceMatch[1]
				}
				state.Ports = []int{port}
				state.Scopes = append(state.Scopes, scope)
				if !exact {
					state.PortRangeRules = append(state.PortRangeRules, strings.TrimSpace(line))
				}
				if broadFirewallScope(scope) {
					state.BroadExposure = true
				}
			}
			return state, nil
		}
	}
	return FirewallState{}, errors.New("no supported firewall provider was detected")
}

func portSpecIncludes(startText, endText string, target int) bool {
	start, end, _, ok := parsePortSpec(startText)
	if endText != "" {
		start, end, _, ok = parsePortSpec(startText + "-" + endText)
	}
	return ok && target >= start && target <= end
}

func parsePortSpec(value string) (start, end int, exact, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(value), "-", 2)
	if len(parts) == 0 || len(parts) > 2 {
		return 0, 0, false, false
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil || start < 1 || start > 65535 {
		return 0, 0, false, false
	}
	end = start
	exact = true
	if len(parts) == 2 {
		end, err = strconv.Atoi(parts[1])
		if err != nil || end < start || end > 65535 {
			return 0, 0, false, false
		}
		exact = start == end
	}
	return start, end, exact, true
}

func broadFirewallScope(scope string) bool {
	for _, value := range strings.Split(scope, ",") {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "", "any", "*", "0.0.0.0/0", "::/0":
			return true
		}
	}
	return false
}

func commandSuccess(ctx context.Context, name string, args ...string) bool {
	_, err := runCommand(ctx, 8*time.Second, name, args...)
	return err == nil
}

func runJSON(ctx context.Context, target any, name string, args ...string) error {
	out, err := runCommand(ctx, 12*time.Second, name, args...)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, target)
}

func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// #nosec G204 G702 -- probe executables and argument templates are fixed by this package; profile values are passed as individual argv entries.
	cmd := exec.CommandContext(commandCtx, name, args...)
	configureChildProcess(cmd)
	return cmd.CombinedOutput()
}

func readSmallFile(path string) string {
	data, _ := os.ReadFile(path)
	if len(data) > 8192 {
		data = data[:8192]
	}
	return string(data)
}
