package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var errProcessLockHeld = errors.New("another SSH Launchpad wizard is running")

type processLockRecord struct {
	PID     int    `json:"pid"`
	Token   string `json:"token"`
	Created string `json:"created"`
}

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
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	record := processLockRecord{
		PID:     os.Getpid(),
		Token:   hex.EncodeToString(tokenBytes),
		Created: time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	for attempt := 0; attempt < 3; attempt++ {
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if createErr == nil {
			if _, writeErr := file.Write(data); writeErr != nil {
				_ = file.Close()
				_ = removeLockIfUnchanged(path, data)
				return nil, writeErr
			}
			if syncErr := file.Sync(); syncErr != nil {
				_ = file.Close()
				_ = removeLockIfUnchanged(path, data)
				return nil, syncErr
			}
			if closeErr := file.Close(); closeErr != nil {
				_ = removeLockIfUnchanged(path, data)
				return nil, closeErr
			}
			return func() { _ = removeLockIfUnchanged(path, data) }, nil
		}
		if !errors.Is(createErr, os.ErrExist) {
			return nil, createErr
		}
		stale, existing, staleErr := staleProcessLock(path)
		if staleErr != nil {
			return nil, staleErr
		}
		if !stale {
			return nil, errProcessLockHeld
		}
		if err := removeLockIfUnchanged(path, existing); err != nil {
			if errors.Is(err, errProcessLockHeld) {
				continue
			}
			return nil, err
		}
	}
	return nil, errProcessLockHeld
}

func staleProcessLock(path string) (bool, []byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil, nil
		}
		return false, nil, err
	}
	if !info.Mode().IsRegular() {
		return false, nil, errors.New("interactive lock path is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, nil, err
	}
	var record processLockRecord
	if json.Unmarshal(data, &record) != nil || record.PID <= 0 || record.Token == "" {
		if time.Since(info.ModTime()) < 5*time.Minute {
			return false, data, nil
		}
		return true, data, nil
	}
	return !processRunning(record.PID), data, nil
}

func removeLockIfUnchanged(path string, expected []byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !bytes.Equal(current, expected) {
		return errProcessLockHeld
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove process lock: %w", err)
	}
	return nil
}
