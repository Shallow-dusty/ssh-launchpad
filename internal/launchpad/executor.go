package launchpad

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
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
	"unicode/utf16"
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
	configureChildProcess(cmd)
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
	if len(plan.Blockers) > 0 {
		report.ExitCode = ExitVerificationFailed
		report.Error = "Apply is blocked: " + strings.Join(plan.Blockers, " ")
		report.Finished = time.Now().UTC()
		return report, errors.New(report.Error)
	}
	if plan.NoChanges {
		report.Success = true
		report.ExitCode = ExitOK
		report.Finished = time.Now().UTC()
		return report, nil
	}
	for _, action := range plan.Actions {
		if !action.Mutating || len(action.Command) == 0 {
			report.ExitCode = ExitUnsupported
			report.Error = "The plan contains a manual or unsupported action and cannot be reported as successfully applied: " + action.ID
			report.Finished = time.Now().UTC()
			return report, errors.New(report.Error)
		}
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
		return finishReportError(report, ExitPartialFailure, fmt.Errorf("create journal directory: %w", err))
	}
	journal := Journal{SchemaVersion: SchemaVersion, ID: report.ID, Created: started, ProfileName: profile.Name, Status: "running", Actions: plan.Actions}
	journalPath := filepath.Join(journalDir, report.ID+".journal.json")
	report.JournalPath = journalPath
	if err := writeJournalAtomic(journalPath, &journal); err != nil {
		return finishReportError(report, ExitPartialFailure, fmt.Errorf("persist journal before Apply: %w", err))
	}
	runner := e.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	var completed []Action
	for _, action := range plan.Actions {
		result := ActionResult{ActionID: action.ID, Status: "running", Started: time.Now().UTC()}
		e.emit(StageApply, action.ID, "started", action.Summary, &report)
		materializedCommand, err := materializeActionCommand(action, profile, plan.Platform)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.Finished = time.Now().UTC()
			report.Results = append(report.Results, result)
			return e.failAndRollback(ctx, profile, report, journal, journalPath, completed, opts, err)
		}
		command, cleanupArtifact, err := stageVerifiedActionArtifact(action, materializedCommand, journalDir, report.ID)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.Finished = time.Now().UTC()
			report.Results = append(report.Results, result)
			return e.failAndRollback(ctx, profile, report, journal, journalPath, completed, opts, err)
		}
		scheduled := action.SelfCutRisk && opts.ScheduleRisky
		if scheduled {
			command, err = scheduledCommand(command, profile.Safety.ScheduledDelaySecond, action.ID)
			if err != nil {
				cleanupArtifact()
				result.Status = "failed"
				result.Error = err.Error()
				result.Finished = time.Now().UTC()
				report.Results = append(report.Results, result)
				return e.failAndRollback(ctx, profile, report, journal, journalPath, completed, opts, err)
			}
		}
		var buffer safeBuffer
		err = runner.Run(ctx, command, &buffer)
		if !scheduled || err != nil {
			cleanupArtifact()
		}
		result.Output = buffer.String()
		if action.Operation == "authenticate_tailscale" {
			result.Output = redactCredentialText(result.Output, profile.Transport.AuthKey)
		}
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Status = "failed"
			executionError := err.Error()
			if action.Operation == "authenticate_tailscale" {
				executionError = redactCredentialText(executionError, profile.Transport.AuthKey)
			}
			result.Error = executionError
			report.Results = append(report.Results, result)
			e.emit(StageApply, action.ID, "error", executionError, &report)
			return e.failAndRollback(ctx, profile, report, journal, journalPath, completed, opts, errors.New(executionError))
		}
		result.Status = map[bool]string{true: "scheduled", false: "completed"}[action.SelfCutRisk && opts.ScheduleRisky]
		report.Results = append(report.Results, result)
		completed = append(completed, action)
		e.emit(StageApply, action.ID, "completed", result.Status, &report)
		journal.Results = report.Results
		if err := writeJournalAtomic(journalPath, &journal); err != nil {
			return e.failAndRollback(ctx, profile, report, journal, journalPath, completed, opts, fmt.Errorf("persist journal after %s: %w", action.ID, err))
		}
	}
	journal.Status = "completed"
	journal.Results = report.Results
	if err := writeJournalAtomic(journalPath, &journal); err != nil {
		return e.failAndRollback(ctx, profile, report, journal, journalPath, completed, opts, fmt.Errorf("finalize journal: %w", err))
	}
	report.Success = true
	report.ExitCode = ExitOK
	report.Finished = time.Now().UTC()
	return report, nil
}

