package launchpad

import "testing"

func TestNewerVersionDoesNotOfferDowngrade(t *testing.T) {
	cases := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"0.2.1", "0.2.0", true},
		{"0.3.0", "0.2.9", true},
		{"0.2.0", "0.2.0", false},
		{"0.1.0", "0.2.0", false},
	}
	for _, item := range cases {
		if got := newerVersion(item.candidate, item.current); got != item.want {
			t.Fatalf("newerVersion(%q, %q) = %v, want %v", item.candidate, item.current, got, item.want)
		}
	}
}
