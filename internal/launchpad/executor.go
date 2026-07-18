package launchpad

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type CommandRunner interface {
	Run(context.Context, []string, io.Writer) error
}

type OSCommandRunner struct{}

func (OSCommandRunner) Run(ctx context.Context, command []string, output io.Writer) error {
	if len(command) == 0 {
		return errors.New("no executable command is available for this action")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

type Executor struct {
	Runner             CommandRunner
	Sink               EventSink
	AdministratorCheck func(context.Context, Platform) bool
}

func (e Executor) Apply(ctx context.Context, profile Profile, plan Plan, opts ApplyOptions) (Report, error) {
	started := time.Now().UTC()
	report := newReport(StageApply, profile.Name, started)
	report.Plan = &plan
	if !opts.Confirmed {
		report.ExitCode = ExitConfirmationRequired
		report.Error = "Apply requires explicit confirmation."
		report.Finished = time.Now().UTC()
		return report, errors.New(report.Error)
	}
	if plan.NoChanges {
		report.Success = true
		report.ExitCode = ExitOK
		report.Finished = time.Now().UTC()
		return report, nil
	}
	if plan.SelfCutDetected && profile.Safety.PreventSelfCut && !opts.AllowSelfCut && !opts.ScheduleRisky {
		report.ExitCode = ExitSelfCutBlocked
		report.Error = "Self-cut risk detected: use a second control channel, --schedule-risky, or explicitly allow the risk."
		report.Finished = time.Now().UTC()
		return report, errors.New(report.Error)
	}
	if plan.SelfCutDetected && opts.ScheduleRisky {
		if err := preflightExternalVerify(opts.ExternalVerify); err != nil {
			report.ExitCode = ExitSelfCutBlocked
			report.Error = "Scheduled self-cut-sensitive work requires a reachable independent --external-verify-target: " + err.Error()
			report.Finished = time.Now().UTC()
			return report, errors.New(report.Error)
		}
		report.Warnings = append(report.Warnings, "An independent verification endpoint was reachable before scheduling: "+opts.ExternalVerify+". Re-check it from the controller after the delayed action.")
	}
	adminCheck := e.AdministratorCheck
	if adminCheck == nil {
		adminCheck = detectAdministrator
	}
	if containsElevated(plan.Actions) && !adminCheck(ctx, plan.Platform) {
		report.ExitCode = ExitNeedsElevation
		report.Error = "The plan contains elevated actions. Re-run Apply from an administrator/root session."
		report.Finished = time.Now().UTC()
		return report, errors.New(report.Error)
	}
	journalDir := opts.JournalDir
	if journalDir == "" {
		journalDir = stateDir(profile)
	}
	if err := os.MkdirAll(journalDir, 0o700); err != nil {
		return report, err
	}
	journal := Journal{SchemaVersion: SchemaVersion, ID: report.ID, Created: started, ProfileName: profile.Name, Status: "running", Actions: plan.Actions}
	journalPath := filepath.Join(journalDir, report.ID+".journal.json")
	report.JournalPath = journalPath
	if err := writeJSONAtomic(journalPath, journal); err != nil {
		return report, err
	}
	runner := e.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	var completed []Action
	for _, action := range plan.Actions {
		result := ActionResult{ActionID: action.ID, Status: "running", Started: time.Now().UTC()}
		e.emit(StageApply, action.ID, "started", action.Summary, &report)
		if !action.Mutating || len(action.Command) == 0 {
			result.Status = "manual"
			result.Output = action.Reason
			result.Finished = time.Now().UTC()
			report.Results = append(report.Results, result)
			journal.Results = report.Results
			_ = writeJSONAtomic(journalPath, journal)
			continue
		}
		command := action.Command
		if action.SelfCutRisk && opts.ScheduleRisky {
			var err error
			command, err = scheduledCommand(command, profile.Safety.ScheduledDelaySecond, action.ID)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				result.Finished = time.Now().UTC()
				report.Results = append(report.Results, result)
				return e.failAndRollback(ctx, profile, report, journal, journalPath, completed, opts, err)
			}
		}
		var buffer safeBuffer
		err := runner.Run(ctx, command, &buffer)
		result.Output = buffer.String()
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			report.Results = append(report.Results, result)
			e.emit(StageApply, action.ID, "error", err.Error(), &report)
			return e.failAndRollback(ctx, profile, report, journal, journalPath, completed, opts, err)
		}
		result.Status = map[bool]string{true: "scheduled", false: "completed"}[action.SelfCutRisk && opts.ScheduleRisky]
		report.Results = append(report.Results, result)
		completed = append(completed, action)
		e.emit(StageApply, action.ID, "completed", result.Status, &report)
		journal.Results = report.Results
		_ = writeJSONAtomic(journalPath, journal)
	}
	report.Success = true
	report.ExitCode = ExitOK
	report.Finished = time.Now().UTC()
	journal.Status = "completed"
	journal.Results = report.Results
	_ = writeJSONAtomic(journalPath, journal)
	return report, nil
}

func (e Executor) Rollback(ctx context.Context, journalPath string) (Report, error) {
	started := time.Now().UTC()
	report := newReport(StageRollback, "", started)
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return report, err
	}
	var journal Journal
	if err := json.Unmarshal(data, &journal); err != nil {
		return report, err
	}
	report.ProfileName = journal.ProfileName
	runner := e.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	for i := len(journal.Actions) - 1; i >= 0; i-- {
		action := journal.Actions[i]
		if !action.Reversible || len(action.RollbackCommand) == 0 || !resultCompleted(journal.Results, action.ID) {
			continue
		}
		var buffer safeBuffer
		result := ActionResult{ActionID: action.ID, Status: "rollback-running", Started: time.Now().UTC()}
		err := runner.Run(ctx, action.RollbackCommand, &buffer)
		result.Output = buffer.String()
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Status = "rollback-failed"
			result.Error = err.Error()
			report.Results = append(report.Results, result)
			report.ExitCode = ExitPartialFailure
			report.Error = err.Error()
			report.Finished = time.Now().UTC()
			return report, err
		}
		result.Status = "rolled-back"
		report.Results = append(report.Results, result)
	}
	journal.Status = "rolled-back"
	_ = writeJSONAtomic(journalPath, journal)
	report.Success = true
	report.ExitCode = ExitOK
	report.Finished = time.Now().UTC()
	return report, nil
}

