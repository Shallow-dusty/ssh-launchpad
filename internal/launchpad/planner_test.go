package launchpad

import (
	"strings"
	"testing"
	"time"
)

func healthySnapshot(platform Platform) Snapshot {
	return Snapshot{
		Timestamp:        time.Now(),
		Platform:         platform,
		IsAdministrator:  true,
		SessionTransport: "local",
		PackageManager:   "apt-get",
		SSHClient:        Capability{Installed: true},
		SSHServer:        Capability{Installed: true},
		SSHService:       ServiceState{Name: "sshd", Installed: true, Running: true, StartPolicy: "enabled"},
		SSHPort:          22,
		SSHConfigValid:   true,
		Tailscale:        TransportState{Installed: true, Online: true, IP: "100.64.0.1"},
		Firewall:         FirewallState{Provider: "ufw", Ports: []int{22}, Scopes: []string{"100.64.0.0/10 fd7a:115c:a1e0::/48"}},
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
