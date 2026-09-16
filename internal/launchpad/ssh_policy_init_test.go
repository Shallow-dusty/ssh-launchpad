package launchpad

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// A fresh Windows capability install ships no sshd_config; the file appears
// only when sshd starts for the first time. The probe must treat that
// documented intermediate state as "configure me" instead of dead-ending the
// phased flow, while still failing closed for every other unreadable policy.
func TestInspectSSHPolicyReportsMissingWindowsPolicyAsNotInitialized(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "sshd_config")
	err := inspectSSHPolicy(missing, PlatformWindows)
	if !errors.Is(err, errSSHPolicyNotInitialized) {
		t.Fatalf("expected the not-initialized sentinel, got %v", err)
	}
}

func TestInspectSSHPolicyKeepsMissingUnixPolicyFatal(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "sshd_config")
	err := inspectSSHPolicy(missing, PlatformLinux)
	if err == nil {
		t.Fatal("a missing unix policy file must stay a hard probe failure")
	}
	if errors.Is(err, errSSHPolicyNotInitialized) {
		t.Fatal("unix has no stock template to recreate, so the sentinel must not be used")
	}
}

func TestPlannerPlansConfigurationForUninitializedWindowsPolicy(t *testing.T) {
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHPolicyNotInitialized = true
	snapshot.SSHConfigValid = false
	snapshot.SSHAuthenticationChecked = false
	snapshot.SSHPort = 0
	plan := (Planner{}).Build(profile, snapshot)
	if len(plan.Blockers) != 0 {
		t.Fatalf("an uninitialized policy file must not block the plan: %v", plan.Blockers)
	}
	if !hasOperation(plan.Actions, "configure_sshd") {
		t.Fatalf("expected a configure_sshd action, got %#v", plan.Actions)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("the user must be told that the stock template is written instead of guessed policy")
	}
}

func TestPlannerStillBlocksUnreadablePolicy(t *testing.T) {
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHPolicyError = "read SSH policy: access is denied"
	plan := (Planner{}).Build(profile, snapshot)
	if len(plan.Blockers) != 1 || !strings.Contains(plan.Blockers[0], "read SSH policy") {
		t.Fatalf("unreadable policy must stay a blocker, got %#v", plan.Blockers)
	}
}

func TestWindowsEnableSshdGeneratesMissingHostKeys(t *testing.T) {
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHService.Running = false
	plan := (Planner{}).Build(profile, snapshot)
	for _, action := range plan.Actions {
		if action.Operation != "enable_sshd" {
			continue
		}
		command := strings.Join(action.Command, " ")
		for _, want := range []string{"ssh-keygen.exe", "-A", "ssh_host_ed25519_key", "SetAccessRuleProtection", "/setowner"} {
			if !strings.Contains(command, want) {
				t.Fatalf("enable_sshd must bootstrap host keys (%q missing): %s", want, command)
			}
		}
		return
	}
	t.Fatalf("expected an enable_sshd action, got %#v", plan.Actions)
}

func hasOperation(actions []Action, operation string) bool {
	for _, action := range actions {
		if action.Operation == operation {
			return true
		}
	}
	return false
}

func TestWindowsRuleCannotReachSshd(t *testing.T) {
	cases := []struct {
		name    string
		program string
		pkg     string
		service string
		want    bool
	}{
		{"unscoped rule can reach sshd", "Any", "", "Any", false},
		{"open ssh capability rule targets sshd", `C:\Windows\system32\OpenSSH\sshd.exe`, "", "Any", false},
		{"managed rule without program filter", "", "", "", false},
		{"remote desktop shadow helper", `C:\Windows\system32\RdpSa.exe`, "", "Any", true},
		{"store app container", "Any", "S-1-15-2-2434737943-167758768", "Any", true},
		{"unrelated service", "Any", "", "RDP", true},
	}
	for _, tc := range cases {
		if got := windowsRuleCannotReachSshd(tc.program, tc.pkg, tc.service); got != tc.want {
			t.Fatalf("%s: windowsRuleCannotReachSshd(%q,%q,%q) = %v, want %v", tc.name, tc.program, tc.pkg, tc.service, got, tc.want)
		}
	}
}

