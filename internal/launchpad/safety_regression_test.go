package launchpad

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type regressionRunner func(context.Context, []string, io.Writer) error

func (f regressionRunner) Run(ctx context.Context, a []string, w io.Writer) error {
	return f(ctx, a, w)
}

func TestSSHPolicyRejectsMatchAndIncludedPorts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sshd_config")
	for _, text := range []string{"PasswordAuthentication no\nMatch User alice\n PasswordAuthentication yes\n", "Match=User alice\nPasswordAuthentication yes\n", "ListenAddress 0.0.0.0:22\n"} {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if inspectSSHPolicy(path, PlatformLinux) == nil {
			t.Fatalf("accepted unsupported policy %q", text)
		}
	}
	child := filepath.Join(dir, "included.conf")
	os.WriteFile(child, []byte("Port 22\n"), 0600)
	os.WriteFile(path, []byte("Include included.conf\n"), 0600)
	if inspectSSHPolicy(path, PlatformLinux) == nil {
		t.Fatal("included Port accepted")
	}
	os.WriteFile(child, []byte("PasswordAuthentication no\n"), 0600)
	if err := inspectSSHPolicy(path, PlatformLinux); err != nil {
		t.Fatal(err)
	}
}

func TestSSHPolicyAllowsOnlyStockWindowsAdminMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshd_config")
	stock := "Match Group administrators\n AuthorizedKeysFile __PROGRAMDATA__/ssh/administrators_authorized_keys\n"
	os.WriteFile(path, []byte(stock), 0600)
	if err := inspectSSHPolicy(path, PlatformWindows); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte(stock+" PasswordAuthentication yes\n"), 0600)
	if inspectSSHPolicy(path, PlatformWindows) == nil {
		t.Fatal("unsafe admin override accepted")
	}
}

func TestSSHPortInventoryAndPlannerRejectExtraPorts(t *testing.T) {
	ports := parseConfiguredSSHPorts([]byte("port 2222\nport 22\nport 2222\n"))
	if len(ports) != 2 {
		t.Fatal(ports)
	}
	p := DefaultProfile()
	p.SSH.Port = 2222
	s := healthySnapshot(PlatformLinux)
	s.SSHPort = 2222
	s.SSHPorts = ports
	plan := (Planner{}).Build(p, s)
	found := false
	for _, a := range plan.Actions {
		if a.Operation == "configure_sshd" {
			found = true
			if !strings.Contains(strings.Join(a.Command, " "), `/^port(=|$)/`) {
				t.Fatal("old port not removed")
			}
		}
	}
	if !found || plan.NoChanges {
		t.Fatal("extra port falsely converged")
	}
	s.SSHPolicyError = "unsupported Match"
	plan = (Planner{}).Build(p, s)
	if len(plan.Blockers) == 0 {
		t.Fatal("unsupported policy must block")
	}
}

func TestFirewallInventoriesFailClosed(t *testing.T) {
	const header = "Status: active\nDefault: deny (incoming), allow (outgoing), disabled (routed)\n"
	const safe = "22/tcp ALLOW IN 100.64.0.0/10\n22/tcp (v6) ALLOW IN fd7a:115c:a1e0::/48\n"
	s, err := parseUFWInventory(header+safe, 22)
	if err != nil || !s.Checked {
		t.Fatalf("safe inventory: %+v %v", s, err)
	}
	for _, bad := range []string{"OpenSSH ALLOW Anywhere\n", "22/tcp DENY IN Anywhere\n", "Anywhere ALLOW 192.168.1.0/24\n"} {
		s, err := parseUFWInventory(header+safe+bad, 22)
		if err == nil || s.Checked {
			t.Fatalf("unsafe rule accepted %s", bad)
		}
	}
	if _, err := parseUFWInventory(safe, 22); err == nil {
		t.Fatal("unknown defaults accepted")
	}
	for _, action := range []string{"drop", "reject"} {
		if _, err := parseRichRules(`rule family="ipv4" source address="100.64.0.0/10" port port="22" protocol="tcp" `+action, 22); err == nil {
			t.Fatal("non-allow counted as allow")
		}
	}
}

func TestFirewallFirstFailurePropagates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	shim := `#!/bin/sh
printf '%s\n' "$*" >> "$AUDIT_CALLS"
case "$*" in *100.64.0.0/10*) exit 17;; *) exit 0;; esac
`
	if err := os.WriteFile(filepath.Join(dir, "ufw"), []byte(shim), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("AUDIT_CALLS", log)
	p := DefaultProfile()
	s := healthySnapshot(PlatformLinux)
	s.Firewall.Scopes = nil
	a := configureFirewallAction(p, s, exposureScopes(p, s))
	if err := exec.Command(a.Command[0], a.Command[1:]...).Run(); err == nil {
		t.Fatal("first failed add was masked")
	}
}

func TestFirewallRecoveryPreservesExistingScopes(t *testing.T) {
	p := DefaultProfile()
	s := healthySnapshot(PlatformLinux)
	s.Firewall.Scopes = []string{"100.64.0.0/10"}
	a := configureFirewallAction(p, s, exposureScopes(p, s))
	if strings.Contains(strings.Join(a.RollbackCommand, " "), "100.64.0.0/10") {
		t.Fatal("rollback deletes pre-existing rule")
	}
	if !strings.Contains(strings.Join(a.Command, " "), "fd7a:") {
		t.Fatal("missing scope not installed")
	}
}

func TestPartialActionRecoveryUsesFreshContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls []string
	mutated := false
	runner := regressionRunner(func(c context.Context, a []string, _ io.Writer) error {
		calls = append(calls, a[0])
		if a[0] == "mutate" {
			mutated = true
			cancel()
			return errors.New("partial failure")
		}
		if c.Err() != nil {
			t.Fatal("recovery inherits cancelled context")
		}
		mutated = false
		return nil
	})
	p := DefaultProfile()
	plan := Plan{Platform: PlatformLinux, Actions: []Action{{ID: "one", Mutating: true, Reversible: true, Command: []string{"mutate"}, RollbackCommand: []string{"undo"}}}}
	report, err := (Executor{Runner: runner}).Apply(ctx, p, plan, ApplyOptions{Confirmed: true, JournalDir: t.TempDir()})
	if err == nil || report.Success || mutated || len(calls) != 2 {
		t.Fatalf("partial mutation not recovered: %v %+v", calls, report)
	}
}

func TestWriteAheadIntentPersistsBeforeRunner(t *testing.T) {
	dir := t.TempDir()
	runner := regressionRunner(func(_ context.Context, _ []string, _ io.Writer) error {
		files, _ := filepath.Glob(filepath.Join(dir, "*.journal.json"))
		if len(files) != 1 {
			t.Fatal("no intent journal")
		}
		j, _, err := readJournal(files[0])
		if err != nil || !j.WriteAhead || !resultHasStatus(j.Results, "one", "running") {
			t.Fatalf("missing persisted in-flight action %+v %v", j, err)
		}
		return nil
	})
	plan := Plan{Platform: PlatformLinux, Actions: []Action{{ID: "one", Mutating: true, Command: []string{"test"}}}}
	if _, err := (Executor{Runner: runner}).Apply(context.Background(), DefaultProfile(), plan, ApplyOptions{Confirmed: true, JournalDir: dir}); err != nil {
		t.Fatal(err)
	}
}

func TestInterruptedJournalRecovery(t *testing.T) {
	for _, writeAhead := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "journal.json")
		j := Journal{SchemaVersion: SchemaVersion, ID: "test", Status: "running", WriteAhead: writeAhead, Actions: []Action{{ID: "one", Reversible: true, RollbackCommand: []string{"undo"}}}}
		if writeAhead {
			j.Results = []ActionResult{{ActionID: "one", Status: "running"}}
		}
		if err := writeJournalAtomic(path, &j); err != nil {
			t.Fatal(err)
		}
		calls := 0
		executor := Executor{Runner: regressionRunner(func(context.Context, []string, io.Writer) error { calls++; return nil })}
		report, err := executor.Rollback(context.Background(), path)
		if writeAhead {
			if err != nil || !report.Success || calls != 1 {
				t.Fatalf("in-flight recovery failed %+v %v", report, err)
			}
		} else if err == nil || report.Success || calls != 0 {
			t.Fatal("legacy uncertain journal falsely reports recovery")
		}
	}
}

func TestRepairMissingBinaryDefersAuthenticationChecks(t *testing.T) {
	p := DefaultProfile()
	s := healthySnapshot(PlatformWindows)
	missing := false
	s.SSHServer.BinaryExists = &missing
	s.SSHConfigValid = false
	s.SSHAuthenticationChecked = false
	s.AuthorizedKeysChecked = false
	plan := (Planner{}).Build(p, s)
	if len(plan.Blockers) != 0 || len(plan.Actions) != 1 || plan.Actions[0].Operation != "repair_ssh_install" {
		t.Fatalf("repair dead-ended: %+v", plan)
	}
}

func TestRunningDisabledServiceRecoveryPreservesRunning(t *testing.T) {
	s := healthySnapshot(PlatformLinux)
	s.SSHService.StartPolicy = "disabled"
	a := enableSSHAction(DefaultProfile(), s)
	cmd := strings.Join(a.RollbackCommand, " ")
	if !strings.Contains(cmd, "systemctl start") || strings.Contains(cmd, "--now") || strings.Contains(cmd, "systemctl stop") {
		t.Fatal(cmd)
	}
}

func TestUnsupportedDependencyStrategyDoesNotFallback(t *testing.T) {
	for _, strategy := range []string{"proxy", "mirror", "cache", "offline"} {
		p := DefaultProfile()
		p.Download.Strategy = strategy
		s := healthySnapshot(PlatformWindows)
		s.SSHServer = Capability{}
		plan := (Planner{}).Build(p, s)
		if len(plan.Blockers) == 0 {
			t.Fatalf("%s silently falls back online", strategy)
		}
	}
}

func TestRollbackDigestBindsExactJournalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.json")
	j := Journal{SchemaVersion: SchemaVersion, ID: "bound", Status: "completed"}
	writeJournalAtomic(path, &j)
	sum, err := FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	j.ProfileName = "changed"
	writeJournalAtomic(path, &j)
	report, err := (Executor{}).RollbackVerified(context.Background(), path, sum)
	if err == nil || report.Success {
		t.Fatal("changed journal accepted across elevation")
	}
}
