package launchpad

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

func probeSSHServer(ctx context.Context, platform Platform, profile Profile) (Capability, ServiceState, int) {
	service := ServiceState{Name: profile.Advanced.LinuxSSHService}
	port := 0
	switch platform {
	case PlatformWindows:
		service.Name = profile.Advanced.WindowsSSHService
		script := `$s=Get-CimInstance Win32_Service -Filter "Name='sshd'" -ErrorAction SilentlyContinue; $l=Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object {$_.OwningProcess -in (Get-Process sshd -ErrorAction SilentlyContinue).Id} | Select-Object -First 1; $exe=$null; if($s){$sp=$s.PathName; if($sp -match '^"([^"]+)"'){$exe=$Matches[1]}elseif($sp){$exe=($sp -split ' ')[0]}}; $bin=[bool]($exe -and (Test-Path -LiteralPath $exe -PathType Leaf)); [pscustomobject]@{installed=[bool]$s;running=($s.State -eq 'Running');startPolicy=$s.StartMode;port=$l.LocalPort;path=(Get-Command sshd.exe -ErrorAction SilentlyContinue).Source;binaryExists=$bin}|ConvertTo-Json -Compress`
		var v struct {
			Installed    bool   `json:"installed"`
			Running      bool   `json:"running"`
			StartPolicy  string `json:"startPolicy"`
			Port         int    `json:"port"`
			Path         string `json:"path"`
			BinaryExists bool   `json:"binaryExists"`
		}
		if runJSON(ctx, &v, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script) == nil {
			binaryExists := v.BinaryExists
			return Capability{Installed: v.Installed, Path: v.Path, BinaryExists: &binaryExists}, ServiceState{Name: "sshd", Installed: v.Installed, Running: v.Running, StartPolicy: v.StartPolicy}, v.Port
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
	Ports                        []int
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
		Ports:                        parseConfiguredSSHPorts(out),
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

// programDataDir returns the Windows ProgramData directory with a safe
// fallback matching the other probes in this package.
func programDataDir() string {
	if programData := os.Getenv("ProgramData"); programData != "" {
		return programData
	}
	return `C:\ProgramData`
}

// isAdminDefaultAuthorizedKeysPath reports whether the effective
// AuthorizedKeysFile printed by sshd -T is the stock per-user default that
// Win32-OpenSSH replaces with administrators_authorized_keys for admin-group
// users. Anything explicitly configured fails closed to the strict resolver.
func isAdminDefaultAuthorizedKeysPath(configured string, snapshot Snapshot) bool {
	fields := strings.Fields(configured)
	if len(fields) != 1 {
		return false
	}
	configured = fields[0]
	if strings.Contains(configured, "%") {
		return false
	}
	home, err := targetUserHome()
	if err != nil {
		return false
	}
	defaultPath := filepath.Join(home, ".ssh", "authorized_keys")
	if !filepath.IsAbs(configured) {
		configured = filepath.Join(home, configured)
	}
	return strings.EqualFold(filepath.Clean(configured), filepath.Clean(defaultPath))
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
