package launchpad

import (
	"context"
	"errors"
	"strings"
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
	if opts.Confirmed {
		if strings.TrimSpace(opts.ExpectedPlanDigest) == "" {
			return failedApplyReport(profile.Name, planReport.Plan, ExitConfirmationRequired, "Apply requires the digest of the explicitly reviewed plan.")
		}
		if !strings.EqualFold(strings.TrimSpace(opts.ExpectedPlanDigest), planReport.Plan.Digest) {
			return failedApplyReport(profile.Name, planReport.Plan, ExitConfirmationRequired, "The machine state or profile changed after Plan. Review and confirm the new plan before Apply.")
		}
	}
	return e.Executor.Apply(ctx, profile, *planReport.Plan, opts)
}

func failedApplyReport(profile string, plan *Plan, code int, message string) (Report, error) {
	report := newReport(StageApply, profile, time.Now().UTC())
	report.Plan = plan
	report.ExitCode = code
	report.Error = message
	report.Finished = time.Now().UTC()
	return report, errors.New(message)
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
	if !plan.NoChanges || len(plan.Blockers) > 0 {
		report.ExitCode = ExitVerificationFailed
		report.Error = "Verification found remaining drift."
		if len(plan.Blockers) > 0 {
			report.Error = "Verification is blocked: " + strings.Join(plan.Blockers, " ")
		}
		report.Warnings = append(report.Warnings, plan.Warnings...)
		return report, errors.New(report.Error)
	}
	report.Success = true
	report.ExitCode = ExitOK
	return report, nil
}
