package launchpad

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func TestCancelledDelayNeverStartsOrRecoversAction(t *testing.T) {
	calls := 0
	executor := Executor{
		Runner: regressionRunner(func(context.Context, []string, io.Writer) error { calls++; return nil }),
		Delay: func(ctx context.Context, _ time.Duration) error {
			if ctx.Value(mutationLockKey{}) == nil {
				t.Fatal("delay lost mutation lock")
			}
			return context.Canceled
		},
	}
	// The action is risky but the snapshot did not identify an active channel.
	plan := Plan{Platform: detectPlatform(), Actions: []Action{{ID: "delayed", Mutating: true, SelfCutRisk: true, Reversible: true, Command: []string{"forward"}, RollbackCommand: []string{"undo"}}}}
	report, err := executor.Apply(context.Background(), DefaultProfile(), plan, ApplyOptions{Confirmed: true, ScheduleRisky: true, AutoRollback: true, JournalDir: t.TempDir()})
	if !errors.Is(err, context.Canceled) || report.Success || calls != 0 {
		t.Fatalf("cancelled action executed: %+v %v calls=%d", report, err, calls)
	}
	if !resultHasStatus(report.Results, "delayed", "not-started") {
		t.Fatal("cancelled delay recorded as attempted")
	}
	recovered, err := executor.Rollback(context.Background(), report.JournalPath)
	if err != nil || !recovered.Success || calls != 0 {
		t.Fatalf("recovery executed unstarted action: %+v %v", recovered, err)
	}
}

func TestSelfCutOverrideStillRequiresExternalEndpoint(t *testing.T) {
	plan := Plan{SelfCutDetected: true, Actions: []Action{{ID: "risk", Mutating: true, Command: []string{"must-not-run"}}}}
	report, err := (Executor{}).Apply(context.Background(), DefaultProfile(), plan, ApplyOptions{Confirmed: true, AllowSelfCut: true, JournalDir: t.TempDir()})
	if err == nil || report.ExitCode != ExitSelfCutBlocked {
		t.Fatalf("override bypassed external evidence: %+v %v", report, err)
	}
}

func TestUncertainRecoveryCannotClaimSuccess(t *testing.T) {
	for _, status := range []string{"scheduled", "running", "failed"} {
		t.Run(status, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.journal.json")
			journal := Journal{SchemaVersion: SchemaVersion, ID: "test", WriteAhead: true, Status: "failed", Actions: []Action{{ID: "uncertain", Mutating: true, Reversible: status == "scheduled", RollbackCommand: []string{"undo"}}}, Results: []ActionResult{{ActionID: "uncertain", Status: status}}}
			if err := writeJournalAtomic(path, &journal); err != nil {
				t.Fatal(err)
			}
			calls := 0
			executor := Executor{Runner: regressionRunner(func(context.Context, []string, io.Writer) error { calls++; return nil })}
			for i := 0; i < 2; i++ {
				report, err := executor.Rollback(context.Background(), path)
				if err == nil || report.Success || calls != 0 {
					t.Fatalf("uncertain action accepted: %+v %v calls=%d", report, err, calls)
				}
			}
		})
	}
}
