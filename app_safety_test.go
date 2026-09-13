package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

type safetyTestProbe struct{}

func (safetyTestProbe) Check(context.Context, launchpad.Profile) (launchpad.Snapshot, error) {
	return launchpad.Snapshot{Platform: launchpad.PlatformLinux}, nil
}

func TestPrivateExportTightensExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission postcondition")
	}
	path := filepath.Join(t.TempDir(), "card.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("existing export retained mode %o", info.Mode().Perm())
	}
}

func TestBeginElevatedApplyReturnsStableSnapshotBeforeAsyncJobStarts(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("LocalAppData", cache)
	app := NewApp()
	app.ctx = context.Background()
	app.engine.Probe = safetyTestProbe{}
	profile := launchpad.DefaultProfile()
	profile.SSH.Enabled = false
	plan := (launchpad.Planner{}).Build(profile, launchpad.Snapshot{Platform: launchpad.PlatformLinux})
	job, err := app.BeginElevatedApply(DesktopRequest{
		Stage:         launchpad.StageApply,
		Profile:       profile,
		PlanDigest:    plan.Digest,
		Confirmed:     true,
		PlanNoChanges: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "waiting-for-permission" || job.Report != nil || job.Error != "" {
		t.Fatalf("async job snapshot was overwritten before return: %+v", job)
	}
	defer app.DismissElevatedJob(job.ID)
	deadline := time.Now().Add(5 * time.Second)
	for {
		status, err := app.ElevatedApplyStatus(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if status.State == "completed" || status.State == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("test job did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}
