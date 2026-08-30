package launchpad

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type recordingRunner struct {
	commands [][]string
	failAt   int
	failure  error
}

type artifactRunner struct {
	source  string
	content []byte
	called  bool
}

func (r *artifactRunner) Run(_ context.Context, command []string, _ io.Writer) error {
	r.called = true
	if len(command) == 0 || filepath.Clean(command[0]) == filepath.Clean(r.source) {
		return errors.New("offline artifact was not executed from verified staging")
	}
	data, err := os.ReadFile(command[0])
	if err != nil {
		return err
	}
	if !bytes.Equal(data, r.content) {
		return errors.New("staged artifact content changed")
	}
	return nil
}

func (r *recordingRunner) Run(_ context.Context, command []string, output io.Writer) error {
	r.commands = append(r.commands, append([]string(nil), command...))
	_, _ = io.WriteString(output, "ran "+strings.Join(command, " "))
	if r.failAt > 0 && len(r.commands) == r.failAt {
		if r.failure != nil {
			return r.failure
		}
		return errors.New("injected failure")
	}
	return nil
}

func TestPartialFailureRollsBackCompletedActions(t *testing.T) {
	runner := &recordingRunner{failAt: 2}
	executor := Executor{Runner: runner}
	plan := Plan{
		Platform: PlatformLinux,
		Actions: []Action{
			{ID: "one", Mutating: true, Reversible: true, Command: []string{"do-one"}, RollbackCommand: []string{"undo-one"}},
			{ID: "two", Mutating: true, Reversible: true, Command: []string{"do-two"}, RollbackCommand: []string{"undo-two"}},
		},
	}
	profile := DefaultProfile()
	report, err := executor.Apply(context.Background(), profile, plan, ApplyOptions{Confirmed: true, AutoRollback: true, JournalDir: t.TempDir()})
	if err == nil || report.ExitCode != ExitPartialFailure {
		t.Fatalf("expected partial failure, got err=%v report=%+v", err, report)
	}
	if got := strings.Join(runner.commands[len(runner.commands)-1], " "); got != "undo-one" {
		t.Fatalf("expected rollback, got %s", got)
	}
}

func TestApplyRequiresConfirmation(t *testing.T) {
	executor := Executor{Runner: &recordingRunner{}}
	report, err := executor.Apply(context.Background(), DefaultProfile(), Plan{Actions: []Action{{ID: "x", Mutating: true, Command: []string{"x"}}}}, ApplyOptions{})
	if err == nil || report.ExitCode != ExitConfirmationRequired {
		t.Fatalf("unexpected result: %+v %v", report, err)
	}
}

func TestSelfCutIsBlockedByDefault(t *testing.T) {
	executor := Executor{Runner: &recordingRunner{}}
	plan := Plan{Platform: PlatformLinux, SelfCutDetected: true, Actions: []Action{{ID: "x", Mutating: true, SelfCutRisk: true, Command: []string{"x"}}}}
	report, err := executor.Apply(context.Background(), DefaultProfile(), plan, ApplyOptions{Confirmed: true, JournalDir: t.TempDir()})
	if err == nil || report.ExitCode != ExitSelfCutBlocked {
		t.Fatalf("unexpected result: %+v %v", report, err)
	}
}

func TestScheduledFallbackCanBeCancelled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fallback is Unix-only")
	}
	command, err := scheduledCommand([]string{"restart-ssh"}, 5, "configure_sshd")
	if err != nil {
		t.Fatal(err)
	}
	generated := strings.Join(command, " ")
	if output, syntaxErr := exec.Command("sh", "-n", "-c", command[2]).CombinedOutput(); syntaxErr != nil {
		t.Fatalf("scheduled fallback has invalid shell syntax: %v\n%s", syntaxErr, output)
	}
	cancelCommand := cancelScheduledCommand("configure_sshd")
	if output, syntaxErr := exec.Command("sh", "-n", "-c", cancelCommand[2]).CombinedOutput(); syntaxErr != nil {
		t.Fatalf("scheduled cancellation has invalid shell syntax: %v\n%s", syntaxErr, output)
	}
	for _, required := range []string{"systemd-run", ".pid", "set -C", "kill", "pidfile"} {
		if !strings.Contains(generated, required) {
			t.Fatalf("scheduled command missing cancellable fallback %q: %s", required, generated)
		}
	}
	cancel := strings.Join(cancelCommand, " ")
	if !strings.Contains(cancel, "kill") || !strings.Contains(cancel, ".pid") {
		t.Fatalf("scheduled cancellation does not cover fallback process: %s", cancel)
	}
}

