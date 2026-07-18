package launchpad

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
)

type recordingRunner struct {
	commands [][]string
	failAt   int
}

func (r *recordingRunner) Run(_ context.Context, command []string, output io.Writer) error {
	r.commands = append(r.commands, append([]string(nil), command...))
	_, _ = io.WriteString(output, "ran "+strings.Join(command, " "))
	if r.failAt > 0 && len(r.commands) == r.failAt {
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
