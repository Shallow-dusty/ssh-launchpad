package launchpad

import (
	"context"
	"strings"
	"testing"
)

type sequenceProbe struct {
	snapshots []Snapshot
	index     int
}

func (p *sequenceProbe) Check(_ context.Context, _ Profile) (Snapshot, error) {
	if p.index >= len(p.snapshots) {
		return p.snapshots[len(p.snapshots)-1], nil
	}
	snapshot := p.snapshots[p.index]
	p.index++
	return snapshot, nil
}

func TestVerifyFailsClosedOnInvalidSSHOrFirewallEvidence(t *testing.T) {
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformLinux)
	snapshot.SSHConfigValid = false
	snapshot.Firewall.Scopes = append(snapshot.Firewall.Scopes, "203.0.113.0/24")
	engine := NewEngine(nil)
	engine.Probe = &sequenceProbe{snapshots: []Snapshot{snapshot}}
	report, err := engine.Verify(context.Background(), profile)
	if err == nil || report.Success || report.ExitCode != ExitVerificationFailed {
		t.Fatalf("Verify accepted unsafe evidence: %+v %v", report, err)
	}
}

func TestApplyRequiresTheReviewedPlanDigest(t *testing.T) {
	profile := DefaultProfile()
	snapshot := healthySnapshot(PlatformLinux)
	probe := &sequenceProbe{snapshots: []Snapshot{snapshot}}
	engine := NewEngine(nil)
	engine.Probe = probe
	engine.Executor.Runner = &recordingRunner{}
	engine.Executor.AdministratorCheck = func(context.Context, Platform) bool { return true }

	report, err := engine.Apply(context.Background(), profile, ApplyOptions{Confirmed: true, JournalDir: t.TempDir()})
	if err == nil || report.ExitCode != ExitConfirmationRequired {
		t.Fatalf("missing reviewed digest was accepted: %+v %v", report, err)
	}
	report, err = engine.Apply(context.Background(), profile, ApplyOptions{Confirmed: true, ExpectedPlanDigest: strings.Repeat("0", 64), JournalDir: t.TempDir()})
	if err == nil || report.ExitCode != ExitConfirmationRequired {
		t.Fatalf("stale reviewed digest was accepted: %+v %v", report, err)
	}
}

func TestRepeatedApplyBecomesNoOpAfterStateMatches(t *testing.T) {
	profile := DefaultProfile()
	first := healthySnapshot(PlatformLinux)
	first.SSHService.Running = false
	first.Firewall = FirewallState{Checked: true, Enabled: true, Provider: first.Firewall.Provider}
	second := healthySnapshot(PlatformLinux)
	probe := &sequenceProbe{snapshots: []Snapshot{first, second}}
	runner := &recordingRunner{}
	engine := NewEngine(nil)
	engine.Probe = probe
	engine.Executor.Runner = runner
	engine.Executor.AdministratorCheck = func(context.Context, Platform) bool { return true }
	options := ApplyOptions{
		Confirmed:          true,
		ExpectedPlanDigest: (Planner{}).Build(profile, first).Digest,
		JournalDir:         t.TempDir(),
	}

	initial, err := engine.Apply(context.Background(), profile, options)
	if err != nil || !initial.Success || len(runner.commands) == 0 {
		t.Fatalf("first Apply should execute drift: %+v %v", initial, err)
	}
	commandCount := len(runner.commands)
	options.ExpectedPlanDigest = (Planner{}).Build(profile, second).Digest
	repeated, err := engine.Apply(context.Background(), profile, options)
	if err != nil || !repeated.Success || repeated.Plan == nil || !repeated.Plan.NoChanges {
		t.Fatalf("second Apply should be a no-op: %+v %v", repeated, err)
	}
	if len(runner.commands) != commandCount {
		t.Fatal("second Apply executed additional commands")
	}
}
