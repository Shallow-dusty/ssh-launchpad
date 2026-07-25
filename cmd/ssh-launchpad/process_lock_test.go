package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessLockBlocksLiveOwnerAndCanBeReacquired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interactive.lock")
	release, err := acquireProcessLockAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireProcessLockAt(path); !errors.Is(err, errProcessLockHeld) {
		t.Fatalf("live owner was not protected: %v", err)
	}
	release()
	reacquired, err := acquireProcessLockAt(path)
	if err != nil {
		t.Fatalf("released lock could not be reacquired: %v", err)
	}
	reacquired()
}

func TestProcessLockRecoversDeadOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interactive.lock")
	record, err := json.Marshal(processLockRecord{
		PID:     int(^uint32(0) >> 1),
		Token:   "stale",
		Created: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(record, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	release, err := acquireProcessLockAt(path)
	if err != nil {
		t.Fatalf("dead owner was not recovered: %v", err)
	}
	release()
}

func TestProcessLockCleanupDoesNotDeleteReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interactive.lock")
	release, err := acquireProcessLockAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	replacement := []byte("{\"pid\":1,\"token\":\"replacement\"}\n")
	if err := os.WriteFile(path, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	release()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("replacement lock was deleted: %v", err)
	}
	if string(got) != string(replacement) {
		t.Fatalf("replacement lock was changed: %q", got)
	}
}
