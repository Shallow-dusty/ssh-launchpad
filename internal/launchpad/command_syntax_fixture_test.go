package launchpad

import "testing"

const syntaxTestPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller"

func generatedSyntaxTestActions(t *testing.T, platform Platform) []Action {
	t.Helper()
	profile := DefaultProfile()
	profile.SSH.Port = 2222
	profile.SSH.PublicKeys = []string{syntaxTestPublicKey}
	snapshot := Snapshot{
		Platform:              platform,
		PackageManager:        "apt-get",
		SSHService:            ServiceState{Name: "sshd"},
		AuthorizedKeysChecked: true,
		Tailscale:             TransportState{Installed: true, Online: true},
		Firewall:              FirewallState{Provider: "ufw"},
	}
	if platform == PlatformWindows {
		snapshot.PackageManager = "winget"
		snapshot.Firewall.Provider = "windows-firewall"
	}
	if platform == PlatformMacOS {
		snapshot.SSHClient = Capability{Installed: true}
		snapshot.SSHServer = Capability{Installed: true}
		snapshot.SSHService.Name = "com.openssh.sshd"
		snapshot.Firewall.Provider = "application-firewall"
	}
	plan := (Planner{}).Build(profile, snapshot)
	if len(plan.Actions) == 0 {
		t.Fatal("expected generated actions")
	}
	return plan.Actions
}

func TestParseConfiguredSSHPortUsesEffectiveConfiguration(t *testing.T) {
	output := []byte("port 2222\naddressfamily any\npasswordauthentication no\n")
	if got := parseConfiguredSSHPort(output); got != 2222 {
		t.Fatalf("effective sshd port mismatch: got %d", got)
	}
	if got := parseConfiguredSSHPort([]byte("passwordauthentication no\n")); got != 0 {
		t.Fatalf("missing port should remain unknown, got %d", got)
	}
}
