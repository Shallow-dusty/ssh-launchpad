package launchpad

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func healthySnapshot(platform Platform) Snapshot {
	return Snapshot{
		Timestamp:             time.Now(),
		Platform:              platform,
		IsAdministrator:       true,
		SessionTransport:      "local",
		PackageManager:        "apt-get",
		SSHClient:             Capability{Installed: true},
		SSHServer:             Capability{Installed: true},
		SSHService:            ServiceState{Name: "sshd", Installed: true, Running: true, StartPolicy: "enabled"},
		SSHPort:               22,
		SSHConfigValid:        true,
		AuthorizedKeysChecked: true,
		AuthorizedKeysMatch:   true,
		Tailscale:             TransportState{Installed: true, Online: true, IP: "100.64.0.1"},
		Firewall:              FirewallState{Provider: "ufw", Ports: []int{22}, Scopes: []string{"100.64.0.0/10 fd7a:115c:a1e0::/48"}},
	}
}

func TestPlannerIsIdempotentWhenStateMatches(t *testing.T) {
	p := DefaultProfile()
	plan := (Planner{}).Build(p, healthySnapshot(PlatformLinux))
	if !plan.NoChanges || len(plan.Actions) != 0 {
		t.Fatalf("expected no-op plan, got %#v", plan.Actions)
	}
}

func TestPlannerSeparatesInstallConfigServiceAndFirewall(t *testing.T) {
	p := DefaultProfile()
	p.SSH.Port = 2222
	s := healthySnapshot(PlatformWindows)
	s.SSHServer = Capability{}
	s.SSHService = ServiceState{Name: "sshd"}
	s.Firewall = FirewallState{}
	plan := (Planner{}).Build(p, s)
	got := map[string]bool{}
	for _, action := range plan.Actions {
		got[action.Layer] = true
	}
	for _, layer := range []string{"ssh-packages", "ssh-config", "ssh-service", "firewall"} {
		if !got[layer] {
			t.Errorf("missing layer %s", layer)
		}
	}
}

func TestPlannerDetectsSelfCut(t *testing.T) {
	p := DefaultProfile()
	p.SSH.Port = 2222
	s := healthySnapshot(PlatformLinux)
	s.SessionTransport = "ssh"
	plan := (Planner{}).Build(p, s)
	if !plan.SelfCutDetected {
		t.Fatal("expected self-cut detection")
	}
	for _, action := range plan.Actions {
		if action.Operation == "configure_sshd" && !action.SelfCutRisk {
			t.Fatal("sshd configuration should be marked as self-cut risk")
		}
	}
}

func TestConfigureSSHRestartsOnlyAnAlreadyRunningService(t *testing.T) {
	profile := DefaultProfile()
	profile.SSH.Port = 2222
	running := healthySnapshot(PlatformWindows)
	plan := (Planner{}).Build(profile, running)
	for _, action := range plan.Actions {
		if action.Operation == "configure_sshd" {
			if !strings.Contains(strings.Join(action.Command, " "), "Restart-Service sshd") {
				t.Fatal("running sshd must restart after a validated config change")
			}
			if !strings.Contains(strings.Join(action.Command, " "), "backup restored") && !strings.Contains(strings.Join(action.Command, " "), "Copy-Item $b $p") {
				t.Fatal("config restart failure must restore the backup")
			}
			return
		}
	}
	t.Fatal("configure_sshd action missing")
}

func TestFirewallCommandIsPortAndScopeAware(t *testing.T) {
	p := DefaultProfile()
	p.SSH.Port = 2222
	s := healthySnapshot(PlatformWindows)
	s.Firewall = FirewallState{}
	plan := (Planner{}).Build(p, s)
	for _, action := range plan.Actions {
		if action.Operation == "configure_firewall" {
			command := strings.Join(action.Command, " ")
			if !strings.Contains(command, "2222") || !strings.Contains(command, "100.64.0.0/10") {
				t.Fatalf("firewall command is not port/scope aware: %s", command)
			}
			return
		}
	}
	t.Fatal("firewall action not found")
}

func TestFirewallIsConfiguredBeforeStartingANewSSHService(t *testing.T) {
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHService.Running = false
	snapshot.Firewall = FirewallState{}
	plan := (Planner{}).Build(profile, snapshot)
	firewallIndex, serviceIndex := -1, -1
	for index, action := range plan.Actions {
		switch action.Operation {
		case "configure_firewall":
			firewallIndex = index
		case "enable_sshd":
			serviceIndex = index
		}
	}
	if firewallIndex < 0 || serviceIndex < 0 || firewallIndex > serviceIndex {
		t.Fatalf("firewall must precede service start: %#v", plan.Actions)
	}
}

