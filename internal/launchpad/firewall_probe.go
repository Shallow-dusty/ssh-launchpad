package launchpad

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func probeFirewall(ctx context.Context, platform Platform, port int) (FirewallState, error) {
	switch platform {
	case PlatformWindows:
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $target=%d; function Test-Port($spec){foreach($part in @($spec)-split ','){$part=$part.Trim(); if($part -in @('Any','*')){return $true}; if($part -match '^(\d+)-(\d+)$' -and $target -ge [int]$Matches[1] -and $target -le [int]$Matches[2]){return $true}; if($part -match '^\d+$' -and [int]$part -eq $target){return $true}}; return $false}; $r=Get-NetFirewallPortFilter -PolicyStore ActiveStore -ErrorAction Stop | Where-Object {$_.Protocol -in @('TCP','6','Any','256') -and (Test-Port $_.LocalPort)} | ForEach-Object {$filter=$_; $rule=Get-NetFirewallRule -AssociatedNetFirewallPortFilter $filter -ErrorAction Stop | Where-Object {$_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow'}; foreach($item in $rule){$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $item -ErrorAction Stop; $app=$item | Get-NetFirewallApplicationFilter -ErrorAction SilentlyContinue; $svc=$item | Get-NetFirewallServiceFilter -ErrorAction SilentlyContinue; [pscustomobject]@{port=$target;name=$item.Name;displayName=$item.DisplayName;scope=($a.RemoteAddress -join ',');exactPort=([string]$filter.LocalPort -eq [string]$target);program=[string]$app.Program;package=[string]$app.Package;service=[string]$svc.Service;protocol=[string]$filter.Protocol;localPort=[string]$filter.LocalPort}}}; $stale=Get-NetFirewallRule -Name 'SSH-Launchpad-TCP-*' -ErrorAction SilentlyContinue | Where-Object {$_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow'} | ForEach-Object {$sp=$_ | Get-NetFirewallPortFilter; $sa=$_ | Get-NetFirewallAddressFilter; [pscustomobject]@{port=$target;name=$_.Name;displayName=$_.DisplayName;scope=($sa.RemoteAddress -join ',');exactPort=$false;program='';package='';service='';protocol=[string]$sp.Protocol;localPort=[string]$sp.LocalPort;managedLeftover=$true}}; ConvertTo-Json -InputObject @(@($r)+@($stale)) -Compress`, port)
		// The inventory sweep enumerates every TCP port filter and its rule
		// associations; measured at 14.4s on an idle, nearly rule-free Windows
		// Server VM without elevation, so real consumer machines with many
		// rules routinely exceed the old 12s limit and get killed mid-sweep.
		out, err := runCommand(ctx, 45*time.Second, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		if err != nil {
			return FirewallState{Provider: "windows-firewall"}, err
		}
		var rules []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Scope       string `json:"scope"`
			ExactPort   bool   `json:"exactPort"`
			Program     string `json:"program"`
			Package     string `json:"package"`
			Service     string `json:"service"`
			Protocol    string `json:"protocol"`
			LocalPort   string `json:"localPort"`
			Leftover    bool   `json:"managedLeftover"`
		}
		if len(strings.TrimSpace(string(out))) > 2 && json.Unmarshal(out, &rules) != nil {
			return FirewallState{Provider: "windows-firewall"}, errors.New("could not parse Windows firewall rule inventory")
		}
		profileScript := `$ErrorActionPreference='Stop'; if(@(Get-NetFirewallProfile -PolicyStore ActiveStore -ErrorAction Stop | Where-Object {-not $_.Enabled -or $_.DefaultInboundAction -eq 'Allow'}).Count -gt 0){exit 1}`
		if _, err := runCommand(ctx, 8*time.Second, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", profileScript); err != nil {
			return FirewallState{Provider: "windows-firewall"}, errors.New("one or more Windows Firewall profiles are disabled or unreadable")
		}
		state := FirewallState{Checked: true, Enabled: true, Provider: "windows-firewall"}
		for _, rule := range rules {
			// Windows firewall program/package/service filters name the receiving
			// process or app container. A rule bound to something other than the
			// SSH daemon cannot deliver inbound traffic to sshd, so it must not be
			// reported as SSH exposure (stock Windows ships many Any-protocol
			// Store-app and Remote Desktop rules that would otherwise block every
			// fresh machine).
			if windowsRuleCannotReachSshd(rule.Program, rule.Package, rule.Service) {
				continue
			}
			if isThirdPartyBroadRule(rule.DisplayName, rule.Protocol, rule.LocalPort, rule.Scope, rule.Program, rule.Package, rule.Service) {
				// Recognised third-party rule that admits every port from a
				// specific firewall profile (e.g. Tailscale's Tailscale-In).
				// Record it so the plan can warn precisely, but do not let it
				// masquerade as unexplained exposure of the SSH port or keep
				// the managed scope from converging.
				if rule.DisplayName != "" {
					state.ThirdPartyBroadRules = append(state.ThirdPartyBroadRules, rule.DisplayName)
				}
				continue
			}
			if stalePort, managedName := managedFirewallRulePort(rule.Name); rule.Leftover || (managedName && stalePort != port) {
				// A managed rule left behind by an earlier SSH port. Apply
				// disables it with recorded rollback state instead of letting
				// it accumulate or block the new scope as foreign exposure.
				if rule.Name != managedFirewallRuleName(port) {
					state.StaleManagedRules = append(state.StaleManagedRules, rule.Name)
				}
				continue
			}
			state.Ports = []int{port}
			managed := rule.Name == managedFirewallRuleName(port)
			for _, scope := range strings.Split(rule.Scope, ",") {
				normalized := normalizeFirewallScope(scope)
				state.Scopes = append(state.Scopes, normalized)
				if managed {
					// The tool owns this rule and rewrites only its RemoteAddress,
					// so its current scope is a reconfiguration, not foreign
					// exposure that must be reviewed before rescoping.
					state.ManagedScopes = append(state.ManagedScopes, normalized)
				}
			}
			name := rule.Name
			if name == "" {
				name = rule.DisplayName
			}
			if !rule.ExactPort {
				if name == "" {
					name = "unnamed port-range rule"
				}
				state.PortRangeRules = append(state.PortRangeRules, name)
			}
			if broadFirewallScope(rule.Scope) {
				state.BroadExposure = true
				if name != "" {
					if rule.ExactPort {
						state.ConflictingRules = append(state.ConflictingRules, name)
					} else {
						state.UnresolvedBroadRules = append(state.UnresolvedBroadRules, name)
					}
				}
			}
		}
		return state, nil
	case PlatformMacOS:
		return FirewallState{Checked: true, Enabled: true, Provider: "application-firewall"}, nil
	default:
		return probeUnixFirewall(ctx, port)
	}
}

// windowsRuleCannotReachSshd reports whether a Windows firewall rule is bound
// to a program, app container, or service that is definitely not the SSH
// daemon. Such a rule can never deliver inbound traffic to sshd, regardless of
// how broad its protocol/port scope looks.
func windowsRuleCannotReachSshd(program, pkg, service string) bool {
	isUnscoped := func(value string) bool {
		value = strings.ToLower(strings.TrimSpace(value))
		return value == "" || value == "any" || value == "*"
	}
	mentionsSSHD := func(value string) bool {
		return strings.Contains(strings.ToLower(value), "sshd")
	}
	if !isUnscoped(pkg) {
		return true
	}
	if !isUnscoped(program) && !mentionsSSHD(program) {
		return true
	}
	if !isUnscoped(service) && !mentionsSSHD(service) {
		return true
	}
	return false
}

// managedFirewallRulePort reports whether a rule name belongs to the managed
// rule family and which SSH port it was created for.
func managedFirewallRulePort(name string) (int, bool) {
	const prefix = "SSH-Launchpad-TCP-"
	if !strings.HasPrefix(name, prefix) {
		return 0, false
	}
	port, err := strconv.Atoi(strings.TrimPrefix(name, prefix))
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

// isThirdPartyBroadRule recognises an any-port inbound rule that a
// well-known third-party product installs and that Apply must not treat as
// unexplained SSH exposure. Tailscale creates one such rule ("Tailscale-In")
// per relevant firewall profile; it admits every port from those profiles, so
// it is reported as a warning instead of silently expanding the managed scope.
func isThirdPartyBroadRule(displayName, protocol, localPort, scope, program, pkg, service string) bool {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(displayName)), "tailscale-") {
		return false
	}
	unscoped := func(value string) bool {
		value = strings.ToLower(strings.TrimSpace(value))
		return value == "" || value == "any" || value == "*"
	}
	if !unscoped(program) || !unscoped(pkg) || !unscoped(service) {
		return false
	}
	if !unscoped(localPort) || !broadFirewallScope(scope) {
		return false
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	return protocol == "any" || protocol == "256"
}

func portSpecIncludes(startText, endText string, target int) bool {
	start, end, _, ok := parsePortSpec(startText)
	if endText != "" {
		start, end, _, ok = parsePortSpec(startText + "-" + endText)
	}
	return ok && target >= start && target <= end
}

func parsePortSpec(value string) (start, end int, exact, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(value), "-", 2)
	if len(parts) == 0 || len(parts) > 2 {
		return 0, 0, false, false
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil || start < 1 || start > 65535 {
		return 0, 0, false, false
	}
	end = start
	exact = true
	if len(parts) == 2 {
		end, err = strconv.Atoi(parts[1])
		if err != nil || end < start || end > 65535 {
			return 0, 0, false, false
		}
		exact = start == end
	}
	return start, end, exact, true
}

func broadFirewallScope(scope string) bool {
	for _, value := range strings.Split(scope, ",") {
		switch normalizeFirewallScope(value) {
		case "", "any", "*", "0.0.0.0/0", "::/0":
			return true
		}
	}
	return false
}
