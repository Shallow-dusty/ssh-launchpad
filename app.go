package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	mu     sync.Mutex
	engine *launchpad.Engine
	jobs   map[string]*elevatedJobRecord
}

type DesktopRequest struct {
	Stage          launchpad.Stage   `json:"stage"`
	Profile        launchpad.Profile `json:"profile"`
	PlanDigest     string            `json:"planDigest,omitempty"`
	Confirmed      bool              `json:"confirmed"`
	AllowSelfCut   bool              `json:"allowSelfCut"`
	ScheduleRisky  bool              `json:"scheduleRisky"`
	ExternalVerify string            `json:"externalVerify"`
	// PlanNoChanges and PlanNeedsElevation are routing hints taken from the
	// plan the user just reviewed. They only select the execution path
	// (direct vs elevated, apply vs no-op); the engine re-plans inside Apply
	// and rejects the request if the digest no longer matches, so a stale
	// or dishonest hint fails closed instead of changing the wrong thing.
	PlanNoChanges      bool `json:"planNoChanges,omitempty"`
	PlanNeedsElevation bool `json:"planNeedsElevation,omitempty"`
}

func NewApp() *App {
	app := &App{jobs: map[string]*elevatedJobRecord{}}
	app.engine = launchpad.NewEngine(func(event launchpad.Event) {
		app.mu.Lock()
		ctx := app.ctx
		app.mu.Unlock()
		if ctx != nil {
			wailsruntime.EventsEmit(ctx, "launchpad:event", event)
		}
	})
	return app
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ctx = ctx
}

func (a *App) DefaultProfile() launchpad.Profile {
	return normalizeDesktopProfile(launchpad.DefaultProfile())
}

func (a *App) CheckForUpdate() (launchpad.UpdateInfo, error) {
	return launchpad.CheckForUpdate(a.ctx)
}

func (a *App) Run(request DesktopRequest) (launchpad.Report, error) {
	request.Profile = normalizeDesktopProfile(request.Profile)
	if err := request.Profile.Validate(); err != nil {
		return desktopInputFailure(request, err.Error()), nil
	}
	var report launchpad.Report
	var runErr error
	switch request.Stage {
	case launchpad.StageCheck:
		report, runErr = a.engine.Check(a.ctx, request.Profile)
	case launchpad.StagePlan:
		report, runErr = a.engine.Plan(a.ctx, request.Profile)
	case launchpad.StageApply:
		report, runErr = a.engine.Apply(a.ctx, request.Profile, launchpad.ApplyOptions{
			Confirmed:          request.Confirmed,
			ExpectedPlanDigest: request.PlanDigest,
			AllowSelfCut:       request.AllowSelfCut,
			ScheduleRisky:      request.ScheduleRisky,
			AutoRollback:       request.Profile.Safety.AutoRollback,
			ExternalVerify:     request.ExternalVerify,
		})
	case launchpad.StageVerify:
		report, runErr = a.engine.Verify(a.ctx, request.Profile)
	default:
		return desktopInputFailure(request, "unsupported stage"), nil
	}
	// A Wails promise rejection discards the report and its reason code. Keep
	// expected engine failures as data; the frontend adapter presents them.
	if runErr != nil && report.Error == "" {
		report.Error = runErr.Error()
	}
	return report, nil
}

func desktopInputFailure(request DesktopRequest, message string) launchpad.Report {
	return launchpad.Report{
		SchemaVersion: launchpad.SchemaVersion,
		Version:       launchpad.Version,
		Stage:         request.Stage,
		ProfileName:   request.Profile.Name,
		ExitCode:      launchpad.ExitInvalidProfile,
		ReasonCode:    launchpad.ReasonInvalidProfile,
		Error:         message,
	}
}

func normalizeDesktopProfile(profile launchpad.Profile) launchpad.Profile {
	if runtime.GOOS == "windows" && strings.TrimSpace(profile.Advanced.StateDir) == "" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		profile.Advanced.StateDir = filepath.Join(programData, "SSH Launchpad")
	}
	return profile
}

func desktopApplyOptions(request DesktopRequest) launchpad.ApplyOptions {
	return launchpad.ApplyOptions{
		Confirmed:          true,
		ExpectedPlanDigest: request.PlanDigest,
		AllowSelfCut:       request.AllowSelfCut,
		ScheduleRisky:      request.ScheduleRisky,
		AutoRollback:       request.Profile.Safety.AutoRollback,
		ExternalVerify:     request.ExternalVerify,
	}
}

func (a *App) Rollback(journalPath string) (launchpad.Report, error) {
	if runtime.GOOS == "windows" && !launchpad.CurrentProcessElevated(a.ctx) {
		return a.elevatedRollback(journalPath)
	}
	report, err := a.engine.Executor.Rollback(a.ctx, journalPath)
	// Wails rejects promises when error != nil, discarding useful partial
	// recovery evidence. Expected execution failures belong in the report.
	if err != nil && report.Error == "" {
		report.Error = err.Error()
	}
	return report, nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
