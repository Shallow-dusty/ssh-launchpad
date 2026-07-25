package main

import "testing"

func TestSafeCardFilename(t *testing.T) {
	if got, want := safeCardFilename(` Shallow:Main/PC? `), "Shallow-Main-PC-"; got != want {
		t.Fatalf("safe card filename mismatch: got %q want %q", got, want)
	}
	if got := safeCardFilename(`...`); got != "ssh-launchpad-personal" {
		t.Fatalf("empty sanitized name should use fallback: %q", got)
	}
}
