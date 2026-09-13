package launchpad

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
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

// repairSSHInstallAction restores a Windows OpenSSH Server installation whose
// sshd executable has gone missing (typically quarantined by antivirus)
// while the service registration and the Windows capability state still
// report installed. It removes and re-adds the Windows capability. When the
// server was installed outside Windows capabilities, the command fails with
// actionable guidance instead of guessing.
func repairSSHInstallAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("repair-ssh-install", "repair_ssh_install", "ssh-packages", RiskMedium, "Reinstall the OpenSSH Server capability whose sshd.exe is missing", "The sshd service is registered but its executable is gone, usually because antivirus software quarantined it.")
	a.RequiresElevation = true
	a.Reversible = false
	if snapshot.Platform != PlatformWindows {
		a.Command = nil
		return a
	}
	a.Command = psCommand(`$ErrorActionPreference='Stop'; $n='OpenSSH.Server~~~~0.0.1.0'; Stop-Service sshd -Force -ErrorAction SilentlyContinue; $c=Get-WindowsCapability -Online -Name $n -ErrorAction Stop; if($c.State -ne 'Installed'){throw 'sshd.exe is missing but OpenSSH Server is not a Windows capability; rerun the original OpenSSH installer or the offline payload instead'}; Write-Progress -Activity 'OpenSSH Server repair' -Status 'Removing broken capability'; Remove-WindowsCapability -Online -Name $n | Out-Null; Write-Progress -Activity 'OpenSSH Server repair' -Status 'Reinstalling'; Add-WindowsCapability -Online -Name $n | Out-Null`)
	return a
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

func installSSHAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("install-ssh", "install_ssh", "ssh-packages", RiskMedium, "Install the OpenSSH client and server", "One or both OpenSSH capabilities are missing.")
	a.RequiresElevation = true
	a.Reversible = false
	switch snapshot.Platform {
	case PlatformWindows:
		a.Command = psCommand(`$ErrorActionPreference='Stop'; foreach($n in 'OpenSSH.Client~~~~0.0.1.0','OpenSSH.Server~~~~0.0.1.0'){ $c=Get-WindowsCapability -Online -Name $n; if($c.State -ne 'Installed'){ Write-Progress -Activity 'Windows OpenSSH servicing' -Status $n; Add-WindowsCapability -Online -Name $n | Out-Host } }`)
	case PlatformMacOS:
		a.Mutating = false
		a.Risk = RiskLow
		a.Summary = "Use the OpenSSH components included with macOS"
		a.Command = nil
	case PlatformLinux, PlatformWSL:
		a.Command = unixCommand(linuxInstallSSH(snapshot.PackageManager))
	default:
		a.Command = nil
	}
	return a
}

// authenticateTailscaleAction joins the tailnet with the profile's auth key.
// The step is deliberately irreversible: leaving a tailnet is an account-level
// decision, and rollback of a partially applied plan still covers every SSH
// and firewall change that follows. Joining a tailnet on its own does not
// expose SSH.
func authenticateTailscaleAction(snapshot Snapshot) Action {
	a := baseAction("authenticate-tailscale", "authenticate_tailscale", "transport", RiskMedium, "Join the configured Tailscale network with the profile auth key", "The profile supplies a Tailscale auth key and this device is not online.")
	a.RequiresElevation = snapshot.Platform != PlatformMacOS
	a.Reversible = false
	a.Command = []string{tailscaleAuthCommandMarker}
	return a
}

func installTailscaleAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("install-tailscale", "install_tailscale", "transport", RiskMedium, "Install Tailscale as the optional secure transport", "Tailnet mode is requested and Tailscale is missing.")
	a.RequiresElevation = true
	a.Reversible = false
	if profile.Download.Strategy != "official" && profile.Download.Strategy != "package-manager" && !(profile.Download.Strategy == "offline" && snapshot.Platform == PlatformWindows) {
		a.Reason = "The Tailscale installer does not support this download strategy. Select the system package manager or a verified Windows offline installer; no fallback will be attempted."
		return a
	}
	switch snapshot.Platform {
	case PlatformWindows:
		if profile.Download.Strategy == "offline" {
			a.Command = []string{profile.Download.OfflineBundle, "/quiet"}
			a.Params = map[string]string{
				"artifactPath":   profile.Download.OfflineBundle,
				"artifactSHA256": profile.Download.OfflineSHA256,
			}
		} else if snapshot.PackageManager == "winget" {
			a.Command = []string{"winget.exe", "install", "--id", "Tailscale.Tailscale", "--exact", "--accept-package-agreements", "--accept-source-agreements"}
		} else {
			a.Command = nil
			a.Reason = "Tailscale installation needs winget or an explicitly selected verified offline bundle."
		}
	case PlatformMacOS:
		if snapshot.PackageManager == "brew" {
			a.Command = []string{"brew", "install", "--cask", "tailscale-app"}
		} else {
			a.Command = nil
			a.Reason = "Tailscale installation needs Homebrew or an explicitly selected verified offline bundle."
		}
	case PlatformLinux, PlatformWSL:
		a.Command = nil
		a.Risk = RiskHigh
		a.Summary = "Install Tailscale from its verified official repository or offline bundle"
		a.Reason = "SSH Launchpad will not execute curl-to-shell installers. Configure a trusted package repository or provide an offline bundle."
	}
	return a
}

func configureSSHAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("configure-sshd", "configure_sshd", "ssh-config", RiskHigh, fmt.Sprintf("Set SSH port %d and key-oriented authentication", profile.SSH.Port), "The effective SSH port, configuration validity, or authentication policy does not match the profile.")
	a.RequiresElevation = true
	a.Reversible = true
	a.Command, a.RollbackCommand = configCommands(profile, snapshot)
	a.Params = map[string]string{"port": strconv.Itoa(profile.SSH.Port), "managedBlock": "SSH-LAUNCHPAD"}
	return a
}

func configureKeysAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("configure-authorized-keys", "configure_keys", "authentication", RiskHigh, "Merge the declared SSH public keys", "One or more declared controller public keys are not present. Existing keys are preserved and the file is backed up.")
	a.RequiresElevation = snapshot.Platform == PlatformWindows && snapshot.TargetUserIsAdmin
	a.Reversible = true
	a.Command, a.RollbackCommand = keyCommands(profile, snapshot)
	a.Params = map[string]string{"keyCount": strconv.Itoa(len(profile.SSH.PublicKeys))}
	return a
}

func enableSSHAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("enable-sshd", "enable_sshd", "ssh-service", RiskMedium, "Enable and start the SSH service", "The SSH service is missing from the desired running state.")
	a.RequiresElevation = true
	a.Reversible = true
	switch snapshot.Platform {
	case PlatformWindows:
		a.Command = psCommand(`$ErrorActionPreference='Stop'; Set-Service sshd -StartupType Automatic; if((Get-Service sshd).Status -ne 'Running'){Start-Service sshd}`)
		policy := snapshot.SSHService.StartPolicy
		if strings.EqualFold(policy, "auto") || strings.EqualFold(policy, "automatic") {
			policy = "Automatic"
		} else if strings.EqualFold(policy, "disabled") {
			policy = "Disabled"
		} else {
			policy = "Manual"
		}
		verb := "Stop-Service"
		if snapshot.SSHService.Running {
			verb = "Start-Service"
		}
		a.RollbackCommand = psCommand(fmt.Sprintf("$ErrorActionPreference='Stop'; Set-Service sshd -StartupType Manual; %s sshd; Set-Service sshd -StartupType %s", verb, policy))
	case PlatformMacOS:
		a.Command = []string{"systemsetup", "-setremotelogin", "on"}
		a.RollbackCommand = []string{"systemsetup", "-setremotelogin", map[bool]string{true: "on", false: "off"}[snapshot.SSHService.Running]}
	default:
		service := serviceName(profile, snapshot)
		a.Command = []string{"systemctl", "enable", "--now", service}
		startup := "disable"
		if snapshot.SSHService.StartPolicy == "enabled" {
			startup = "enable"
		}
		state := "stop"
		if snapshot.SSHService.Running {
			state = "start"
		}
		a.RollbackCommand = unixCommand(fmt.Sprintf("set -eu; systemctl %s %s; systemctl %s %s", startup, shQuote(service), state, shQuote(service)))
	}
	return a
}

