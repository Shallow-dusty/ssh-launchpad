package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	mu     sync.Mutex
	engine *launchpad.Engine
}

type DesktopRequest struct {
	Stage          launchpad.Stage   `json:"stage"`
	Profile        launchpad.Profile `json:"profile"`
	Confirmed      bool              `json:"confirmed"`
	AllowSelfCut   bool              `json:"allowSelfCut"`
	ScheduleRisky  bool              `json:"scheduleRisky"`
	ExternalVerify string            `json:"externalVerify"`
}

func NewApp() *App {
	app := &App{}
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
	return launchpad.DefaultProfile()
}

func (a *App) Run(request DesktopRequest) (launchpad.Report, error) {
	if err := request.Profile.Validate(); err != nil {
		return launchpad.Report{ExitCode: launchpad.ExitInvalidProfile, Error: err.Error()}, err
	}
	switch request.Stage {
	case launchpad.StageCheck:
		return a.engine.Check(a.ctx, request.Profile)
	case launchpad.StagePlan:
		return a.engine.Plan(a.ctx, request.Profile)
	case launchpad.StageApply:
		return a.engine.Apply(a.ctx, request.Profile, launchpad.ApplyOptions{
			Confirmed:      request.Confirmed,
			AllowSelfCut:   request.AllowSelfCut,
			ScheduleRisky:  request.ScheduleRisky,
			AutoRollback:   request.Profile.Safety.AutoRollback,
			ExternalVerify: request.ExternalVerify,
		})
	case launchpad.StageVerify:
		return a.engine.Verify(a.ctx, request.Profile)
	default:
		return launchpad.Report{ExitCode: launchpad.ExitInvalidProfile}, errors.New("unsupported stage")
	}
}

func (a *App) Rollback(journalPath string) (launchpad.Report, error) {
	return a.engine.Executor.Rollback(a.ctx, journalPath)
}

func (a *App) ExportReport(report launchpad.Report) (string, error) {
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export SSH Launchpad report",
		DefaultFilename: report.ID + ".report.json",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON report", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(data, '\n'), 0o600)
}
