package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func TestReleaseVersionConsistency(t *testing.T) {
	readJSON := func(path string) map[string]any {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	frontend := readJSON(filepath.Join("frontend", "package.json"))
	if frontend["version"] != launchpad.Version {
		t.Fatalf("frontend version %v != Go version %s", frontend["version"], launchpad.Version)
	}
	wails := readJSON("wails.json")
	info, ok := wails["info"].(map[string]any)
	if !ok || info["productVersion"] != launchpad.Version {
		t.Fatalf("Wails product version %v != Go version %s", info["productVersion"], launchpad.Version)
	}
	if _, err := os.Stat(filepath.Join(".github", "release-notes-v"+launchpad.Version+".md")); err != nil {
		t.Fatalf("release notes for current candidate: %v", err)
	}
}
