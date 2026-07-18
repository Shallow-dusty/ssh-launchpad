package launchpad

import (
	"context"
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

func TestRepeatedApplyBecomesNoOpAfterStateMatches(t *testing.T) {
	profile := DefaultProfile()
	first := healthySnapshot(detectPlatform())
	first.SSHService.Running = false
	first.Firewall = FirewallState{Provider: first.Firewall.Provider}
	second := healthySnapshot(detectPlatform())
	probe := &sequenceProbe{snapshots: []Snapshot{first, second}}
	runner := &recordingRunner{}
	engine := NewEngine(nil)
	engine.Probe = probe
	engine.Executor.Runner = runner
	engine.Executor.AdministratorCheck = func(context.Context, Platform) bool { return true }
	options := ApplyOptions{Confirmed: true, JournalDir: t.TempDir()}

	initial, err := engine.Apply(context.Background(), profile, options)
	if err != nil || !initial.Success || len(runner.commands) == 0 {
		t.Fatalf("first Apply should execute drift: %+v %v", initial, err)
	}
	commandCount := len(runner.commands)
	repeated, err := engine.Apply(context.Background(), profile, options)
	if err != nil || !repeated.Success || repeated.Plan == nil || !repeated.Plan.NoChanges {
		t.Fatalf("second Apply should be a no-op: %+v %v", repeated, err)
	}
	if len(runner.commands) != commandCount {
		t.Fatal("second Apply executed additional commands")
	}
}
