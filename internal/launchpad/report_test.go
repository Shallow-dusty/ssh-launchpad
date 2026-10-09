package launchpad

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestFailureReasonsDoNotChangePreMutationGates(t *testing.T) {
	action := Action{ID: "test", Mutating: true, Command: []string{"never-executed"}}
	for _, test := range []struct {
		name      string
		plan      Plan
		confirmed bool
		code      int
		reason    FailureReason
	}{
		{"confirmation", Plan{Actions: []Action{action}}, false, ExitConfirmationRequired, ReasonConfirmationRequired},
		{"blocker", Plan{Blockers: []string{"blocked"}}, true, ExitVerificationFailed, ReasonVerificationFailed},
		{"manual", Plan{Actions: []Action{{ID: "manual"}}}, true, ExitUnsupported, ReasonUnsupported},
		{"self-cut", Plan{SelfCutDetected: true, Actions: []Action{action}}, true, ExitSelfCutBlocked, ReasonSelfCutBlocked},
		{"elevation", Plan{Platform: PlatformWindows, Actions: []Action{{ID: "admin", Mutating: true, RequiresElevation: true, Command: action.Command}}}, true, ExitNeedsElevation, ReasonElevationRequired},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &recordingRunner{}
			executor := Executor{Runner: runner, AdministratorCheck: func(context.Context, Platform) bool { return false }}
			directory := t.TempDir()
			report, err := executor.Apply(context.Background(), DefaultProfile(), test.plan, ApplyOptions{Confirmed: test.confirmed, JournalDir: directory})
			if err == nil || report.Success || report.ExitCode != test.code || report.ReasonCode != test.reason {
				t.Fatalf("unexpected gate result: %+v %v", report, err)
			}
			entries, readErr := os.ReadDir(directory)
			if readErr != nil || len(entries) != 0 || len(runner.commands) != 0 {
				t.Fatalf("failed gate caused a side effect: commands=%v entries=%v err=%v", runner.commands, entries, readErr)
			}
		})
	}
}

func TestDigestAndMutationLockHaveDistinctReasonsButStableExitCodes(t *testing.T) {
	profile := DefaultProfile()
	profile.SSH.Enabled = false
	engine := NewEngine(nil)
	engine.Probe = &sequenceProbe{snapshots: []Snapshot{{Platform: PlatformLinux}}}
	for _, test := range []struct {
		digest string
		reason FailureReason
	}{
		{"", ReasonConfirmationRequired},
		{strings.Repeat("0", 64), ReasonPlanChanged},
	} {
		report, err := engine.Apply(context.Background(), profile, ApplyOptions{Confirmed: true, ExpectedPlanDigest: test.digest})
		if err == nil || report.ExitCode != ExitConfirmationRequired || report.ReasonCode != test.reason {
			t.Fatalf("reason mismatch: %+v %v", report, err)
		}
	}
	_, release, err := mutationContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	report, err := engine.Apply(context.Background(), profile, ApplyOptions{})
	if err == nil || report.ExitCode != ExitConfirmationRequired || report.ReasonCode != ReasonMutationBusy {
		t.Fatalf("busy lock was mistaken for a changed plan: %+v %v", report, err)
	}
}

func TestReasonMetadataIsAdditiveAndOptional(t *testing.T) {
	for _, reason := range []FailureReason{"", ReasonPlanChanged} {
		report := Report{SchemaVersion: SchemaVersion, ExitCode: ExitConfirmationRequired, Error: "unchanged prose", ReasonCode: reason}
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "reasonCode") != (reason != "") {
			t.Fatalf("reason optionality changed: %s", data)
		}
		var legacy struct {
			SchemaVersion int    `json:"schemaVersion"`
			ExitCode      int    `json:"exitCode"`
			Error         string `json:"error"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil || legacy.SchemaVersion != 1 || legacy.ExitCode != 5 || legacy.Error != report.Error {
			t.Fatalf("legacy fields changed: %+v %v", legacy, err)
		}
	}
	failure, err := finishReportError(Report{Stage: StageRollback}, ExitPartialFailure, errors.New("failure"))
	if err == nil || failure.ReasonCode != ReasonRollbackFailed || failure.ExitCode != ExitPartialFailure {
		t.Fatalf("rollback reason missing: %+v %v", failure, err)
	}
}
