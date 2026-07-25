package launchpad

import (
	"encoding/json"
	"testing"
)

func TestProfileRejectsPrivateKeyAndBadCIDR(t *testing.T) {
	p := DefaultProfile()
	p.SSH.PublicKeys = []string{"-----BEGIN OPENSSH PRIVATE KEY-----"}
	p.Exposure.Mode = "custom"
	p.Exposure.CustomCIDRs = []string{"not-a-network"}
	if err := p.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDefaultProfileIsValid(t *testing.T) {
	if err := DefaultProfile().Validate(); err != nil {
		t.Fatalf("default profile invalid: %v", err)
	}
}

func TestDefaultProfileJSONKeepsEmptyCollections(t *testing.T) {
	data, err := json.Marshal(DefaultProfile())
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	ssh := payload["ssh"].(map[string]any)
	if keys, ok := ssh["publicKeys"].([]any); !ok || len(keys) != 0 {
		t.Fatalf("publicKeys must be an empty JSON array, got %#v", ssh["publicKeys"])
	}
	exposure := payload["exposure"].(map[string]any)
	if cidrs, ok := exposure["customCidrs"].([]any); !ok || len(cidrs) != 0 {
		t.Fatalf("customCidrs must be an empty JSON array, got %#v", exposure["customCidrs"])
	}
	if labels, ok := payload["labels"].(map[string]any); !ok || len(labels) != 0 {
		t.Fatalf("labels must be an empty JSON object, got %#v", payload["labels"])
	}
}

func TestValidatePublicKeyRejectsFakeBase64AndAcceptsParsedKey(t *testing.T) {
	if err := ValidatePublicKey("ssh-ed25519 AAAA fake"); err == nil {
		t.Fatal("syntactically shaped but invalid key must be rejected")
	}
	const valid = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEB controller"
	if err := ValidatePublicKey(valid); err != nil {
		t.Fatalf("valid parsed key rejected: %v", err)
	}
}
