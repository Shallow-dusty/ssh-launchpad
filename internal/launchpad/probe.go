package launchpad

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
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
		SSHConfigValid:   true,
	}
	s.IsAdministrator = detectAdministrator(ctx, platform)
	s.PackageManager = detectPackageManager()
	s.SSHClient = probeCapability(ctx, "ssh", "-V")
	s.SSHServer, s.SSHService, s.SSHPort = probeSSHServer(ctx, platform, profile)
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

func detectAdministrator(ctx context.Context, platform Platform) bool {
	if platform == PlatformWindows {
		_, err := runCommand(ctx, 5*time.Second, "net.exe", "session")
		return err == nil
	}
	return os.Geteuid() == 0
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
		return Capability{Installed: pathErr == nil, Path: path}, ServiceState{Name: service.Name, Installed: pathErr == nil, Running: err == nil}, detectListeningPort(ctx)
	default:
		name := profile.Advanced.LinuxSSHService
		if name == "" || name == "auto" {
			name = firstExistingService(ctx, "sshd", "ssh")
		}
		service.Name = name
		path, pathErr := exec.LookPath("sshd")
		active := commandSuccess(ctx, "systemctl", "is-active", "--quiet", name)
		enabled := commandSuccess(ctx, "systemctl", "is-enabled", "--quiet", name)
		return Capability{Installed: pathErr == nil, Path: path}, ServiceState{Name: name, Installed: pathErr == nil, Running: active, StartPolicy: map[bool]string{true: "enabled", false: "disabled"}[enabled]}, detectListeningPort(ctx)
	}
	return Capability{}, service, port
}

func detectListeningPort(ctx context.Context) int {
	for _, args := range [][]string{{"-lnt"}, {"-lntp"}} {
		out, err := runCommand(ctx, 5*time.Second, "ss", args...)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "sshd") || strings.Contains(line, ":22 ") {
				fields := strings.Fields(line)
				for _, field := range fields {
					if i := strings.LastIndex(field, ":"); i >= 0 {
						if p, err := strconv.Atoi(strings.Trim(field[i+1:], "[]")); err == nil {
							return p
						}
					}
				}
			}
		}
	}
	return 0
}

func firstExistingService(ctx context.Context, names ...string) string {
	for _, name := range names {
		if _, err := runCommand(ctx, 5*time.Second, "systemctl", "status", name); err == nil {
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
	return NetworkState{
		GitHubDNS:    ghErr == nil,
		TailscaleDNS: tsErr == nil,
		ProxySet:     os.Getenv("HTTPS_PROXY") != "" || os.Getenv("https_proxy") != "",
	}
}

func probeFirewall(ctx context.Context, platform Platform, port int) (FirewallState, error) {
	switch platform {
	case PlatformWindows:
		script := fmt.Sprintf(`$target=%d; function Test-Port($spec){foreach($part in @($spec)-split ','){$part=$part.Trim(); if($part -match '^(\d+)-(\d+)$' -and $target -ge [int]$Matches[1] -and $target -le [int]$Matches[2]){return $true}; if($part -match '^\d+$' -and [int]$part -eq $target){return $true}}; return $false}; $r=Get-NetFirewallPortFilter -Protocol TCP -ErrorAction SilentlyContinue | Where-Object {Test-Port $_.LocalPort} | ForEach-Object {$rule=Get-NetFirewallRule -AssociatedNetFirewallPortFilter $_ -ErrorAction SilentlyContinue | Where-Object {$_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow'}; foreach($item in $rule){$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $item -ErrorAction SilentlyContinue; [pscustomobject]@{port=$target;name=$item.DisplayName;scope=($a.RemoteAddress -join ',')}}}; @($r)|ConvertTo-Json -Compress`, port)
		out, err := runCommand(ctx, 12*time.Second, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		if err != nil {
			return FirewallState{Provider: "windows-firewall"}, err
		}
		if len(out) > 2 {
			return FirewallState{Provider: "windows-firewall", Ports: []int{port}, Scopes: []string{strings.TrimSpace(string(out))}}, nil
		}
		return FirewallState{Provider: "windows-firewall"}, nil
	case PlatformMacOS:
		return FirewallState{Provider: "application-firewall"}, nil
	default:
		for _, provider := range []string{"ufw", "firewall-cmd"} {
			if _, err := exec.LookPath(provider); err == nil {
				out, _ := runCommand(ctx, 8*time.Second, provider, map[string][]string{"ufw": {"status"}, "firewall-cmd": {"--list-all"}}[provider]...)
				if strings.Contains(string(out), strconv.Itoa(port)) {
					return FirewallState{Provider: provider, Ports: []int{port}, Scopes: []string{strings.TrimSpace(string(out))}}, nil
				}
				return FirewallState{Provider: provider}, nil
			}
		}
	}
	return FirewallState{}, nil
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
	return exec.CommandContext(commandCtx, name, args...).CombinedOutput()
}

func readSmallFile(path string) string {
	data, _ := os.ReadFile(path)
	if len(data) > 8192 {
		data = data[:8192]
	}
	return string(data)
}
