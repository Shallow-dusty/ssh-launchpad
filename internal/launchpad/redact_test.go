package launchpad

import (
	"strings"
	"testing"
)

func TestRedactReportRemovesIdentityAndCredentialLikeData(t *testing.T) {
	report := Report{
		Snapshot: &Snapshot{Hostname: "PRIVATE-HOST", TargetUser: "alice", Tailscale: TransportState{IP: "fd7a:115c:a1e0::1"}},
		Plan: &Plan{Actions: []Action{{
			Operation: "configure_keys",
			Command:   []string{"powershell", "-Command", "AAAAC3Nza-private-key-payload"},
		}}},
		Results: []ActionResult{{
			Output: `C:\Users\alice\.ssh\id.pub ssh-ed25519 AAAA alice@example token=secret password=hunter2`,
			Error:  "/home/alice/file cookie=secret fd7a:115c:a1e0::1",
		}},
	}
	data := strings.ToLower(report.Results[0].Output + report.Results[0].Error)
	redacted := RedactReport(report)
	joined := strings.ToLower(redacted.Results[0].Output + redacted.Results[0].Error)
	for _, secret := range []string{"alice", "secret", "hunter2", "fd7a:115c:a1e0::1", "private-host", "aaaac3nza-private-key-payload"} {
		all := joined + strings.ToLower(redacted.Snapshot.Hostname+redacted.Snapshot.TargetUser+redacted.Snapshot.Tailscale.IP) + strings.ToLower(strings.Join(redacted.Plan.Actions[0].Command, " "))
		if strings.Contains(all, secret) {
			t.Fatalf("redacted report retained %q (source %q)", secret, data)
		}
	}
}