func (e Executor) failAndRollback(ctx context.Context, profile Profile, report Report, journal Journal, journalPath string, completed []Action, opts ApplyOptions, cause error) (Report, error) {
	report.Success = false
	report.ExitCode = ExitPartialFailure
	report.Error = cause.Error()
	if opts.AutoRollback || profile.Safety.AutoRollback {
		runner := e.Runner
		if runner == nil {
			runner = OSCommandRunner{}
		}
		for i := len(completed) - 1; i >= 0; i-- {
			action := completed[i]
			if !action.Reversible || len(action.RollbackCommand) == 0 {
				continue
			}
			var output safeBuffer
			result := ActionResult{ActionID: action.ID, Status: "rollback-running", Started: time.Now().UTC()}
			err := runner.Run(ctx, action.RollbackCommand, &output)
			result.Output = output.String()
			result.Finished = time.Now().UTC()
			if err != nil {
				result.Status = "rollback-failed"
				result.Error = err.Error()
			} else {
				result.Status = "rolled-back"
			}
			report.Results = append(report.Results, result)
		}
	}
	report.Finished = time.Now().UTC()
	journal.Status = "failed"
	journal.Results = report.Results
	_ = writeJSONAtomic(journalPath, journal)
	return report, cause
}

func (e Executor) emit(stage Stage, actionID, kind, message string, report *Report) {
	event := Event{Timestamp: time.Now().UTC(), Stage: stage, ActionID: actionID, Kind: kind, Message: message}
	report.Events = append(report.Events, event)
	if e.Sink != nil {
		e.Sink(event)
	}
}

func scheduledCommand(command []string, delay int, actionID string) ([]string, error) {
	if len(command) == 0 {
		return nil, errors.New("cannot schedule an empty command")
	}
	taskID := sanitizeTaskID(actionID)
	if runtime.GOOS == "windows" {
		quoted := windowsCommandLine(command)
		payload := fmt.Sprintf("try { & %s } finally { Unregister-ScheduledTask -TaskName 'SSH-Launchpad-%s' -Confirm:$false -ErrorAction SilentlyContinue }", quoted, taskID)
		encoded := base64.StdEncoding.EncodeToString([]byte(stringsToUTF16LE(payload)))
		launcher := fmt.Sprintf(`$a=New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -NonInteractive -EncodedCommand %s'; $t=New-ScheduledTaskTrigger -Once -At (Get-Date).AddSeconds(%d); $s=New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Minutes 10); Register-ScheduledTask -TaskName 'SSH-Launchpad-%s' -Action $a -Trigger $t -Settings $s -RunLevel Highest -Force | Out-Null`, encoded, delay, taskID)
		return psCommand(launcher), nil
	}
	shell := shellJoin(command)
	payload := shQuote(shell)
	script := fmt.Sprintf("if command -v systemd-run >/dev/null 2>&1; then systemd-run --unit=%s --on-active=%ds --collect /bin/sh -c %s; else nohup /bin/sh -c %s >/tmp/%s.log 2>&1 </dev/null & fi", shQuote("ssh-launchpad-"+taskID), delay, payload, shQuote(fmt.Sprintf("sleep %d; exec %s", delay, shell)), shQuote("ssh-launchpad-"+taskID))
	return unixCommand(script), nil
}

func preflightExternalVerify(target string) error {
	if target == "" {
		return errors.New("target is missing")
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		return fmt.Errorf("target must be host:port: %w", err)
	}
	connection, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		return fmt.Errorf("%s is not reachable before scheduling: %w", target, err)
	}
	return connection.Close()
}

func sanitizeTaskID(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "action"
	}
	return builder.String()
}

func windowsCommandLine(command []string) string {
	parts := make([]string, len(command))
	for i, part := range command {
		parts[i] = "'" + strings.ReplaceAll(part, "'", "''") + "'"
	}
	return strings.Join(parts, " ")
}

func shellJoin(command []string) string {
	parts := make([]string, len(command))
	for i, part := range command {
		parts[i] = shQuote(part)
	}
	return strings.Join(parts, " ")
}

func stringsToUTF16LE(value string) string {
	var out bytes.Buffer
	for _, r := range value {
		out.WriteByte(byte(r))
		out.WriteByte(byte(r >> 8))
	}
	return out.String()
}

func containsElevated(actions []Action) bool {
	for _, action := range actions {
		if action.Mutating && action.RequiresElevation && len(action.Command) > 0 {
			return true
		}
	}
	return false
}

func resultCompleted(results []ActionResult, id string) bool {
	for _, result := range results {
		if result.ActionID == id && (result.Status == "completed" || result.Status == "scheduled") {
			return true
		}
	}
	return false
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func newReport(stage Stage, profile string, started time.Time) Report {
	return Report{SchemaVersion: SchemaVersion, Version: Version, ID: fmt.Sprintf("%s-%d", stage, started.UnixNano()), Stage: stage, Started: started, ProfileName: profile}
}

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	const max = 64 * 1024
	value := b.b.String()
	if len(value) > max {
		return value[:max] + "\n[output truncated]"
	}
	return value
}
