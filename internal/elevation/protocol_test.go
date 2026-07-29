package elevation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func confirmedApplyOptions() launchpad.ApplyOptions {
	return launchpad.ApplyOptions{Confirmed: true, ExpectedPlanDigest: strings.Repeat("a", 64)}
}

func TestRequestDigestRejectsTampering(t *testing.T) {
	directory := t.TempDir()
	responsePath := filepath.Join(directory, "response.json")
	if err := PrecreateFile(responsePath); err != nil {
		t.Fatal(err)
	}
	request := NewRequest(
		launchpad.DefaultProfile(),
		confirmedApplyOptions(),
		responsePath,
		"",
		"en",
	)
	path := filepath.Join(directory, "request.json")
	digest, err := WriteRequest(path, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRequest(path, digest); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRequest(path, digest); err == nil {
		t.Fatal("tampered request should be rejected")
	}
}

func TestConsumeRequestRemovesCredentialBearingFile(t *testing.T) {
	directory := t.TempDir()
	responsePath := filepath.Join(directory, "response.json")
	if err := PrecreateFile(responsePath); err != nil {
		t.Fatal(err)
	}
	profile := launchpad.DefaultProfile()
	profile.Transport.AuthKey = "tskey-" + "auth-example-once"
	request := NewRequest(profile, confirmedApplyOptions(), responsePath, "", "en")
	path := filepath.Join(directory, "request.json")
	digest, err := WriteRequest(path, request)
	if err != nil {
		t.Fatal(err)
	}
	consumed, err := ConsumeRequest(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	if consumed.Profile.Transport.AuthKey != profile.Transport.AuthKey {
		t.Fatal("consumed request lost the auth key before Apply")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("credential-bearing request still exists after verification: %v", err)
	}
}

func TestElevationRequestRequiresReviewedPlanDigest(t *testing.T) {
	directory := t.TempDir()
	responsePath := filepath.Join(directory, "response.json")
	if err := PrecreateFile(responsePath); err != nil {
		t.Fatal(err)
	}
	request := NewRequest(launchpad.DefaultProfile(), launchpad.ApplyOptions{Confirmed: true}, responsePath, "", "en")
	if _, err := WriteRequest(filepath.Join(directory, "request.json"), request); err == nil {
		t.Fatal("elevation request without reviewed plan digest was accepted")
	}
}

func TestWriteRequestRefusesExistingPath(t *testing.T) {
	directory := t.TempDir()
	responsePath := filepath.Join(directory, "response.json")
	if err := PrecreateFile(responsePath); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "request.json")
	if err := os.WriteFile(path, []byte("do not replace"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := NewRequest(
		launchpad.DefaultProfile(),
		confirmedApplyOptions(),
		responsePath,
		"",
		"en",
	)
	if _, err := WriteRequest(path, request); err == nil {
		t.Fatal("existing request path should not be overwritten")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "do not replace" {
		t.Fatalf("existing request changed: %q", data)
	}
}

func TestVerifiedRequestCannotRedirectElevatedOutput(t *testing.T) {
	directory := t.TempDir()
	responsePath := filepath.Join(directory, "response.json")
	if err := PrecreateFile(responsePath); err != nil {
		t.Fatal(err)
	}
	request := NewRequest(
		launchpad.DefaultProfile(),
		confirmedApplyOptions(),
		filepath.Join(t.TempDir(), "unrelated.json"),
		"",
		"en",
	)
	path := filepath.Join(directory, "request.json")
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyRequest(path, Digest(data)); err == nil {
		t.Fatal("elevated output path outside the request directory should be rejected")
	}
}

func TestResponseUsesPrecreatedRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "response.json")
	if err := PrecreateFile(path); err != nil {
		t.Fatal(err)
	}
	response := Response{Report: launchpad.Report{Success: true, ExitCode: launchpad.ExitOK}}
	if err := WriteResponse(path, response); err != nil {
		t.Fatal(err)
	}
	got, err := ReadResponse(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Report.Success {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestWindowsStartProcessScriptPreservesOneArgumentLine(t *testing.T) {
	script := WindowsStartProcessScript(
		`C:\Program Files\SSH Launchpad\SSH-Launchpad.exe`,
		[]string{"--elevated-helper", "--request", `C:\Users\Example User\request.json`, "--sha256", "abc123"},
	)
	for _, required := range []string{
		"Start-Process",
		"-Verb RunAs",
		`--request "C:\Users\Example User\request.json"`,
		"NativeErrorCode -eq 1223",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("launcher command missing %q: %s", required, script)
		}
	}
	if strings.Contains(script, "-ArgumentList @(") {
		t.Fatalf("argument arrays would lose quoting: %s", script)
	}
}

func TestUTF16LEBase64PreservesSurrogatePairs(t *testing.T) {
	if got, want := UTF16LEBase64("A😀"), "QQA92ADe"; got != want {
		t.Fatalf("unexpected UTF-16LE base64: got %s want %s", got, want)
	}
}
