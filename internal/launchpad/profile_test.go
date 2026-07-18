package launchpad

import "testing"

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