func TestScheduledSelfCutRequiresReachableExternalVerify(t *testing.T) {
	executor := Executor{Runner: &recordingRunner{}}
	plan := Plan{Platform: detectPlatform(), SelfCutDetected: true, Actions: []Action{{ID: "restart-transport", Mutating: true, SelfCutRisk: true, Command: []string{"safe-test-command"}}}}
	options := ApplyOptions{Confirmed: true, ScheduleRisky: true, JournalDir: t.TempDir()}
	report, err := executor.Apply(context.Background(), DefaultProfile(), plan, options)
	if err == nil || report.ExitCode != ExitSelfCutBlocked {
		t.Fatalf("missing external target should be blocked: %+v %v", report, err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	options.ExternalVerify = listener.Addr().String()
	report, err = executor.Apply(context.Background(), DefaultProfile(), plan, options)
	if err != nil || !report.Success {
		t.Fatalf("reachable independent target should permit scheduling: %+v %v", report, err)
	}
}

func TestApplyRejectsPlanBlockersAndManualActions(t *testing.T) {
	executor := Executor{Runner: &recordingRunner{}}
	report, err := executor.Apply(context.Background(), DefaultProfile(), Plan{Blockers: []string{"sign in first"}}, ApplyOptions{Confirmed: true})
	if err == nil || report.Success || report.ExitCode != ExitVerificationFailed {
		t.Fatalf("blocker was reported as success: %+v %v", report, err)
	}
	report, err = executor.Apply(context.Background(), DefaultProfile(), Plan{Actions: []Action{{ID: "manual", Mutating: false}}}, ApplyOptions{Confirmed: true})
	if err == nil || report.Success || report.ExitCode != ExitUnsupported {
		t.Fatalf("manual action was reported as success: %+v %v", report, err)
	}
}

func TestJournalSetupAndReadFailuresNeverReturnSuccess(t *testing.T) {
	executor := Executor{Runner: &recordingRunner{}}
	journalFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(journalFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Platform: PlatformLinux, Actions: []Action{{ID: "x", Mutating: true, Command: []string{"x"}}}}
	report, err := executor.Apply(context.Background(), DefaultProfile(), plan, ApplyOptions{Confirmed: true, JournalDir: journalFile})
	if err == nil || report.ExitCode == ExitOK || report.Finished.IsZero() {
		t.Fatalf("journal setup failure returned success: %+v %v", report, err)
	}

	report, err = executor.Rollback(context.Background(), filepath.Join(t.TempDir(), "missing.json"))
	if err == nil || report.ExitCode == ExitOK || report.Finished.IsZero() {
		t.Fatalf("missing rollback journal returned success: %+v %v", report, err)
	}
}

// A digest-mismatched journal no longer blocks rollback: the digest is
// self-computed and only flags accidental corruption, so the recovery path
// warns and continues instead of refusing to recover.
func TestRollbackWarnsOnDigestMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.json")
	journal := Journal{SchemaVersion: SchemaVersion, ID: "tamper-test", Status: "completed"}
	if err := writeJournalAtomic(path, &journal); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`"status": "completed"`), []byte(`"status": "running"`), 1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := (Executor{}).Rollback(context.Background(), path)
	if err != nil {
		t.Fatalf("digest-mismatched journal was rejected: %v", err)
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected a digest-mismatch warning, got %+v", report)
	}
}

func TestRollbackIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.json")
	journal := Journal{
		SchemaVersion: SchemaVersion,
		ID:            "rollback-test",
		ProfileName:   "test",
		Status:        "completed",
		Actions: []Action{{
			ID:              "configure-sshd",
			Operation:       "configure_sshd",
			Reversible:      true,
			RollbackCommand: []string{"undo"},
		}},
		Results: []ActionResult{{ActionID: "configure-sshd", Status: "completed"}},
	}
	if err := writeJournalAtomic(path, &journal); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	executor := Executor{Runner: runner}
	for attempt := 0; attempt < 2; attempt++ {
		report, err := executor.Rollback(context.Background(), path)
		if err != nil || !report.Success {
			t.Fatalf("rollback attempt %d failed: %+v %v", attempt+1, report, err)
		}
	}
	if len(runner.commands) != 1 {
		t.Fatalf("rollback command ran %d times, want once", len(runner.commands))
	}
}

