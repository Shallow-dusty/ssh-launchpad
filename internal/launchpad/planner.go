package launchpad

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type Planner struct{}

// tailscaleAuthCommandMarker stands in for the real tailscale-up argv in the
// reviewable plan. The auth key is materialized into the command only at Apply
// time so the secret never appears in the inspectable plan or the journal;
// the plan digest still binds it because the digest covers the whole profile.
const tailscaleAuthCommandMarker = "__ssh_launchpad_tailscale_auth__"

func (Planner) Build(profile Profile, snapshot Snapshot) (plan Plan) {
	plan = Plan{
		Timestamp:   time.Now().UTC(),
		ProfileName: profile.Name,
		Platform:    snapshot.Platform,
		ReadOnly:    true,
		HighestRisk: RiskLow,
	}
	defer func() {
		for index := range plan.Actions {
			if snapshot.SessionTransport == "ssh" && isSelfCutOperation(plan.Actions[index].Operation) {
				plan.Actions[index].SelfCutRisk = true
				if plan.Actions[index].Risk == RiskLow || plan.Actions[index].Risk == RiskMedium {
					plan.Actions[index].Risk = RiskHigh
				}
				plan.SelfCutDetected = true
			}
			if riskRank(plan.Actions[index].Risk) > riskRank(plan.HighestRisk) {
				plan.HighestRisk = plan.Actions[index].Risk
			}
		}
		plan.NoChanges = len(plan.Actions) == 0 && len(plan.Blockers) == 0
		plan.Digest = PlanDigest(profile, plan)
	}()
	if profile.Target.Platform != PlatformAuto && profile.Target.Platform != snapshot.Platform {
		plan.Blockers = append(plan.Blockers, fmt.Sprintf("The profile targets %s, but this process detected %s. Apply is blocked on the wrong target platform.", profile.Target.Platform, snapshot.Platform))
		return plan
	}
	if profile.Target.WSL && snapshot.Platform != PlatformWSL {
		plan.Blockers = append(plan.Blockers, "The profile requires a WSL target, but this process is not running inside WSL.")
		return plan
	}
	if !profile.SSH.Enabled {
		plan.NoChanges = true
		plan.Warnings = append(plan.Warnings, "SSH is disabled in the selected profile; no SSH mutations were planned.")
		return plan
	}
	if profile.Transport.Mode == "tailnet" && !snapshot.Tailscale.Online {
		transportReadyAfterApply := false
		if !snapshot.Tailscale.Installed && profile.Transport.Install {
			action := installTailscaleAction(profile, snapshot)
			if len(action.Command) == 0 {
				plan.Blockers = append(plan.Blockers, action.Reason)
			} else {
				plan.Actions = append(plan.Actions, action)
			}
		}
		switch {
		case len(plan.Blockers) > 0:
			// The blocker above already explains why no transport action was planned.
		case strings.TrimSpace(profile.Transport.AuthKey) != "" && (snapshot.Tailscale.Installed || profile.Transport.Install):
			plan.Actions = append(plan.Actions, authenticateTailscaleAction(snapshot))
			plan.Warnings = append(plan.Warnings, "The profile carries a Tailscale auth key. It is used only during Apply and never appears in the plan, journal, or reports.")
			transportReadyAfterApply = true
		case !snapshot.Tailscale.Installed && profile.Transport.Install:
			plan.Warnings = append(plan.Warnings, "This is a phased setup. After Tailscale is installed, sign in and run Check again; SSH and firewall changes are intentionally deferred.")
		default:
			plan.Blockers = append(plan.Blockers, "Tailnet exposure is selected, but Tailscale is not online. Install/sign in to Tailscale, then run Check again.")
		}
		if !transportReadyAfterApply {
			plan.NoChanges = len(plan.Actions) == 0 && len(plan.Blockers) == 0
			return plan
		}
	}
	if !snapshot.SSHClient.Installed || !snapshot.SSHServer.Installed ||
		(snapshot.Platform == PlatformWindows && snapshot.SSHServer.BinaryExists != nil && !*snapshot.SSHServer.BinaryExists) {
		if profile.Download.Strategy != "official" && profile.Download.Strategy != "package-manager" {
			plan.Blockers = append(plan.Blockers, "OpenSSH installation supports only the configured system package manager; offline/mirror/proxy/cache strategies are unsupported. No online fallback will be attempted.")
			return plan
		}
		if snapshot.SSHServer.Installed && snapshot.SSHServer.BinaryExists != nil && !*snapshot.SSHServer.BinaryExists {
			plan.Actions = append(plan.Actions, repairSSHInstallAction(profile, snapshot))
		} else {
			plan.Actions = append(plan.Actions, installSSHAction(profile, snapshot))
		}
		plan.Warnings = append(plan.Warnings, "Phased setup: restore OpenSSH first, then Check and review a new plan before authentication/firewall changes. The package manager may start its default service.")
		return plan
	}
	if snapshot.SSHPolicyError != "" {
		plan.Blockers = append(plan.Blockers, snapshot.SSHPolicyError)
		return plan
	}
	if snapshot.SSHPolicyNotInitialized {
		plan.Warnings = append(plan.Warnings, "The target has no SSH policy file yet (a fresh Windows OpenSSH install creates it on first service start). Apply writes the packaged sshd_config_default template with the managed block instead of guessing existing policy.")
	}
	if len(snapshot.Firewall.ThirdPartyBroadRules) > 0 {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("The installed third-party transport added broad inbound rules (%s) that admit every port from their firewall profiles. SSH Launchpad scopes only its own rule (%s); review or narrow those rules where the matching profile is untrusted.", strings.Join(uniqueStrings(snapshot.Firewall.ThirdPartyBroadRules), ", "), strings.Join(exposureScopes(profile, snapshot), ", ")))
	}
	configDrift := snapshot.SSHPort != profile.SSH.Port || snapshot.SSHPort == 0 || len(snapshot.SSHPorts) > 1 ||
		!snapshot.SSHConfigValid || !snapshot.SSHAuthenticationChecked ||
		snapshot.SSHPasswordAuthentication != profile.SSH.PasswordAuthentication ||
		snapshot.SSHKbdInteractiveAuthentication || !snapshot.SSHPubkeyAuthentication
	if configDrift {
		plan.Actions = append(plan.Actions, configureSSHAction(profile, snapshot))
	}
	if !profile.SSH.PasswordAuthentication && len(profile.SSH.PublicKeys) == 0 {
		switch {
		case !snapshot.AuthorizedKeysChecked:
			plan.Blockers = append(plan.Blockers, "Password authentication is disabled, but the existing authorized_keys file could not be verified. Declare a controller public key or fix access before Apply.")
		case snapshot.AuthorizedKeysCount == 0:
			plan.Blockers = append(plan.Blockers, "Password authentication is disabled and no usable public key exists. Declare at least one controller public key before Apply.")
		}
	}
	if len(profile.SSH.PublicKeys) > 0 && !snapshot.AuthorizedKeysChecked {
		plan.Blockers = append(plan.Blockers, "The target authorized_keys file could not be verified. Fix its permissions or run Check with sufficient access before applying.")
	} else if len(profile.SSH.PublicKeys) > 0 && !snapshot.AuthorizedKeysMatch {
		plan.Actions = append(plan.Actions, configureKeysAction(profile, snapshot))
	}
	if profile.Exposure.Mode != "none" && !firewallMatches(snapshot.Firewall, profile, snapshot) {
		scopes := exposureScopes(profile, snapshot)
		switch {
		case !snapshot.Firewall.Checked:
			plan.Blockers = append(plan.Blockers, "The firewall state could not be verified. Apply is blocked until Check can read the complete port-and-scope rule set.")
		case !snapshot.Firewall.Enabled:
			plan.Blockers = append(plan.Blockers, "The detected firewall provider is not active on every relevant profile. Enable it before exposing SSH.")
		case !supportedFirewallProvider(snapshot.Platform, snapshot.Firewall.Provider):
			plan.Blockers = append(plan.Blockers, "No supported firewall provider is available for this target; SSH exposure cannot be verified safely.")
		case len(snapshot.Firewall.UnresolvedBroadRules) > 0:
			plan.Blockers = append(plan.Blockers, "Broad inbound firewall rules that cover multiple ports also expose SSH. Review them manually before Apply: "+strings.Join(snapshot.Firewall.UnresolvedBroadRules, ", "))
		case len(snapshot.Firewall.PortRangeRules) > 0:
			plan.Blockers = append(plan.Blockers, "Existing inbound firewall rules cover a range of ports that includes SSH. Replace or narrow them before Apply: "+strings.Join(snapshot.Firewall.PortRangeRules, ", "))
		case snapshot.Firewall.BroadExposure && !(snapshot.Platform == PlatformWindows && len(snapshot.Firewall.ConflictingRules) > 0):
			plan.Blockers = append(plan.Blockers, "An existing broad inbound rule exposes the SSH port and cannot be safely remediated automatically. Restrict or remove it before Apply.")
		case hasUnexpectedFirewallScopes(snapshot.Firewall, scopes, snapshot.Platform == PlatformWindows && len(snapshot.Firewall.ConflictingRules) > 0):
			plan.Blockers = append(plan.Blockers, "Existing inbound rules expose the SSH port to source networks outside the requested scope. Review and remove those rules before Apply.")
		case len(scopes) == 0:
			plan.Blockers = append(plan.Blockers, "No safe source network could be detected for the requested exposure mode. Choose explicit CIDRs or connect the target to the intended network.")
		default:
			action := configureFirewallAction(profile, snapshot, scopes)
			if len(action.Command) == 0 {
				plan.Blockers = append(plan.Blockers, action.Reason)
			} else {
				plan.Actions = append(plan.Actions, action)
			}
		}
	}
	if !snapshot.SSHService.Running || snapshot.SSHService.StartPolicy == "disabled" {
		plan.Actions = append(plan.Actions, enableSSHAction(profile, snapshot))
	}
	return plan
}

