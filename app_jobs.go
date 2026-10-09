package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	elevationprotocol "github.com/Shallow-dusty/ssh-launchpad/internal/elevation"
	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func (a *App) BeginElevatedApply(request DesktopRequest) (ElevatedJob, error) {
	if request.Stage != launchpad.StageApply || !request.Confirmed {
		return ElevatedJob{}, errors.New("safe install requires an explicitly confirmed Apply request")
	}
	request.Profile = normalizeDesktopProfile(request.Profile)
	if err := request.Profile.Validate(); err != nil {
		return ElevatedJob{}, err
	}
	// The authoritative digest check happens inside Apply's own re-plan;
	// here we only need the digest to be well-formed so the elevated helper
	// accepts the handoff.
	if decoded, err := hex.DecodeString(strings.TrimSpace(request.PlanDigest)); err != nil || len(decoded) != sha256.Size {
		return ElevatedJob{}, errors.New("safe install requires the digest of the reviewed plan")
	}
	if request.PlanNoChanges {
		report, applyErr := a.engine.Apply(a.ctx, request.Profile, desktopApplyOptions(request))
		state := JobFailed
		if report.Success {
			state = "completed"
		}
		return ElevatedJob{ID: report.ID, State: state, Report: &report, Error: errorText(applyErr)}, nil
	}
	id, err := newJobID()
	if err != nil {
		return ElevatedJob{}, err
	}
	root, err := jobRoot()
	if err != nil {
		return ElevatedJob{}, err
	}
	pruneOldJobs(root)
	directory := filepath.Join(root, id)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return ElevatedJob{}, err
	}
	record := &elevatedJobRecord{
		status:       ElevatedJob{ID: id, State: "waiting-for-permission"},
		directory:    directory,
		responsePath: filepath.Join(directory, "response.json"),
		eventsPath:   filepath.Join(directory, "events.jsonl"),
	}
	a.mu.Lock()
	a.jobs[id] = record
	a.mu.Unlock()

	if runtime.GOOS == "windows" && request.PlanNeedsElevation && !launchpad.CurrentProcessElevated(a.ctx) {
		for _, path := range []string{record.responsePath, record.eventsPath} {
			if err := elevationprotocol.PrecreateFile(path); err != nil {
				a.DismissElevatedJob(id)
				return ElevatedJob{}, err
			}
		}
		requestPath := filepath.Join(directory, "request.json")
		elevatedRequest := elevationprotocol.NewRequest(
			request.Profile,
			desktopApplyOptions(request),
			record.responsePath,
			record.eventsPath,
			"",
		)
		digest, err := elevationprotocol.WriteRequest(requestPath, elevatedRequest)
		if err != nil {
			a.DismissElevatedJob(id)
			return ElevatedJob{}, err
		}
		executable, err := os.Executable()
		if err != nil {
			a.DismissElevatedJob(id)
			return ElevatedJob{}, err
		}
		initial := record.status
		go a.runUACJob(record, executable, requestPath, digest)
		return initial, nil
	}

	initial := record.status
	go a.runDirectJob(record, request)
	return initial, nil
}

func (a *App) ElevatedApplyStatus(id string) (ElevatedJob, error) {
	a.mu.Lock()
	record := a.jobs[id]
	a.mu.Unlock()
	if record == nil {
		return ElevatedJob{}, errors.New("safe install job was not found")
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	status := record.status
	status.Events = readJobEvents(record.eventsPath)
	if status.State == "waiting-for-permission" && len(status.Events) > 0 {
		status.State = "running"
	}
	return status, nil
}

func (a *App) DismissElevatedJob(id string) {
	a.mu.Lock()
	record := a.jobs[id]
	delete(a.jobs, id)
	a.mu.Unlock()
	if record != nil {
		_ = os.RemoveAll(record.directory)
	}
}

func (a *App) runUACJob(record *elevatedJobRecord, executable, requestPath, digest string) {
	defer os.Remove(requestPath)
	err := launchElevatedHelper(context.Background(), executable, requestPath, digest)
	finalizeUACJob(record, err)
}

func finalizeUACJob(record *elevatedJobRecord, launchErr error) {
	response, responseErr := elevationprotocol.ReadResponse(record.responsePath)
	record.mu.Lock()
	defer record.mu.Unlock()
	if responseErr == nil {
		record.status.Report = &response.Report
		record.status.Error = response.Error
		if response.Report.Success {
			record.status.State = "completed"
		} else {
			record.status.State = "failed"
		}
		return
	}
	if errors.Is(launchErr, errUACCancelled) {
		record.status.State = "cancelled"
		record.status.Error = "Windows 权限确认被取消，电脑没有改动。可以返回后重试。"
		return
	}
	record.status.State = "failed"
	if launchErr != nil {
		record.status.Error = "Windows 管理员进程未完成：" + launchErr.Error()
		return
	}
	record.status.Error = "Windows 管理员进程没有返回安装结果：" + responseErr.Error()
}

func (a *App) runDirectJob(record *elevatedJobRecord, request DesktopRequest) {
	record.mu.Lock()
	record.status.State = "running"
	record.mu.Unlock()
	report, err := a.engine.Apply(a.ctx, request.Profile, desktopApplyOptions(request))
	record.mu.Lock()
	defer record.mu.Unlock()
	record.status.Report = &report
	record.status.Error = errorText(err)
	if report.Success {
		record.status.State = "completed"
	} else {
		record.status.State = "failed"
	}
}