// materializeActionCommand replaces the tailscale auth marker with the real
// command carrying the profile's auth key. This happens only inside Apply, so
// the secret stays out of the reviewable plan, the journal, and staged
// artifacts. The key briefly appears in the child process argv; that
// trade-off is recorded in docs/threat-model.md.
func materializeActionCommand(action Action, profile Profile, platform Platform) ([]string, error) {
	if len(action.Command) != 1 || action.Command[0] != tailscaleAuthCommandMarker {
		return action.Command, nil
	}
	key := strings.TrimSpace(profile.Transport.AuthKey)
	if key == "" {
		return nil, errors.New("tailscale authentication was planned without an auth key")
	}
	switch platform {
	case PlatformWindows:
		// A freshly installed tailscale.exe may not be on PATH yet, so poll
		// briefly before giving up; the install action immediately precedes
		// this one in the same Apply run.
		quotedKey := strings.ReplaceAll(key, "'", "''")
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $key='%s'; $exe=$null; for($i=0;$i -lt 30 -and !$exe;$i++){ $command=Get-Command tailscale.exe -ErrorAction SilentlyContinue; if($command){$exe=$command.Source}; if(!$exe){$candidate=Join-Path $env:ProgramFiles 'Tailscale\tailscale.exe'; if(Test-Path $candidate){$exe=$candidate}}; if(!$exe){Start-Sleep -Seconds 1} }; if(!$exe){throw 'tailscale.exe was not found after installation'}; & $exe up ("--auth-key="+$key); if($LASTEXITCODE -ne 0){throw 'Tailscale authentication failed'}`, quotedKey)
		return psCommand(script), nil
	case PlatformLinux, PlatformWSL, PlatformMacOS:
		return []string{"tailscale", "up", "--auth-key=" + key}, nil
	default:
		return nil, fmt.Errorf("tailscale authentication is unsupported on platform %q", platform)
	}
}

func (e Executor) Rollback(ctx context.Context, journalPath string) (Report, error) {
	started := time.Now().UTC()
	report := newReport(StageRollback, "", started)
	report.JournalPath = journalPath
	journal, warnings, err := readJournal(journalPath)
	if err != nil {
		return finishReportError(report, ExitInvalidProfile, fmt.Errorf("read rollback journal: %w", err))
	}
	report.Warnings = append(report.Warnings, warnings...)
	report.ProfileName = journal.ProfileName
	if journal.Status == "rolled-back" {
		report.Success = true
		report.ExitCode = ExitOK
		report.Finished = time.Now().UTC()
		return report, nil
	}
	runner := e.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	for i := len(journal.Actions) - 1; i >= 0; i-- {
		action := journal.Actions[i]
		if !action.Reversible || len(action.RollbackCommand) == 0 || !resultCompleted(journal.Results, action.ID) || resultHasStatus(journal.Results, action.ID, "rolled-back") {
			continue
		}
		var buffer safeBuffer
		result := ActionResult{ActionID: action.ID, Status: "rollback-running", Started: time.Now().UTC()}
		if resultHasStatus(journal.Results, action.ID, "scheduled") {
			if cancelCommand := cancelScheduledCommand(action.ID); len(cancelCommand) > 0 {
				if cancelErr := runner.Run(ctx, cancelCommand, &buffer); cancelErr != nil {
					result.Status = "rollback-failed"
					result.Error = "could not cancel scheduled action: " + cancelErr.Error()
					result.Finished = time.Now().UTC()
					report.Results = append(report.Results, result)
					journal.Status = "rollback-failed"
					journal.Results = append(journal.Results, result)
					_ = writeJournalAtomic(journalPath, &journal)
					return finishReportError(report, ExitPartialFailure, errors.New(result.Error))
				}
			}
		}
		err := runner.Run(ctx, action.RollbackCommand, &buffer)
		result.Output = buffer.String()
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Status = "rollback-failed"
			result.Error = err.Error()
			report.Results = append(report.Results, result)
			journal.Status = "rollback-failed"
			journal.Results = append(journal.Results, result)
			_ = writeJournalAtomic(journalPath, &journal)
			return finishReportError(report, ExitPartialFailure, err)
		}
		result.Status = "rolled-back"
		report.Results = append(report.Results, result)
		journal.Results = append(journal.Results, result)
		if err := writeJournalAtomic(journalPath, &journal); err != nil {
			return finishReportError(report, ExitPartialFailure, fmt.Errorf("persist rollback result: %w", err))
		}
	}
	journal.Status = "rolled-back"
	if err := writeJournalAtomic(journalPath, &journal); err != nil {
		return finishReportError(report, ExitPartialFailure, fmt.Errorf("Rollback completed but the journal could not be updated: %w", err))
	}
	report.Success = true
	report.ExitCode = ExitOK
	report.Finished = time.Now().UTC()
	return report, nil
}

func (e Executor) failAndRollback(ctx context.Context, profile Profile, report Report, journal Journal, journalPath string, completed []Action, opts ApplyOptions, cause error) (Report, error) {
	report.Success = false
	report.ExitCode = ExitPartialFailure
	var artifactErr *artifactVerificationError
	if errors.As(cause, &artifactErr) {
		report.ExitCode = ExitDownloadFailure
	}
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
			if action.SelfCutRisk && opts.ScheduleRisky {
				if cancelCommand := cancelScheduledCommand(action.ID); len(cancelCommand) > 0 {
					if cancelErr := runner.Run(ctx, cancelCommand, &output); cancelErr != nil {
						result.Status = "rollback-failed"
						result.Error = "could not cancel scheduled action: " + cancelErr.Error()
						result.Finished = time.Now().UTC()
						report.Results = append(report.Results, result)
						continue
					}
				}
			}
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
	if journalErr := writeJournalAtomic(journalPath, &journal); journalErr != nil {
		report.Warnings = append(report.Warnings, "The failure journal could not be updated: "+journalErr.Error())
	}
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
		encoded := base64.StdEncoding.EncodeToString(stringsToUTF16LE(payload))
		launcher := fmt.Sprintf(`$a=New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -NonInteractive -EncodedCommand %s'; $t=New-ScheduledTaskTrigger -Once -At (Get-Date).AddSeconds(%d); $s=New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Minutes 10); Register-ScheduledTask -TaskName 'SSH-Launchpad-%s' -Action $a -Trigger $t -Settings $s -RunLevel Highest -Force | Out-Null`, encoded, delay, taskID)
		return psCommand(launcher), nil
	}
	shell := shellJoin(command)
	payload := shQuote(shell)
	script := fmt.Sprintf("if command -v systemd-run >/dev/null 2>&1; then systemd-run --unit=%s --on-active=%ds --collect /bin/sh -c %s; else nohup /bin/sh -c %s >/tmp/%s.log 2>&1 </dev/null & fi", shQuote("ssh-launchpad-"+taskID), delay, payload, shQuote(fmt.Sprintf("sleep %d; exec %s", delay, shell)), shQuote("ssh-launchpad-"+taskID))
	return unixCommand(script), nil
}