// normalizeFirewallScope canonicalizes firewall scope strings so semantically
// equal scopes compare equal across providers. Windows normalizes CIDR
// notation into netmask form (100.64.0.0/10 -> 100.64.0.0/255.192.0.0),
// which Go's net.ParseCIDR rejects; convert netmask form back to CIDR before
// comparing, otherwise the planner would consider a correctly scoped rule
// drifted on every Check and rebuild it on every Apply.
func normalizeFirewallScope(scope string) string {
	part := strings.TrimSpace(scope)
	if part == "" {
		return ""
	}
	if _, network, err := net.ParseCIDR(part); err == nil {
		return strings.ToLower(network.String())
	}
	if ip, mask, ok := strings.Cut(part, "/"); ok {
		parsedIP := net.ParseIP(strings.TrimSpace(ip)).To4()
		parsedMask := net.ParseIP(strings.TrimSpace(mask)).To4()
		if parsedIP != nil && parsedMask != nil {
			if ones, bits := net.IPMask(parsedMask).Size(); bits == 32 {
				if _, network, err := net.ParseCIDR(fmt.Sprintf("%s/%d", parsedIP, ones)); err == nil {
					return strings.ToLower(network.String())
				}
			}
		}
	}
	return strings.ToLower(part)
}

func baseAction(id, operation, layer string, risk Risk, summary, reason string) Action {
	return Action{ID: id, Operation: operation, Layer: layer, Risk: risk, Summary: summary, Reason: reason, Mutating: true}
}

