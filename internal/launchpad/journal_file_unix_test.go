//go:build !windows

package launchpad

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRollbackRejectsSymlinkJournal(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.json")
	if err := os.WriteFile(target, []byte(`{"schemaVersion":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "journal.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	report, err := (Executor{}).Rollback(context.Background(), link)
	if err == nil || report.ExitCode == ExitOK {
		t.Fatalf("symlink journal was accepted: %+v %v", report, err)
	}
}
