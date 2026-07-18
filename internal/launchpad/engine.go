package launchpad

import (
	"context"
	"errors"
	"time"
)

type Engine struct {
	Probe    Probe
	Planner  Planner
	Executor Executor
}

func NewEngine(sink EventSink) *Engine {
	return &Engine{Probe: SystemProbe{}, Planner: Planner{}, Executor: Executor{Sink: sink}}
}

func (e *Engine) Check(ctx context.Context, profile Profile) (Report, error) {
	started := time.Now().UTC()
	report := newReport(StageCheck, profile.Name, started)
	snapshot, err := e.Probe.Check(ctx, profile)
	report.Snapshot = &snapshot
	report.Finished = time.Now().UTC()
	if err != nil {
		report.ExitCode = ExitVerificationFailed
		report.Error = err.Error()
		return report, err
	}
	report.Success = true
	report.ExitCode = ExitOK
	return report, nil
}

func (e *Engine) Plan(ctx context.Context, profile Profile) (Report, error) {
	started := time.Now().UTC()
	report := newReport(StagePlan, profile.Name, started)
	snapshot, err := e.Probe.Check(ctx, profile)
	report.Snapshot = &snapshot
	if err != nil {
		report.ExitCode = ExitVerificationFailed
		report.Error = err.Error()
		report.Finished = time.Now().UTC()
		return report, err
	}
	plan := e.Planner.Build(profile, snapshot)
	report.Plan = &plan
	report.Success = true
	report.ExitCode = ExitOK
	report.Finished = time.Now().UTC()
	return report, nil
}

func (e *Engine) Apply(ctx context.Context, profile Profile, opts ApplyOptions) (Report, error) {
	planReport, err := e.Plan(ctx, profile)
	if err != nil {
		return planReport, err
	}
	return e.Executor.Apply(ctx, profile, *planReport.Plan, opts)
}

func (e *Engine) Verify(ctx context.Context, profile Profile) (Report, error) {
	started := time.Now().UTC()
	report := newReport(StageVerify, profile.Name, started)
	snapshot, err := e.Probe.Check(ctx, profile)
	report.Snapshot = &snapshot
	if err != nil {
		report.ExitCode = ExitVerificationFailed
		report.Error = err.Error()
		report.Finished = time.Now().UTC()
		return report, err
	}
	plan := e.Planner.Build(profile, snapshot)
	report.Plan = &plan
	report.Finished = time.Now().UTC()
	if !plan.NoChanges {
		report.ExitCode = ExitVerificationFailed
		report.Error = "Verification found remaining drift."
		report.Warnings = append(report.Warnings, plan.Warnings...)
		return report, errors.New(report.Error)
	}
	report.Success = true
	report.ExitCode = ExitOK
	return report, nil
}