func TestUninitializedWindowsPolicyKnowsKeyPath(t *testing.T) {
	// The probe resolves the stock key path from the packaged template, so the
	// planner must not block on "key file could not be verified" merely because
	// the fresh install has no policy file yet.
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHPolicyNotInitialized = true
	snapshot.SSHConfigValid = false
	snapshot.SSHAuthenticationChecked = false
	snapshot.AuthorizedKeysChecked = true
	snapshot.AuthorizedKeysMatch = false
	profile := DefaultProfile()
	profile.SSH.PublicKeys = []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller"}
	plan := (Planner{}).Build(profile, snapshot)
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "authorized_keys file could not be verified") {
			t.Fatalf("uninitialized policy must not block on key verification: %v", plan.Blockers)
		}
	}
	if !hasOperation(plan.Actions, "configure_keys") {
		t.Fatalf("expected a configure_keys action, got %#v", plan.Actions)
	}
}

func TestWindowsConfigActionCreatesMissingDirectory(t *testing.T) {
	// A fresh capability install has no C:\ProgramData\ssh directory, so the
	// config action must create it before writing the template.
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHPolicyNotInitialized = true
	snapshot.SSHConfigValid = false
	snapshot.SSHAuthenticationChecked = false
	plan := (Planner{}).Build(profile, snapshot)
	for _, action := range plan.Actions {
		if action.Operation != "configure_sshd" {
			continue
		}
		command := strings.Join(action.Command, " ")
		for _, want := range []string{"sshd_config_default", "New-Item -ItemType Directory -Force", "ssh-keygen.exe", "/setowner"} {
			if !strings.Contains(command, want) {
				t.Fatalf("configure_sshd must bootstrap the missing policy directory (%q missing)", want)
			}
		}
		return
	}
	t.Fatalf("expected a configure_sshd action, got %#v", plan.Actions)
}

func TestRollbackCommandsConsumeBackups(t *testing.T) {
	// A rolled-back failure must leave the host retryable: the guard that stops
	// Apply from overwriting recovery material is only useful until the recorded
	// restore has actually run, so the rollback command consumes its own backup.
	profile := DefaultProfile()
	profile.SSH.PublicKeys = []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller"}
	for _, platform := range []Platform{PlatformWindows, PlatformLinux} {
		snapshot := healthySnapshot(platform)
		snapshot.SSHPort = 2222
		snapshot.AuthorizedKeysMatch = false
		plan := (Planner{}).Build(profile, snapshot)
		var config, keys *Action
		for index := range plan.Actions {
			switch plan.Actions[index].Operation {
			case "configure_sshd":
				config = &plan.Actions[index]
			case "configure_keys":
				keys = &plan.Actions[index]
			}
		}
		if config == nil || keys == nil {
			t.Fatalf("%s: expected config and key actions, got %#v", platform, plan.Actions)
		}
		if !rollbackConsumesBackup(*config) {
			t.Fatalf("%s: configure_sshd rollback must consume its backup: %v", platform, config.RollbackCommand)
		}
		if !rollbackConsumesBackup(*keys) {
			t.Fatalf("%s: configure_keys rollback must consume its backup: %v", platform, keys.RollbackCommand)
		}
	}
}

func rollbackConsumesBackup(action Action) bool {
	command := strings.Join(action.RollbackCommand, " ")
	return strings.Contains(command, "$backup") && strings.Contains(command, "rm -f") ||
		strings.Contains(command, "Remove-Item -LiteralPath $b -Force") ||
		strings.Contains(command, "Remove-Item -LiteralPath $backup -Force") ||
		strings.Contains(command, "if(Test-Path -LiteralPath $f){ Remove-Item -LiteralPath $f -Force -ErrorAction Stop }")
}

func TestManagedScopesDoNotBlockRescoping(t *testing.T) {
	// Rescoping must work on a host the tool already configured: the previous
	// scope lives in the rule Apply rewrites, not in a foreign rule.
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.Firewall.Scopes = []string{"10.0.0.0/24"}
	snapshot.Firewall.ManagedScopes = []string{"10.0.0.0/24"}
	plan := (Planner{}).Build(profile, snapshot)
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "outside the requested scope") {
			t.Fatalf("the managed rule's own scope must not block rescoping: %v", plan.Blockers)
		}
	}
	if !hasOperation(plan.Actions, "configure_firewall") {
		t.Fatalf("expected the firewall rule to be rewritten, got %#v", plan.Actions)
	}
}

