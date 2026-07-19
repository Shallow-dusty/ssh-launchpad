package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestElevatedRequestDigestRejectsTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	original := []byte(`{"confirmed":true}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := requestDigest(original)
	if _, err := verifyRequestDigest(path, digest); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"confirmed":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyRequestDigest(path, digest); err == nil {
		t.Fatal("tampered request should be rejected")
	}
}

func TestHelperPowerShellUsesRunAsAndExactDigest(t *testing.T) {
	command := helperPowerShell(
		`C:\Program Files\SSH Launchpad\SSH-Launchpad.exe`,
		`C:\Temp\request.json`,
		`C:\Temp\response.json`,
		`C:\Temp\events.jsonl`,
		"abc123",
	)
	for _, required := range []string{"Start-Process", "-Verb RunAs", "--elevated-helper", "abc123"} {
		if !strings.Contains(command, required) {
			t.Fatalf("launcher command missing %q: %s", required, command)
		}
	}
}

func TestHelperArgsRequireIntegrityInputs(t *testing.T) {
	if _, err := parseHelperArgs([]string{"--request", "a"}); err == nil {
		t.Fatal("incomplete helper arguments should fail")
	}
}
