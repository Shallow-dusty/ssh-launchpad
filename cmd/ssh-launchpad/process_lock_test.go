package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
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
	deadPID := int(^uint32(0) >> 1)
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n", deadPID)), 0o600); err != nil {
		t.Fatal(err)
	}
	release, err := acquireProcessLockAt(path)
	if err != nil {
		t.Fatalf("dead owner was not recovered: %v", err)
	}
	release()
}

func TestProcessLockRejectsUnreadableLockPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interactive.lock")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireProcessLockAt(path); err == nil || errors.Is(err, errProcessLockHeld) {
		t.Fatalf("directory at lock path must surface an error, got: %v", err)
	}
}
