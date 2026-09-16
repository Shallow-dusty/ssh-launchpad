package launchpad

import "testing"

// The generated-command syntax tests can only catch broken Windows PowerShell
// (or broken POSIX shell) when the fixture actually plans those actions. A
// fixture that stops at the phased install action silently reduced coverage
// once already, so the action set is asserted explicitly.
func TestSyntaxFixtureCoversWindowsFirewallAction(t *testing.T) {
	actions := generatedSyntaxTestActions(t, PlatformWindows)
	for _, action := range actions {
		if action.Operation == "configure_firewall" {
			if len(action.RollbackCommand) == 0 {
				t.Fatal("firewall action needs a rollback command in the syntax fixture")
			}
			return
		}
	}
	ids := []string{}
	for _, action := range actions {
		ids = append(ids, action.Operation)
	}
	t.Fatalf("syntax fixture does not cover configure_firewall: %v", ids)
}
