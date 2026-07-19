package launchpad

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var publicKeyPattern = regexp.MustCompile(`^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp(256|384|521)|sk-ssh-ed25519@openssh\.com)\s+[A-Za-z0-9+/=]+(?:\s+[^\r\n]+)?$`)

func ValidatePublicKey(value string) error {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "PRIVATE KEY") {
		return errors.New("private keys are not accepted")
	}
	if !publicKeyPattern.MatchString(value) {
		return errors.New("value is not a supported OpenSSH public key")
	}
	return nil
}

func DefaultProfile() Profile {
	return Profile{
		SchemaVersion: SchemaVersion,
		Name:          "default",
		Target:        TargetProfile{Platform: PlatformAuto},
		SSH: SSHProfile{
			Enabled:                true,
			Port:                   22,
			PasswordAuthentication: false,
		},
		Transport: TransportProfile{Mode: "tailnet", Install: false},
		Exposure:  ExposureProfile{Mode: "tailnet"},
		Download: DownloadProfile{
			Strategy: "official",
			Retries:  3,
		},
		Safety: SafetyProfile{
			ConfirmHighRisk:      true,
			PreventSelfCut:       true,
			ScheduledDelaySecond: 20,
			AutoRollback:         true,
		},
		Advanced: AdvancedProfile{
			WindowsSSHService: "sshd",
			LinuxSSHService:   "auto",
			MacOSSSHLabel:     "com.openssh.sshd",
		},
	}
}

func LoadProfile(path string) (Profile, error) {
	if path == "" {
		p := DefaultProfile()
		return p, p.Validate()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}
	p := DefaultProfile()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &p)
	default:
		err = json.Unmarshal(data, &p)
	}
	if err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func (p Profile) Validate() error {
	var errs []error
	if p.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("schemaVersion must be %d", SchemaVersion))
	}
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	switch p.Target.Platform {
	case PlatformAuto, PlatformWindows, PlatformLinux, PlatformMacOS, PlatformWSL:
	default:
		errs = append(errs, fmt.Errorf("unsupported target platform %q", p.Target.Platform))
	}
	if p.SSH.Port < 1 || p.SSH.Port > 65535 {
		errs = append(errs, errors.New("ssh.port must be between 1 and 65535"))
	}
	for i, key := range p.SSH.PublicKeys {
		if err := ValidatePublicKey(key); err != nil {
			errs = append(errs, fmt.Errorf("ssh.publicKeys[%d] is not a supported OpenSSH public key", i))
		}
	}
	switch p.Transport.Mode {
	case "tailnet", "lan", "custom", "none":
	default:
		errs = append(errs, fmt.Errorf("unsupported transport.mode %q", p.Transport.Mode))
	}
	switch p.Exposure.Mode {
	case "tailnet", "lan", "custom", "none":
	default:
		errs = append(errs, fmt.Errorf("unsupported exposure.mode %q", p.Exposure.Mode))
	}
	if p.Exposure.Mode == "custom" && len(p.Exposure.CustomCIDRs) == 0 {
		errs = append(errs, errors.New("custom exposure requires customCidrs"))
	}
	for _, cidr := range p.Exposure.CustomCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			errs = append(errs, fmt.Errorf("invalid custom CIDR %q", cidr))
		}
	}
	switch p.Download.Strategy {
	case "official", "package-manager", "mirror", "proxy", "offline", "cache":
	default:
		errs = append(errs, fmt.Errorf("unsupported download.strategy %q", p.Download.Strategy))
	}
	if p.Download.Strategy == "mirror" {
		if err := validateHTTPSURL(p.Download.MirrorBaseURL); err != nil {
			errs = append(errs, fmt.Errorf("mirrorBaseUrl: %w", err))
		}
	}
	if p.Download.Strategy == "proxy" {
		u, err := url.Parse(p.Download.ProxyURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") {
			errs = append(errs, errors.New("proxyUrl must use http, https, or socks5"))
		}
	}
	if p.Download.Strategy == "offline" && strings.TrimSpace(p.Download.OfflineBundle) == "" {
		errs = append(errs, errors.New("offline strategy requires offlineBundle"))
	}
	if p.Download.Retries < 0 || p.Download.Retries > 10 {
		errs = append(errs, errors.New("download.retries must be between 0 and 10"))
	}
	if p.Safety.ScheduledDelaySecond < 5 || p.Safety.ScheduledDelaySecond > 3600 {
		errs = append(errs, errors.New("scheduledDelaySeconds must be between 5 and 3600"))
	}
	return errors.Join(errs...)
}

func validateHTTPSURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.Host == "" {
		return errors.New("must be an absolute HTTPS URL")
	}
	return nil
}
