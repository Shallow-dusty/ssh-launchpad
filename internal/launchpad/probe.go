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
	"runtime"
	"strings"
	"sync"
	"time"
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
		s.SSHPorts = effective.Ports
		if s.SSHServer.Installed && (s.SSHServer.BinaryExists == nil || *s.SSHServer.BinaryExists) {
			if err := inspectSSHPolicy(sshConfigPath(platform), platform); err != nil {
				if errors.Is(err, errSSHPolicyNotInitialized) {
					s.SSHPolicyNotInitialized = true
					// Apply writes the packaged sshd_config_default template, so the key
					// path is the stock one (machine-wide administrators_authorized_keys
					// for administrators, per-user otherwise). It is known before the file
					// exists; the key action creates it.
					if keyPath, pathErr := authorizedKeysPath(s); pathErr == nil {
						s.SSHAuthorizedKeysFileChecked = true
						s.SSHAuthorizedKeysFile = keyPath
					}
				} else {
					s.SSHPolicyError = err.Error()
				}
				effective.Checked = false
			}
		}
		if effective.Checked {
			// sshd -T parsed the complete effective configuration successfully. It
			// is a sufficient syntax check when an unprivileged probe cannot read
			// host private keys required by sshd -t.
			s.SSHConfigValid = true
			if path, err := resolveEffectiveAuthorizedKeysPath(effective.AuthorizedKeysFile, s); err == nil {
				s.SSHAuthorizedKeysFileChecked = true
				s.SSHAuthorizedKeysFile = path
			} else if platform == PlatformWindows && s.TargetUserIsAdmin && hasStockAdminMatch(sshConfigPath(platform)) && isAdminDefaultAuthorizedKeysPath(effective.AuthorizedKeysFile, s) {
				// Win32-OpenSSH serves members of the Administrators group from
				// the machine-wide administrators_authorized_keys file whenever
				// the effective AuthorizedKeysFile is the default per-user path,
				// even though sshd -T still prints that per-user default. Model
				// that redirection so stock Windows hosts stay verifiable.
				s.SSHAuthorizedKeysFileChecked = true
				s.SSHAuthorizedKeysFile = filepath.Join(programDataDir(), "ssh", "administrators_authorized_keys")
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