func firewallMatches(state FirewallState, profile Profile, snapshot Snapshot) bool {
	if !state.Checked || !state.Enabled || !supportedFirewallProvider(snapshot.Platform, state.Provider) || state.BroadExposure || len(state.UnresolvedBroadRules) > 0 || len(state.PortRangeRules) > 0 || len(state.StaleManagedRules) > 0 || !containsInt(state.Ports, profile.SSH.Port) {
		return false
	}
	desired := firewallScopeSet(exposureScopes(profile, snapshot))
	existing := firewallScopeSet(state.Scopes)
	if len(desired) == 0 || len(existing) != len(desired) {
		return false
	}
	for scope := range desired {
		if !existing[scope] {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func hasUnexpectedFirewallScopes(state FirewallState, desiredScopes []string, ignoreRemediableBroad bool) bool {
	desired := firewallScopeSet(desiredScopes)
	managed := firewallScopeSet(state.ManagedScopes)
	for scope := range firewallScopeSet(state.Scopes) {
		if ignoreRemediableBroad && broadFirewallScope(scope) {
			continue
		}
		if managed[scope] {
			// Only the managed rule carries this scope, and Apply rewrites that
			// rule in place, so it is not unexplained inbound exposure.
			continue
		}
		if !desired[scope] {
			return true
		}
	}
	return false
}

// managedFirewallRuleName mirrors the Windows rule Apply owns and updates in
// place. The probe uses it to separate our own scope from foreign exposure.
func managedFirewallRuleName(port int) string {
	return fmt.Sprintf("SSH-Launchpad-TCP-%d", port)
}

func firewallScopeSet(scopes []string) map[string]bool {
	result := map[string]bool{}
	for _, raw := range scopes {
		parts := strings.FieldsFunc(raw, func(value rune) bool {
			return value == ',' || unicode.IsSpace(value)
		})
		for _, part := range parts {
			part = normalizeFirewallScope(part)
			if part == "" {
				continue
			}
			result[part] = true
		}
	}
	return result
}

func supportedFirewallProvider(platform Platform, provider string) bool {
	switch platform {
	case PlatformWindows:
		return provider == "windows-firewall"
	case PlatformLinux, PlatformWSL:
		return provider == "ufw" || provider == "firewall-cmd"
	case PlatformMacOS:
		return provider == "application-firewall"
	default:
		return false
	}
}

func exposureScopes(profile Profile, snapshot Snapshot) []string {
	switch profile.Exposure.Mode {
	case "tailnet":
		return []string{"100.64.0.0/10", "fd7a:115c:a1e0::/48"}
	case "lan":
		if snapshot.Platform == PlatformWindows {
			return []string{"LocalSubnet"}
		}
		return append([]string(nil), snapshot.Network.LANScopes...)
	case "custom":
		valid := make([]string, 0, len(profile.Exposure.CustomCIDRs))
		for _, cidr := range profile.Exposure.CustomCIDRs {
			if _, _, err := net.ParseCIDR(cidr); err == nil {
				valid = append(valid, cidr)
			}
		}
		return valid
	default:
		return nil
	}
}

func psStringArray(values []string) string {
	if len(values) == 0 {
		return "@()"
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "'"+strings.ReplaceAll(value, "'", "''")+"'")
	}
	return "@(" + strings.Join(quoted, ",") + ")"
}

func psCommand(script string) []string {
	return []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
}

func unixCommand(script string) []string {
	return []string{"/bin/sh", "-c", script}
}

func shQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func containsInt(values []int, value int) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func isSelfCutOperation(operation string) bool {
	switch operation {
	case "configure_sshd", "enable_sshd", "configure_firewall", "install_tailscale", "install_ssh", "repair_ssh_install", "authenticate_tailscale":
		return true
	default:
		return false
	}
}

func riskRank(r Risk) int {
	return map[Risk]int{RiskLow: 1, RiskMedium: 2, RiskHigh: 3, RiskCritical: 4}[r]
}

func stateDir(profile Profile) string {
	if profile.Advanced.StateDir != "" {
		return profile.Advanced.StateDir
	}
	return filepath.Join(".", "artifacts")
}
