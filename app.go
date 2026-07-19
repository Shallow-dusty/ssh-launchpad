package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
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
	Confirmed      bool              `json:"confirmed"`
	AllowSelfCut   bool              `json:"allowSelfCut"`
	ScheduleRisky  bool              `json:"scheduleRisky"`
	ExternalVerify string            `json:"externalVerify"`
}

type PublicKeyInfo struct {
	Label          string `json:"label"`
	Path           string `json:"path"`
	PublicKey      string `json:"publicKey"`
	PrivateKeyPath string `json:"privateKeyPath,omitempty"`
	Generated      bool   `json:"generated"`
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
	return launchpad.DefaultProfile()
}

func (a *App) CheckForUpdate() (launchpad.UpdateInfo, error) {
	return launchpad.CheckForUpdate(a.ctx)
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

func (a *App) BeginElevatedApply(request DesktopRequest) (ElevatedJob, error) {
	if request.Stage != launchpad.StageApply || !request.Confirmed {
		return ElevatedJob{}, errors.New("safe install requires an explicitly confirmed Apply request")
	}
	if runtime.GOOS == "windows" && strings.TrimSpace(request.Profile.Advanced.StateDir) == "" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		request.Profile.Advanced.StateDir = filepath.Join(programData, "SSH Launchpad")
	}
	if err := request.Profile.Validate(); err != nil {
		return ElevatedJob{}, err
	}
	planReport, err := a.engine.Plan(a.ctx, request.Profile)
	if err != nil {
		return ElevatedJob{}, err
	}
	if planReport.Plan == nil || planReport.Plan.NoChanges {
		report, applyErr := a.engine.Apply(a.ctx, request.Profile, launchpad.ApplyOptions{
			Confirmed:      true,
			AllowSelfCut:   request.AllowSelfCut,
			ScheduleRisky:  request.ScheduleRisky,
			AutoRollback:   request.Profile.Safety.AutoRollback,
			ExternalVerify: request.ExternalVerify,
		})
		return ElevatedJob{ID: report.ID, State: "completed", Report: &report, Error: errorText(applyErr)}, nil
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

	if runtime.GOOS == "windows" && planReport.Snapshot != nil && !planReport.Snapshot.IsAdministrator && planNeedsElevation(planReport.Plan) {
		data, err := json.MarshalIndent(request, "", "  ")
		if err != nil {
			return ElevatedJob{}, err
		}
		requestPath := filepath.Join(directory, "request.json")
		if err := os.WriteFile(requestPath, append(data, '\n'), 0o600); err != nil {
			return ElevatedJob{}, err
		}
		executable, err := os.Executable()
		if err != nil {
			return ElevatedJob{}, err
		}
		digest := requestDigest(append(data, '\n'))
		go a.runUACJob(record, executable, requestPath, digest)
		return record.status, nil
	}

	go a.runDirectJob(record, request)
	return record.status, nil
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
	err := launchElevatedHelper(context.Background(), executable, requestPath, record.responsePath, record.eventsPath, digest)
	response, responseErr := readHelperResponse(record.responsePath)
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
	record.status.State = "cancelled"
	record.status.Error = "Windows 权限确认被取消，电脑没有改动。可以返回后重试。"
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "exit status") {
		record.status.Error = "无法启动 Windows 权限确认：" + err.Error()
	}
}

func (a *App) runDirectJob(record *elevatedJobRecord, request DesktopRequest) {
	record.mu.Lock()
	record.status.State = "running"
	record.mu.Unlock()
	report, err := a.engine.Apply(a.ctx, request.Profile, launchpad.ApplyOptions{
		Confirmed:      true,
		AllowSelfCut:   request.AllowSelfCut,
		ScheduleRisky:  request.ScheduleRisky,
		AutoRollback:   request.Profile.Safety.AutoRollback,
		ExternalVerify: request.ExternalVerify,
	})
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
	data, err := json.MarshalIndent(launchpad.RedactReport(report), "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(data, '\n'), 0o600)
}

func (a *App) ImportProfile() (launchpad.Profile, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入 SSH Launchpad 配置",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "YAML / JSON profile", Pattern: "*.yaml;*.yml;*.json"},
		},
	})
	if err != nil || path == "" {
		return launchpad.Profile{}, err
	}
	return launchpad.LoadProfile(path)
}