func TestForeignScopeStillBlocksRescoping(t *testing.T) {
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.Firewall.Scopes = []string{"10.0.0.0/24"}
	plan := (Planner{}).Build(profile, snapshot)
	found := false
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "outside the requested scope") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a foreign scope must stay a blocker: %v", plan.Blockers)
	}
}

func TestTailscaleOwnedBroadRulesWarnInsteadOfBlocking(t *testing.T) {
	// Tailscale's installer adds an any-port inbound rule per firewall profile.
	// Blocking on it would make the recommended tailnet path unverifiable on
	// every machine that uses Tailscale, so it is reported, not a dead end.
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.Firewall.ThirdPartyBroadRules = []string{"Tailscale-In", "Tailscale-In"}
	plan := (Planner{}).Build(DefaultProfile(), snapshot)
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "Broad inbound firewall rules") {
			t.Fatalf("third-party transport rules must not block: %v", plan.Blockers)
		}
	}
	joined := strings.Join(plan.Warnings, " ")
	if !strings.Contains(joined, "Tailscale-In") {
		t.Fatalf("expected a precise warning naming the third-party rule: %v", plan.Warnings)
	}
}

func TestThirdPartyBroadRuleRecognition(t *testing.T) {
	cases := []struct {
		name     string
		display  string
		protocol string
		port     string
		scope    string
		program  string
		want     bool
	}{
		{"tailscale any-port rule", "Tailscale-In", "Any", "Any", "Any", "Any", true},
		{"tailscale narrow rule", "Tailscale-In", "TCP", "22", "Any", "Any", false},
		{"tailscale program rule", "Tailscale-Process", "Any", "Any", "Any", `C:\Program Files\Tailscale\tailscaled.exe`, false},
		{"unrelated broad rule", "SomeOtherApp", "Any", "Any", "Any", "Any", false},
	}
	for _, tc := range cases {
		got := isThirdPartyBroadRule(tc.display, tc.protocol, tc.port, tc.scope, tc.program, "", "")
		if got != tc.want {
			t.Fatalf("%s: isThirdPartyBroadRule = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStaleManagedRulesAreDisabledNotForeign(t *testing.T) {
	// Changing the SSH port must retire the previous managed rule instead of
	// leaving an enabled rule (or blocking the new scope) behind.
	profile := DefaultProfile()
	profile.SSH.Port = 2222
	profile.Exposure.Mode = "tailnet"
	profile.Transport.Mode = "tailnet"
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHPort = 22
	snapshot.Tailscale = TransportState{Installed: true, Online: true, IP: "100.64.0.1"}
	snapshot.Firewall.StaleManagedRules = []string{"SSH-Launchpad-TCP-22"}
	plan := (Planner{}).Build(profile, snapshot)
	if len(plan.Blockers) > 0 {
		t.Fatalf("a stale managed rule must not block the new port plan: %v", plan.Blockers)
	}
	var firewall *Action
	for index := range plan.Actions {
		if plan.Actions[index].Operation == "configure_firewall" {
			firewall = &plan.Actions[index]
		}
	}
	if firewall == nil {
		t.Fatalf("expected a configure_firewall action, got %#v", plan.Actions)
	}
	apply := strings.Join(firewall.Command, " ")
	if !strings.Contains(apply, "SSH-Launchpad-TCP-22") {
		t.Fatalf("the stale rule must be disabled by the plan: %s", apply)
	}
	rollback := strings.Join(firewall.RollbackCommand, " ")
	if !strings.Contains(rollback, "SSH-Launchpad-TCP-22") {
		t.Fatalf("the stale rule's prior state must be restorable: %s", rollback)
	}
}

func TestManagedFirewallRulePortRecognition(t *testing.T) {
	if port, ok := managedFirewallRulePort("SSH-Launchpad-TCP-2222"); !ok || port != 2222 {
		t.Fatalf("expected 2222, got %d/%v", port, ok)
	}
	for _, name := range []string{"SSH-Launchpad-TCP-", "SSH-Launchpad-TCP-abc", "OpenSSH-Server-In-TCP", "SSH-Launchpad-UDP-22"} {
		if _, ok := managedFirewallRulePort(name); ok {
			t.Fatalf("%q must not be treated as a managed TCP rule", name)
		}
	}
}

func TestStaleManagedRuleForcesCleanupEvenWhenScopeMatches(t *testing.T) {
	profile := DefaultProfile()
	profile.SSH.Port = 2222
	profile.Exposure.Mode = "tailnet"
	profile.Transport.Mode = "tailnet"
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHPort = 2222
	snapshot.Tailscale = TransportState{Installed: true, Online: true, IP: "100.64.0.1"}
	snapshot.Firewall.Scopes = []string{"100.64.0.0/10", "fd7a:115c:a1e0::/48"}
	snapshot.Firewall.ManagedScopes = append([]string(nil), snapshot.Firewall.Scopes...)
	snapshot.Firewall.Ports = []int{2222}
	snapshot.Firewall.StaleManagedRules = []string{"SSH-Launchpad-TCP-22"}
	plan := (Planner{}).Build(profile, snapshot)
	if !hasOperation(plan.Actions, "configure_firewall") {
		t.Fatalf("an enabled stale managed rule must still be retired: %#v", plan.Actions)
	}
}

func TestFirewallRollbackToleratesAlreadyRestoredRules(t *testing.T) {
	// Recovery is retried by operators and by the bounded auto-rollback. A rule
	// that is already gone or already restored must not make the recovery
	// report a failure.
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.Firewall.ConflictingRules = []string{"OpenSSH-Server-In-TCP"}
	snapshot.Firewall.Ports = nil
	plan := (Planner{}).Build(profile, snapshot)
	if len(plan.Blockers) > 0 {
		t.Fatalf("unexpected blockers: %v", plan.Blockers)
	}
	for _, action := range plan.Actions {
		if action.Operation != "configure_firewall" {
			continue
		}
		rollback := strings.Join(action.RollbackCommand, " ")
		if !strings.Contains(rollback, "-ErrorAction SilentlyContinue; if($data.had)") {
			t.Fatalf("managed rule lookup must tolerate a missing rule: %s", rollback)
		}
		if !strings.Contains(rollback, "Get-NetFirewallRule -Name $r.name -ErrorAction SilentlyContinue") {
			t.Fatalf("conflict restore must tolerate a missing rule: %s", rollback)
		}
		return
	}
	t.Fatalf("expected a configure_firewall action, got %#v", plan.Actions)
}

func TestWindowsRollbacksDoNotFailOnAbsentBackups(t *testing.T) {
	// PowerShell sets exit code 1 when the final statement errors even with
	// -ErrorAction SilentlyContinue, which made a completed Windows rollback
	// report "rollback-failed" and abort the remaining recovery actions. The
	// cleanup must therefore be guarded instead of silent.
	profile := DefaultProfile()
	profile.SSH.Port = 2222
	profile.SSH.PublicKeys = []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller"}
	snapshot := healthySnapshot(PlatformWindows)
	snapshot.SSHPort = 22
	snapshot.AuthorizedKeysMatch = false
	snapshot.Firewall.ConflictingRules = []string{"OpenSSH-Server-In-TCP"}
	snapshot.Firewall.Ports = nil
	plan := (Planner{}).Build(profile, snapshot)
	checked := map[string]bool{}
	for _, action := range plan.Actions {
		switch action.Operation {
		case "configure_sshd", "configure_keys", "configure_firewall":
			command := strings.Join(action.RollbackCommand, " ")
			if strings.Contains(command, "-ErrorAction SilentlyContinue; Remove-Item") ||
				strings.HasSuffix(strings.TrimSpace(command), "ErrorAction SilentlyContinue") {
				t.Fatalf("%s rollback must not end in a silent removal: %s", action.Operation, command)
			}
			if !strings.Contains(command, "if(Test-Path -LiteralPath $f){ Remove-Item -LiteralPath $f -Force -ErrorAction Stop }") {
				t.Fatalf("%s rollback must guard its cleanup: %s", action.Operation, command)
			}
			checked[action.Operation] = true
		}
	}
	for _, operation := range []string{"configure_sshd", "configure_keys", "configure_firewall"} {
		if !checked[operation] {
			t.Fatalf("expected a %s action in the plan, got %#v", operation, plan.Actions)
		}
	}
}