func TestPlannerSupportsAllDeclaredPlatforms(t *testing.T) {
	p := DefaultProfile()
	for _, platform := range []Platform{PlatformWindows, PlatformLinux, PlatformWSL, PlatformMacOS} {
		s := healthySnapshot(platform)
		s.SSHServer = Capability{}
		plan := (Planner{}).Build(p, s)
		if len(plan.Actions) == 0 {
			t.Errorf("%s produced no install plan", platform)
		}
	}
}

func TestTailnetSetupIsPhasedAndRequiresOnlineTransport(t *testing.T) {
	profile := DefaultProfile()
	profile.Transport.Install = true
	missing := healthySnapshot(PlatformWindows)
	missing.PackageManager = "winget"
	missing.Tailscale = TransportState{}
	plan := (Planner{}).Build(profile, missing)
	if len(plan.Actions) != 1 || plan.Actions[0].Operation != "install_tailscale" {
		t.Fatalf("first phase must install only Tailscale: %#v", plan)
	}
	if len(plan.Blockers) != 0 {
		t.Fatalf("install phase should be executable: %#v", plan.Blockers)
	}

	offline := missing
	offline.Tailscale.Installed = true
	plan = (Planner{}).Build(profile, offline)
	if len(plan.Actions) != 0 || len(plan.Blockers) == 0 || plan.NoChanges {
		t.Fatalf("signed-out Tailscale must block SSH/firewall work: %#v", plan)
	}
}

func TestPersonalCardAuthKeyEnablesOnePassTailnetPlanWithoutExposure(t *testing.T) {
	profile := DefaultProfile()
	profile.Transport.Install = true
	profile.Transport.AuthKey = "tskey-auth-example-once"
	profile.SSH.PublicKeys = []string{personalCardTestKey}
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.PackageManager = "winget"
	snapshot.Tailscale = TransportState{}
	snapshot.SSHServer = Capability{}
	snapshot.SSHService = ServiceState{Name: "sshd"}
	snapshot.AuthorizedKeysMatch = false
	snapshot.Firewall = FirewallState{Provider: "windows-firewall"}

	plan := (Planner{}).Build(profile, snapshot)
	operations := make([]string, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		operations = append(operations, action.Operation)
		if strings.Contains(strings.Join(action.Command, " "), profile.Transport.AuthKey) {
			t.Fatal("Tailscale auth key leaked into the inspectable action plan")
		}
	}
	for _, required := range []string{"install_tailscale", "authenticate_tailscale", "install_ssh", "configure_keys", "configure_firewall"} {
		if !slices.Contains(operations, required) {
			t.Fatalf("one-pass personal-card plan missing %s: %#v", required, operations)
		}
	}
	if len(plan.Blockers) != 0 {
		t.Fatalf("one-pass personal-card plan should be executable: %#v", plan.Blockers)
	}
}

func TestBroadWindowsFirewallRuleIsAConflict(t *testing.T) {
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.Firewall.BroadExposure = true
	snapshot.Firewall.ConflictingRules = []string{"OpenSSH-Server-In-TCP"}
	plan := (Planner{}).Build(profile, snapshot)
	for _, action := range plan.Actions {
		if action.Operation == "configure_firewall" {
			command := strings.Join(action.Command, " ")
			if !strings.Contains(command, "OpenSSH-Server-In-TCP") || !strings.Contains(command, "Enabled False") {
				t.Fatalf("conflicting broad rule is not disabled: %s", command)
			}
			return
		}
	}
	t.Fatal("broad exposure should require a replacement firewall rule")
}

func TestLANUsesDetectedUnixScopes(t *testing.T) {
	profile := DefaultProfile()
	profile.Transport.Mode = "lan"
	profile.Exposure.Mode = "lan"
	snapshot := healthySnapshot(PlatformLinux)
	snapshot.Network.LANScopes = []string{"192.168.10.0/24", "fd00:10::/64"}
	snapshot.Firewall = FirewallState{Provider: "ufw"}
	plan := (Planner{}).Build(profile, snapshot)
	for _, action := range plan.Actions {
		if action.Operation == "configure_firewall" {
			command := strings.Join(action.Command, " ")
			if !strings.Contains(command, "192.168.10.0/24") || !strings.Contains(command, "fd00:10::/64") {
				t.Fatalf("LAN scopes were not all planned: %s", command)
			}
			return
		}
	}
	t.Fatal("LAN firewall action missing")
}
