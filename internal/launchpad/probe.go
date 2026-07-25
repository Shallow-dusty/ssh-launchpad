package launchpad

import (
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
	"time"

	"golang.org/x/crypto/ssh"
)

type Probe interface {
	Check(context.Context, Profile) (Snapshot, error)
}

type SystemProbe struct{}

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
	s.SSHServer, s.SSHService, s.SSHPort = probeSSHServer(ctx, platform, profile)
	s.SSHConfigValid = probeSSHConfig(ctx, platform, s.SSHServer)
	s.AuthorizedKeysChecked, s.AuthorizedKeysMatch = probeAuthorizedKeys(s, profile)
	s.Tailscale = probeTailscale(ctx)
	s.Network = probeNetwork(ctx)
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

func probeAuthorizedKeys(snapshot Snapshot, profile Profile) (bool, bool) {
	if len(profile.SSH.PublicKeys) == 0 {
		return true, true
	}
	path, err := authorizedKeysPath(snapshot)
	if err != nil {
		return false, false
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, false
	}
	if err != nil {
		return false, false
	}
	existing := map[string]bool{}
	for len(data) > 0 {
		key, _, _, rest, parseErr := ssh.ParseAuthorizedKey(data)
		if parseErr != nil {
			break
		}
		existing[ssh.FingerprintSHA256(key)] = true
		data = rest
	}
	for _, declared := range profile.SSH.PublicKeys {
		fingerprint, fingerprintErr := publicKeyFingerprint(declared)
		if fingerprintErr != nil || !existing[fingerprint] {
			return true, false
		}
	}
	return true, true
}

func probeFirewall(ctx context.Context, platform Platform, port int) (FirewallState, error) {
	switch platform {
	case PlatformWindows:
		script := fmt.Sprintf(`$target=%d; function Test-Port($spec){foreach($part in @($spec)-split ','){$part=$part.Trim(); if($part -in @('Any','*')){return $true}; if($part -match '^(\d+)-(\d+)$' -and $target -ge [int]$Matches[1] -and $target -le [int]$Matches[2]){return $true}; if($part -match '^\d+$' -and [int]$part -eq $target){return $true}}; return $false}; $r=Get-NetFirewallPortFilter -Protocol TCP -ErrorAction SilentlyContinue | Where-Object {Test-Port $_.LocalPort} | ForEach-Object {$filter=$_; $rule=Get-NetFirewallRule -AssociatedNetFirewallPortFilter $filter -ErrorAction SilentlyContinue | Where-Object {$_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow'}; foreach($item in $rule){$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $item -ErrorAction SilentlyContinue; [pscustomobject]@{port=$target;name=$item.Name;displayName=$item.DisplayName;scope=($a.RemoteAddress -join ',');exactPort=([string]$filter.LocalPort -eq [string]$target)}}}; ConvertTo-Json -InputObject @($r) -Compress`, port)
		out, err := runCommand(ctx, 12*time.Second, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		if err != nil {
			return FirewallState{Provider: "windows-firewall"}, err
		}
		var rules []struct {
			Name      string `json:"name"`
			Scope     string `json:"scope"`
			ExactPort bool   `json:"exactPort"`
		}
		if len(strings.TrimSpace(string(out))) > 2 && json.Unmarshal(out, &rules) != nil {
			return FirewallState{Provider: "windows-firewall"}, errors.New("could not parse Windows firewall rule inventory")
		}
		state := FirewallState{Provider: "windows-firewall"}
		for _, rule := range rules {
			state.Ports = []int{port}
			state.Scopes = append(state.Scopes, rule.Scope)
			if broadFirewallScope(rule.Scope) {
				state.BroadExposure = true
				if rule.Name != "" {
					if rule.ExactPort {
						state.ConflictingRules = append(state.ConflictingRules, rule.Name)
					} else {
						state.UnresolvedBroadRules = append(state.UnresolvedBroadRules, rule.Name)
					}
				}
			}
		}
		return state, nil
	case PlatformMacOS:
		return FirewallState{Provider: "application-firewall"}, nil
	default:
		if _, err := exec.LookPath("ufw"); err == nil {
			out, runErr := runCommand(ctx, 8*time.Second, "ufw", "status")
			if runErr != nil {
				return FirewallState{Provider: "ufw"}, runErr
			}
			state := FirewallState{Provider: "ufw"}
			linePattern := regexp.MustCompile(`^\s*(\d+)(?:/tcp)?(?:\s+\(v6\))?\s+ALLOW(?:\s+IN)?\s+(.+?)\s*$`)
			for _, line := range strings.Split(string(out), "\n") {
				match := linePattern.FindStringSubmatch(line)
				if len(match) != 3 || match[1] != strconv.Itoa(port) {
					continue
				}
				scope := strings.TrimSpace(match[2])
				state.Ports = []int{port}
				state.Scopes = append(state.Scopes, scope)
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
			state := FirewallState{Provider: "firewall-cmd"}
			portsOut, _ := runCommand(ctx, 8*time.Second, "firewall-cmd", "--list-ports")
			for _, declared := range strings.Fields(string(portsOut)) {
				if declared == strconv.Itoa(port)+"/tcp" {
					state.Ports = []int{port}
					state.Scopes = append(state.Scopes, "")
					state.BroadExposure = true
				}
			}
			portPattern := regexp.MustCompile(`port port="` + regexp.QuoteMeta(strconv.Itoa(port)) + `" protocol="tcp"`)
			sourcePattern := regexp.MustCompile(`source address="([^"]+)"`)
			for _, line := range strings.Split(string(out), "\n") {
				if !portPattern.MatchString(line) {
					continue
				}
				scope := ""
				if match := sourcePattern.FindStringSubmatch(line); len(match) == 2 {
					scope = match[1]
				}
				state.Ports = []int{port}
				state.Scopes = append(state.Scopes, scope)
				if broadFirewallScope(scope) {
					state.BroadExposure = true
				}
			}
			return state, nil
		}
	}
	return FirewallState{}, nil
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
