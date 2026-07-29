package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var errProcessLockHeld = errors.New("another SSH Launchpad wizard is running")

// acquireProcessLock prevents two interactive wizards from running at once.
// The design is deliberately simple: an exclusive-create file holding the
// owner's PID, with stale-lock recovery once that PID is gone. Unlock just
// deletes the file. The theoretical race (a replacement lock appearing
// between our staleness check and another owner's unlock) is accepted: the
// worst outcome is two wizards open side by side, and every state-changing
// action still requires explicit confirmation.
func acquireProcessLock() (func(), error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return func() {}, nil
	}
	return acquireProcessLockAt(filepath.Join(cache, "ssh-launchpad", "interactive.lock"))
}

func acquireProcessLockAt(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	pid := strconv.Itoa(os.Getpid()) + "\n"
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if _, writeErr := file.WriteString(pid); writeErr != nil {
				_ = file.Close()
				_ = os.Remove(path)
				return nil, writeErr
			}
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(path)
				return nil, closeErr
			}
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		stale, staleErr := staleProcessLock(path)
		if staleErr != nil {
			return nil, staleErr
		}
		if !stale {
			return nil, errProcessLockHeld
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove stale process lock: %w", err)
		}
	}
	return nil, errProcessLockHeld
}

// staleProcessLock reports whether the lock file belongs to a dead process.
// A file that does not contain a plain PID (for example after a crash
// mid-write) gets a short grace period so a just-created lock is not stolen
// from under its living writer.
func staleProcessLock(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if convErr != nil || pid <= 0 {
		return time.Since(info.ModTime()) >= 5*time.Minute, nil
	}
	return !processRunning(pid), nil
}
