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

func TestIsAdminDefaultAuthorizedKeysPath(t *testing.T) {
	admin := Snapshot{TargetUserIsAdmin: true}
	cases := []struct {
		configured string
		want       bool
	}{
		{".ssh/authorized_keys", true},
		{`C:\ProgramData\ssh\administrators_authorized_keys`, false},
		{`__PROGRAMDATA__/ssh/administrators_authorized_keys`, false},
		{".ssh/authorized_keys .ssh/authorized_keys2", false},
		{"%h/.ssh/authorized_keys", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isAdminDefaultAuthorizedKeysPath(tc.configured, admin); got != tc.want {
			t.Errorf("isAdminDefaultAuthorizedKeysPath(%q) = %v, want %v", tc.configured, got, tc.want)
		}
	}
}