func configureFirewallAction(profile Profile, snapshot Snapshot, scopes []string) Action {
	if snapshot.Platform != PlatformWindows {
		existing := firewallScopeSet(snapshot.Firewall.Scopes)
		missing := make([]string, 0, len(scopes))
		for _, scope := range scopes {
			if !existing[normalizeFirewallScope(scope)] {
				missing = append(missing, scope)
			}
		}
		scopes = missing
	}
	a := baseAction("configure-firewall", "configure_firewall", "firewall", RiskHigh, fmt.Sprintf("Allow TCP %d from %s only", profile.SSH.Port, strings.Join(scopes, ", ")), "No port-and-scope-aware firewall rule matches the profile.")
	a.RequiresElevation = true
	a.Reversible = true
	name := fmt.Sprintf("SSH-Launchpad-TCP-%d", profile.SSH.Port)
	switch snapshot.Platform {
	case PlatformWindows:
		a.Command, a.RollbackCommand = windowsFirewallCommands(name, profile.SSH.Port, scopes, snapshot.Firewall.ConflictingRules)
	case PlatformMacOS:
		a.Command = nil
		a.Risk = RiskMedium
		a.Summary = "Review the macOS application firewall or upstream packet filter"
		a.Reason = "The macOS application firewall does not provide a portable port-and-CIDR rule interface. SSH Launchpad will not claim a rule it cannot verify."
		a.Reversible = false
	default:
		switch snapshot.Firewall.Provider {
		case "firewall-cmd":
			var add, remove []string
			for _, scope := range scopes {
				family := "ipv4"
				if strings.Contains(scope, ":") {
					family = "ipv6"
				}
				rich := fmt.Sprintf(`rule family="%s" source address="%s" port port="%d" protocol="tcp" accept`, family, scope, profile.SSH.Port)
				add = append(add, "firewall-cmd --permanent --add-rich-rule "+shQuote(rich))
				remove = append(remove, "firewall-cmd --permanent --remove-rich-rule "+shQuote(rich))
			}
			add = append(add, "firewall-cmd --reload")
			remove = append(remove, "firewall-cmd --reload")
			rollbackOnFailure := make([]string, 0, len(remove))
			for _, command := range remove {
				rollbackOnFailure = append(rollbackOnFailure, "("+command+") || true")
			}
			a.Command = unixCommand("set -eu; if ! { " + strings.Join(add, " && ") + "; }; then " + strings.Join(rollbackOnFailure, "; ") + "; exit 1; fi")
			a.RollbackCommand = unixCommand("set -eu; " + strings.Join(remove, " && "))
		default:
			var add, remove []string
			for _, scope := range scopes {
				args := "allow from " + shQuote(scope) + " to any port " + strconv.Itoa(profile.SSH.Port) + " proto tcp"
				add = append(add, "ufw "+args)
				remove = append(remove, "ufw --force delete "+args)
			}
			rollbackOnFailure := make([]string, 0, len(remove))
			for _, command := range remove {
				rollbackOnFailure = append(rollbackOnFailure, "("+command+") || true")
			}
			if len(add) == 0 {
				add = []string{"true"}
				remove = []string{"true"}
			}
			a.Command = unixCommand("set -eu; if ! { " + strings.Join(add, " && ") + "; }; then " + strings.Join(rollbackOnFailure, "; ") + "; exit 1; fi")
			a.RollbackCommand = unixCommand("set -eu; " + strings.Join(remove, " && "))
		}
	}
	a.Params = map[string]string{"ruleName": name, "port": strconv.Itoa(profile.SSH.Port), "scopes": strings.Join(scopes, ",")}
	return a
}

func baseAction(id, operation, layer string, risk Risk, summary, reason string) Action {
	return Action{ID: id, Operation: operation, Layer: layer, Risk: risk, Summary: summary, Reason: reason, Mutating: true}
}

func firewallMatches(state FirewallState, profile Profile, snapshot Snapshot) bool {
	if !state.Checked || !state.Enabled || !supportedFirewallProvider(snapshot.Platform, state.Provider) || state.BroadExposure || len(state.UnresolvedBroadRules) > 0 || len(state.PortRangeRules) > 0 || !containsInt(state.Ports, profile.SSH.Port) {
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

func hasUnexpectedFirewallScopes(state FirewallState, desiredScopes []string, ignoreRemediableBroad bool) bool {
	desired := firewallScopeSet(desiredScopes)
	for scope := range firewallScopeSet(state.Scopes) {
		if ignoreRemediableBroad && broadFirewallScope(scope) {
			continue
		}
		if !desired[scope] {
			return true
		}
	}
	return false
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

func serviceName(profile Profile, snapshot Snapshot) string {
	if profile.Advanced.LinuxSSHService != "" && profile.Advanced.LinuxSSHService != "auto" {
		return profile.Advanced.LinuxSSHService
	}
	if snapshot.SSHService.Name != "" && snapshot.SSHService.Name != "auto" {
		return snapshot.SSHService.Name
	}
	return "sshd"
}

func linuxInstallSSH(manager string) string {
	switch manager {
	case "apt-get":
		return "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y openssh-client openssh-server"
	case "dnf":
		return "dnf install -y openssh-clients openssh-server"
	case "yum":
		return "yum install -y openssh-clients openssh-server"
	case "zypper":
		return "zypper --non-interactive install openssh"
	case "pacman":
		return "pacman -S --noconfirm openssh"
	case "apk":
		return "apk add --no-cache openssh-client openssh-server"
	default:
		return "echo 'No supported package manager detected' >&2; exit 9"
	}
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
