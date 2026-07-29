package main

import (
	"strings"
	"testing"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func TestSafeCardFilename(t *testing.T) {
	if got, want := safeCardFilename(` Shallow:Main/PC? `), "Shallow-Main-PC-"; got != want {
		t.Fatalf("safe card filename mismatch: got %q want %q", got, want)
	}
	if got := safeCardFilename(`...`); got != "ssh-launchpad-personal" {
		t.Fatalf("empty sanitized name should use fallback: %q", got)
	}
}

func TestExportProfileOmitsPersonalCardAuthKey(t *testing.T) {
	profile := launchpad.DefaultProfile()
	profile.Transport.AuthKey = "tskey-" + "auth-example-profile-export"
	data, err := marshalExportProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), profile.Transport.AuthKey) || strings.Contains(string(data), "authKey:") {
		t.Fatalf("ordinary profile export must not contain a Tailscale auth key:\n%s", data)
	}
}
