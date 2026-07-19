package launchpad

import (
	"strings"
	"testing"
)

func TestRedactReportRemovesIdentityAndCredentialLikeData(t *testing.T) {
	report := Report{
		Snapshot: &Snapshot{Hostname: "PRIVATE-HOST", Tailscale: TransportState{IP: "100.64.1.2"}},
		Results: []ActionResult{{
			Output: `C:\Users\alice\.ssh\id.pub ssh-ed25519 AAAA alice@example token=secret`,
			Error:  "/home/alice/file cookie=secret",
		}},
	}
	data := strings.ToLower(report.Results[0].Output + report.Results[0].Error)
	redacted := RedactReport(report)
	joined := strings.ToLower(redacted.Results[0].Output + redacted.Results[0].Error)
	for _, secret := range []string{"alice", "secret", "100.64.1.2", "private-host"} {
		if strings.Contains(joined+strings.ToLower(redacted.Snapshot.Hostname+redacted.Snapshot.Tailscale.IP), secret) {
			t.Fatalf("redacted report retained %q (source %q)", secret, data)
		}
	}
}