func TestOfflineArtifactIsHashedAndStagedBeforeExecution(t *testing.T) {
	content := []byte("verified offline installer")
	source := filepath.Join(t.TempDir(), "installer.exe")
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	action := Action{
		ID:        "install-tailscale",
		Operation: "install_tailscale",
		Mutating:  true,
		Command:   []string{source, "/quiet"},
		Params: map[string]string{
			"artifactPath":   source,
			"artifactSHA256": hex.EncodeToString(digest[:]),
		},
	}
	runner := &artifactRunner{source: source, content: content}
	executor := Executor{Runner: runner}
	report, err := executor.Apply(context.Background(), DefaultProfile(), Plan{Platform: PlatformLinux, Actions: []Action{action}}, ApplyOptions{Confirmed: true, JournalDir: t.TempDir()})
	if err != nil || !report.Success || !runner.called {
		t.Fatalf("verified offline artifact was not executed safely: %+v %v", report, err)
	}

	action.Params["artifactSHA256"] = strings.Repeat("0", 64)
	runner.called = false
	report, err = executor.Apply(context.Background(), DefaultProfile(), Plan{Platform: PlatformLinux, Actions: []Action{action}}, ApplyOptions{Confirmed: true, JournalDir: t.TempDir()})
	if err == nil || report.ExitCode != ExitDownloadFailure || runner.called {
		t.Fatalf("bad offline artifact was not rejected before execution: %+v %v", report, err)
	}
}

func TestUTF16LEPreservesSurrogatePairs(t *testing.T) {
	got := stringsToUTF16LE("A😀")
	want := []byte{0x41, 0x00, 0x3d, 0xd8, 0x00, 0xde}
	if !bytes.Equal(got, want) {
		t.Fatalf("UTF-16LE mismatch: %x", got)
	}
}

func TestTailscaleAuthKeyIsMaterializedOnlyForExecutionAndRedactedFromReport(t *testing.T) {
	runner := &recordingRunner{}
	executor := Executor{
		Runner:             runner,
		AdministratorCheck: func(context.Context, Platform) bool { return true },
	}
	profile := DefaultProfile()
	profile.Transport.AuthKey = "tskey-" + "auth-example-once"
	plan := Plan{
		Platform: PlatformLinux,
		Actions: []Action{{
			ID:                "authenticate-tailscale",
			Operation:         "authenticate_tailscale",
			Mutating:          true,
			RequiresElevation: true,
			Command:           []string{tailscaleAuthCommandMarker},
		}},
	}
	report, err := executor.Apply(context.Background(), profile, plan, ApplyOptions{Confirmed: true, JournalDir: t.TempDir()})
	if err != nil || !report.Success {
		t.Fatalf("Tailscale authentication action failed: %+v %v", report, err)
	}
	if got := strings.Join(runner.commands[0], " "); !strings.Contains(got, profile.Transport.AuthKey) {
		t.Fatalf("auth key was not materialized for execution: %s", got)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), profile.Transport.AuthKey) {
		t.Fatalf("auth key leaked into report: %s", encoded)
	}
	if report.Plan.Actions[0].Command[0] != tailscaleAuthCommandMarker {
		t.Fatalf("inspectable plan should retain only the marker: %#v", report.Plan.Actions[0].Command)
	}
}

func TestTailscaleAuthKeyIsRedactedFromFailureAndJournal(t *testing.T) {
	profile := DefaultProfile()
	profile.Transport.AuthKey = "tskey-" + "auth-example--wrapped/credential+tail=="
	runner := &recordingRunner{
		failAt:  1,
		failure: errors.New("tailscale rejected " + profile.Transport.AuthKey),
	}
	executor := Executor{
		Runner:             runner,
		AdministratorCheck: func(context.Context, Platform) bool { return true },
	}
	plan := Plan{
		Platform: PlatformLinux,
		Actions: []Action{{
			ID:                "authenticate-tailscale",
			Operation:         "authenticate_tailscale",
			Mutating:          true,
			RequiresElevation: true,
			Command:           []string{tailscaleAuthCommandMarker},
		}},
	}
	report, err := executor.Apply(context.Background(), profile, plan, ApplyOptions{Confirmed: true, JournalDir: t.TempDir()})
	if err == nil || report.Success {
		t.Fatalf("expected authentication failure: %+v %v", report, err)
	}
	for _, value := range []string{report.Error, err.Error(), report.Results[0].Error, report.Results[0].Output} {
		if strings.Contains(value, profile.Transport.AuthKey) {
			t.Fatalf("auth key leaked through failed Apply: %q", value)
		}
	}
	journal, readErr := os.ReadFile(report.JournalPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(journal), profile.Transport.AuthKey) {
		t.Fatalf("auth key leaked into journal: %s", journal)
	}
}