func cancelScheduledCommand(actionID string) []string {
	taskID := sanitizeTaskID(actionID)
	if runtime.GOOS == "windows" {
		return psCommand(fmt.Sprintf(`Unregister-ScheduledTask -TaskName 'SSH-Launchpad-%s' -Confirm:$false -ErrorAction SilentlyContinue`, taskID))
	}
	unit := shQuote("ssh-launchpad-" + taskID)
	return unixCommand(fmt.Sprintf("if command -v systemctl >/dev/null 2>&1; then systemctl stop %s.timer %s.service 2>/dev/null || true; systemctl reset-failed %s.timer %s.service 2>/dev/null || true; fi", unit, unit, unit, unit))
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

func stringsToUTF16LE(value string) []byte {
	var out bytes.Buffer
	for _, codeUnit := range utf16.Encode([]rune(value)) {
		_ = binary.Write(&out, binary.LittleEndian, codeUnit)
	}
	return out.Bytes()
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

func resultHasStatus(results []ActionResult, id, status string) bool {
	for _, result := range results {
		if result.ActionID == id && result.Status == status {
			return true
		}
	}
	return false
}

type artifactVerificationError struct {
	cause error
}

func (e *artifactVerificationError) Error() string {
	return "offline artifact verification failed: " + e.cause.Error()
}

func (e *artifactVerificationError) Unwrap() error {
	return e.cause
}

func stageVerifiedActionArtifact(action Action, command []string, journalDir, reportID string) ([]string, func(), error) {
	source := strings.TrimSpace(action.Params["artifactPath"])
	if source == "" {
		return command, func() {}, nil
	}
	expected := strings.TrimSpace(action.Params["artifactSHA256"])
	decoded, err := hex.DecodeString(expected)
	if err != nil || len(decoded) != sha256.Size {
		return nil, func() {}, &artifactVerificationError{cause: errors.New("a valid SHA-256 is required")}
	}
	if len(command) == 0 || filepath.Clean(command[0]) != filepath.Clean(source) {
		return nil, func() {}, &artifactVerificationError{cause: errors.New("the planned executable does not match artifactPath")}
	}
	info, err := os.Lstat(source)
	if err != nil {
		return nil, func() {}, &artifactVerificationError{cause: err}
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, func() {}, &artifactVerificationError{cause: errors.New("the offline artifact must be a regular non-symlink file")}
	}
	input, err := os.Open(source)
	if err != nil {
		return nil, func() {}, &artifactVerificationError{cause: err}
	}
	defer input.Close()
	openedInfo, err := input.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() {
		if err == nil {
			err = errors.New("the opened offline artifact is not a regular file")
		}
		return nil, func() {}, &artifactVerificationError{cause: err}
	}
	stage, err := os.MkdirTemp(journalDir, ".verified-"+sanitizeTaskID(reportID)+"-")
	if err != nil {
		return nil, func() {}, &artifactVerificationError{cause: err}
	}
	cleanup := func() {
		_ = os.Remove(filepath.Join(stage, "payload"+filepath.Ext(source)))
		_ = os.Remove(stage)
	}
	destination := filepath.Join(stage, "payload"+filepath.Ext(source))
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		cleanup()
		return nil, func() {}, &artifactVerificationError{cause: err}
	}
	hash := sha256.New()
	const maxArtifactBytes = int64(2 * 1024 * 1024 * 1024)
	written, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(input, maxArtifactBytes+1))
	if copyErr == nil && written > maxArtifactBytes {
		copyErr = errors.New("the offline artifact exceeds the 2 GiB size limit")
	}
	if copyErr == nil {
		copyErr = output.Sync()
	}
	if closeErr := output.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		cleanup()
		return nil, func() {}, &artifactVerificationError{cause: copyErr}
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		cleanup()
		return nil, func() {}, &artifactVerificationError{cause: fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expected, actual)}
	}
	stagedCommand := append([]string(nil), command...)
	stagedCommand[0] = destination
	return stagedCommand, cleanup, nil
}

