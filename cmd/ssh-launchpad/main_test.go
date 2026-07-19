package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func TestLanguageCatalogsAreComplete(t *testing.T) {
	if missing := languageCompleteness(); len(missing) != 0 {
		t.Fatalf("missing translations: %v", missing)
	}
}

func TestGlobalOptionsDoNotChangeCommandShape(t *testing.T) {
	options, args, err := parseGlobalOptions([]string{"check", "--lang", "zh-CN", "--json", "--non-interactive", "--output", "report.json"})
	if err != nil {
		t.Fatal(err)
	}
	if options.lang != langZH || !options.jsonOnly || !options.nonInteractive {
		t.Fatalf("unexpected options: %+v", options)
	}
	if len(args) != 3 || args[0] != "check" || args[1] != "--output" {
		t.Fatalf("unexpected filtered args: %v", args)
	}
}

func TestReportFileIsUTF8JSONWithoutBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "中文-report.json")
	report := launchpad.Report{SchemaVersion: 1, Version: "test", Stage: launchpad.StageCheck, Success: true, ExitCode: 0, ProfileName: "中文"}
	if err := writeReport(path, report); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		t.Fatal("machine JSON must be UTF-8 without BOM")
	}
	var decoded launchpad.Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON not parseable: %v", err)
	}
	if decoded.ProfileName != "中文" {
		t.Fatalf("UTF-8 content changed: %q", decoded.ProfileName)
	}
}

func TestElevatedHelperRejectsTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, []byte(`{"changed":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code := runElevatedApply([]string{"--request", path, "--sha256", "00", "--lang", "en"})
	if code != launchpad.ExitInvalidProfile {
		t.Fatalf("expected tampered request rejection, got %d", code)
	}
}

func TestUnknownLanguageIsRejected(t *testing.T) {
	if _, _, err := parseGlobalOptions([]string{"--lang", "fr", "check"}); err == nil {
		t.Fatal("expected unsupported language error")
	}
}
