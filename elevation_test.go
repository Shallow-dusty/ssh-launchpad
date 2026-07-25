package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelperPowerShellUsesRunAsAndExactDigest(t *testing.T) {
	command := helperPowerShell(
		`C:\Program Files\SSH Launchpad\SSH-Launchpad.exe`,
		`C:\Users\Example User\AppData\Local\SSH Launchpad\jobs\request.json`,
		"abc123",
	)
	for _, required := range []string{
		"Start-Process",
		"-Verb RunAs",
		"--elevated-helper",
		`--request "C:\Users\Example User\AppData\Local\SSH Launchpad\jobs\request.json"`,
		"abc123",
		"NativeErrorCode -eq 1223",
	} {
		if !strings.Contains(command, required) {
			t.Fatalf("launcher command missing %q: %s", required, command)
		}
	}
	if strings.Contains(command, "-ArgumentList @(") {
		t.Fatalf("Start-Process argument arrays lose quoting when joined: %s", command)
	}
}

func TestFinalizeUACJobDistinguishesCancellationFromHelperFailure(t *testing.T) {
	cancelled := &elevatedJobRecord{
		status:       ElevatedJob{ID: "cancelled"},
		responsePath: filepath.Join(t.TempDir(), "missing-response.json"),
	}
	finalizeUACJob(cancelled, errUACCancelled)
	if cancelled.status.State != "cancelled" {
		t.Fatalf("explicit UAC cancellation was classified as %q", cancelled.status.State)
	}

	failed := &elevatedJobRecord{
		status:       ElevatedJob{ID: "failed"},
		responsePath: filepath.Join(t.TempDir(), "missing-response.json"),
	}
	finalizeUACJob(failed, errors.New("exit status 2"))
	if failed.status.State != "failed" {
		t.Fatalf("helper failure was misclassified as %q", failed.status.State)
	}
	if !strings.Contains(failed.status.Error, "exit status 2") {
		t.Fatalf("helper failure detail was lost: %s", failed.status.Error)
	}
}
