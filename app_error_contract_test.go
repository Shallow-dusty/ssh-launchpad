package main

import (
	"context"
	"errors"
	"testing"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

type errorContractProbe struct{ calls int }

func (p *errorContractProbe) Check(context.Context, launchpad.Profile) (launchpad.Snapshot, error) {
	p.calls++
	return launchpad.Snapshot{Platform: launchpad.PlatformLinux}, errors.New("probe failed, not a checksum classification")
}

func TestDesktopStageFailuresRetainReportsInsteadOfRejectingWailsPromises(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	probe := &errorContractProbe{}
	app.engine.Probe = probe
	for _, stage := range []launchpad.Stage{launchpad.StageCheck, launchpad.StagePlan, launchpad.StageVerify} {
		report, rpcErr := app.Run(DesktopRequest{Stage: stage, Profile: launchpad.DefaultProfile()})
		if rpcErr != nil || report.Success || report.ExitCode != launchpad.ExitVerificationFailed || report.ReasonCode != launchpad.ReasonProbeFailed || report.Error == "" {
			t.Fatalf("RPC lost structured stage failure: %+v %v", report, rpcErr)
		}
	}
	if probe.calls != 3 {
		t.Fatalf("unexpected probe calls: %d", probe.calls)
	}
}

func TestDesktopInvalidInputIsAReadOnlyStructuredFailure(t *testing.T) {
	app := NewApp()
	probe := &errorContractProbe{}
	app.engine.Probe = probe
	profile := launchpad.DefaultProfile()
	profile.SSH.Port = 0
	report, rpcErr := app.Run(DesktopRequest{Stage: launchpad.StageCheck, Profile: profile})
	if rpcErr != nil || report.ExitCode != launchpad.ExitInvalidProfile || report.ReasonCode != launchpad.ReasonInvalidProfile || report.SchemaVersion != 1 || probe.calls != 0 {
		t.Fatalf("invalid input crossed the engine boundary: %+v %v calls=%d", report, rpcErr, probe.calls)
	}
}

func TestJobStatesRetainTheirWireValues(t *testing.T) {
	for state, want := range map[JobState]string{
		JobWaitingForPermission: "waiting-for-permission", JobRunning: "running",
		JobCompleted: "completed", JobFailed: "failed", JobCancelled: "cancelled",
	} {
		if string(state) != want {
			t.Fatalf("job state changed: %s != %s", state, want)
		}
	}
}
