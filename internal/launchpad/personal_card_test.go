package launchpad

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const personalCardTestKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller"

func TestPersonalCardRoundTripBuildsGuidedProfile(t *testing.T) {
	profile := DefaultProfile()
	profile.SSH.Port = 2222
	profile.SSH.PublicKeys = []string{personalCardTestKey}
	profile.Transport.Install = true
	profile.Transport.AuthKey = "tskey-auth-example-once"
	card := NewPersonalCard(profile, "Shallow", "Main controller", "Bedroom PC")
	data, err := MarshalPersonalCard(card)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Shallow.sshlaunchpad-card")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPersonalCard(path)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Profile()
	if got.Name != "Shallow" || got.SSH.Port != 2222 || got.Transport.AuthKey != "tskey-auth-example-once" {
		t.Fatalf("unexpected imported profile: %+v", got)
	}
	if got.Labels["cardControllerName"] != "Main controller" || got.Labels["cardNote"] != "Bedroom PC" {
		t.Fatalf("card metadata was not preserved: %+v", got.Labels)
	}
}

func TestPersonalCardRejectsPrivateKeysAndInvalidAuthKey(t *testing.T) {
	profile := DefaultProfile()
	profile.SSH.PublicKeys = []string{"-----BEGIN OPENSSH PRIVATE KEY-----"}
	card := NewPersonalCard(profile, "Unsafe", "", "")
	if err := card.Validate(); err == nil || !strings.Contains(err.Error(), "private keys") {
		t.Fatalf("private key should be rejected: %v", err)
	}

	profile.SSH.PublicKeys = []string{personalCardTestKey}
	profile.Transport.AuthKey = "not-a-tailscale-key"
	card = NewPersonalCard(profile, "Unsafe", "", "")
	if err := card.Validate(); err == nil || !strings.Contains(err.Error(), "tskey-auth-") {
		t.Fatalf("invalid Tailscale key should be rejected: %v", err)
	}
}

func TestLoadPersonalCardRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	profile := DefaultProfile()
	profile.SSH.PublicKeys = []string{personalCardTestKey}
	data, err := MarshalPersonalCard(NewPersonalCard(profile, "Personal", "Controller", ""))
	if err != nil {
		t.Fatal(err)
	}
	for name, payload := range map[string][]byte{
		"unknown field": bytes.Replace(data, []byte(`"displayName": "Personal"`), []byte(`"displayName": "Personal", "privateKey": "no"`), 1),
		"trailing JSON": append(append([]byte(nil), data...), []byte(`{"extra":true}`)...),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.sshlaunchpad-card")
			if err := os.WriteFile(path, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadPersonalCard(path); err == nil {
				t.Fatal("invalid personal card was accepted")
			}
		})
	}
}