// readJournal loads a rollback journal. A digest mismatch is reported as a
// warning rather than a hard failure: the digest is self-computed, so it can
// only flag accidental corruption, and a recovery path must not refuse to
// recover over a checksum it could recompute itself.
func readJournal(path string) (Journal, []string, error) {
	file, err := openJournalRead(path)
	if err != nil {
		return Journal{}, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Journal{}, nil, err
	}
	const maxJournalBytes = 8 * 1024 * 1024
	if info.Size() > maxJournalBytes {
		return Journal{}, nil, errors.New("rollback journal exceeds the size limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxJournalBytes+1))
	if err != nil {
		return Journal{}, nil, err
	}
	if len(data) > maxJournalBytes {
		return Journal{}, nil, errors.New("rollback journal exceeds the size limit")
	}
	var journal Journal
	if err := json.Unmarshal(data, &journal); err != nil {
		return Journal{}, nil, err
	}
	if journal.SchemaVersion != SchemaVersion {
		return Journal{}, nil, fmt.Errorf("unsupported journal schema %d", journal.SchemaVersion)
	}
	if strings.TrimSpace(journal.ID) == "" || len(journal.Actions) > 256 {
		return Journal{}, nil, errors.New("rollback journal has invalid identity or action count")
	}
	var warnings []string
	if journal.Digest != "" && !strings.EqualFold(journal.Digest, journalDigest(journal)) {
		warnings = append(warnings, "The rollback journal digest does not match its contents; continuing with best-effort recovery from the recorded actions.")
	}
	return journal, warnings, nil
}

func journalDigest(journal Journal) string {
	journal.Digest = ""
	data, err := json.Marshal(journal)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func writeJournalAtomic(path string, journal *Journal) error {
	journal.Digest = journalDigest(*journal)
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".journal-*.tmp")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func finishReportError(report Report, code int, err error) (Report, error) {
	report.Success = false
	report.ExitCode = code
	report.Error = err.Error()
	report.Finished = time.Now().UTC()
	return report, err
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
