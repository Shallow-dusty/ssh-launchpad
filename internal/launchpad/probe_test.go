package launchpad

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestCheckBudgetExceededStillReturnsSnapshot pins the field behavior from the
// 2026-08-30 deployment: on a consumer Windows machine whose antivirus
// suspends helper commands, Check must return a partial snapshot with a
// recorded, actionable probe error instead of hanging on the prepare step.
func TestCheckBudgetExceededStillReturnsSnapshot(t *testing.T) {
	original := probeOverallBudget
	probeOverallBudget = time.Nanosecond
	defer func() { probeOverallBudget = original }()

	snapshot, err := SystemProbe{}.Check(context.Background(), Profile{})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	var overall string
	for _, probeErr := range snapshot.ProbeErrors {
		if strings.Contains(probeErr, "overall:") {
			overall = probeErr
		}
	}
	if overall == "" {
		t.Fatalf("expected an overall probe error in %v", snapshot.ProbeErrors)
	}
	if !strings.Contains(overall, "security software") {
		t.Fatalf("overall probe error should attribute the stall, got %q", overall)
	}
}

// TestCheckParentCancelKeepsSnapshotSilent verifies that a canceled parent
// context (for example during app shutdown) does not append the security-
// software attribution: the stall diagnosis is only meaningful when the
// budget, not the caller, ended the probes.
func TestCheckParentCancelKeepsSnapshotSilent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	snapshot, err := SystemProbe{}.Check(ctx, Profile{})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	for _, probeErr := range snapshot.ProbeErrors {
		if strings.Contains(probeErr, "overall:") {
			t.Fatalf("parent cancellation should not add the overall attribution, got %q", probeErr)
		}
	}
}