func (a *App) ExportProfile(profile launchpad.Profile) (string, error) {
	if err := profile.Validate(); err != nil {
		return "", err
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出 SSH Launchpad 配置",
		DefaultFilename: profile.Name + ".ssh-launchpad.yaml",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "YAML profile", Pattern: "*.yaml"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	data, err := yaml.Marshal(profile)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o600)
}

func (a *App) DiscoverPublicKeys() ([]PublicKeyInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	keys := make([]PublicKeyInfo, 0, len(paths))
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil || len(data) > 32*1024 {
			continue
		}
		value := strings.TrimSpace(string(data))
		if launchpad.ValidatePublicKey(value) != nil {
			continue
		}
		keys = append(keys, PublicKeyInfo{
			Label:          filepath.Base(path),
			Path:           path,
			PublicKey:      value,
			PrivateKeyPath: strings.TrimSuffix(path, ".pub"),
		})
	}
	return keys, nil
}

func (a *App) GenerateControllerKey(label string) (PublicKeyInfo, error) {
	sshKeygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return PublicKeyInfo{}, errors.New("未找到 ssh-keygen；请先安装 Windows OpenSSH Client")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return PublicKeyInfo{}, err
	}
	directory := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return PublicKeyInfo{}, err
	}
	privatePath := filepath.Join(directory, "id_ed25519_ssh_launchpad")
	publicPath := privatePath + ".pub"
	if _, err := os.Stat(publicPath); errors.Is(err, os.ErrNotExist) {
		if _, privateErr := os.Stat(privatePath); privateErr == nil {
			return PublicKeyInfo{}, errors.New("检测到已有私钥但缺少对应公钥；为避免覆盖，已停止生成。请用 ssh-keygen -y 恢复公钥或选择其他密钥")
		}
		comment := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(label, "\r", " "), "\n", " "))
		if comment == "" {
			comment = "ssh-launchpad-controller"
		}
		command := exec.Command(sshKeygen, "-t", "ed25519", "-f", privatePath, "-N", "", "-C", comment)
		if output, runErr := command.CombinedOutput(); runErr != nil {
			return PublicKeyInfo{}, fmt.Errorf("生成控制电脑密钥失败: %v: %s", runErr, strings.TrimSpace(string(output)))
		}
		_ = os.Chmod(privatePath, 0o600)
		_ = os.Chmod(publicPath, 0o644)
	}
	data, err := os.ReadFile(publicPath)
	if err != nil {
		return PublicKeyInfo{}, err
	}
	value := strings.TrimSpace(string(data))
	if err := launchpad.ValidatePublicKey(value); err != nil {
		return PublicKeyInfo{}, err
	}
	return PublicKeyInfo{
		Label:          filepath.Base(publicPath),
		Path:           publicPath,
		PublicKey:      value,
		PrivateKeyPath: privatePath,
		Generated:      true,
	}, nil
}

func (a *App) ImportPublicKey() (PublicKeyInfo, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择控制电脑的公钥",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "OpenSSH public key", Pattern: "*.pub;*.txt"},
		},
	})
	if err != nil || path == "" {
		return PublicKeyInfo{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PublicKeyInfo{}, err
	}
	if len(data) > 32*1024 || strings.Contains(string(data), "PRIVATE KEY") {
		return PublicKeyInfo{}, errors.New("文件不是安全的公钥文件；私钥不会被导入")
	}
	for _, line := range strings.Split(string(data), "\n") {
		value := strings.TrimSpace(line)
		if launchpad.ValidatePublicKey(value) == nil {
			return PublicKeyInfo{Label: filepath.Base(path), Path: path, PublicKey: value}, nil
		}
	}
	return PublicKeyInfo{}, errors.New("文件中没有找到支持的 OpenSSH 公钥")
}

func (a *App) ExportPairingFile(publicKey string) (string, error) {
	publicKey = strings.TrimSpace(publicKey)
	if err := launchpad.ValidatePublicKey(publicKey); err != nil {
		return "", err
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出配对公钥",
		DefaultFilename: "ssh-launchpad-controller.pub",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "OpenSSH public key", Pattern: "*.pub"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(publicKey+"\n"), 0o644)
}

func planNeedsElevation(plan *launchpad.Plan) bool {
	for _, action := range plan.Actions {
		if action.Mutating && action.RequiresElevation {
			return true
		}
	}
	return false
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
