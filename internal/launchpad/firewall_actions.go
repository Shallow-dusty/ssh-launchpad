package launchpad

import (
	"fmt"
	"strconv"
	"strings"
)

func windowsFirewallCommands(name string, port int, scopes, conflicts []string) ([]string, []string) {
	backupName := fmt.Sprintf("firewall-%d-%s.json", port, backupStamp())
	prelude := fmt.Sprintf(`$ErrorActionPreference='Stop'; $name='%s'; $backup=Join-Path $env:ProgramData 'SSH Launchpad\%s'; `, name, backupName)
	// Existing managed rules are changed in place: only RemoteAddress is touched,
	// so all other rule/filter attributes survive both Apply and recovery.
	apply := prelude + fmt.Sprintf(`if(Test-Path -LiteralPath $backup){throw 'Firewall backup already exists; review the previous journal first'}; $existing=Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue; if(@($existing).Count -gt 1){throw 'Ambiguous managed firewall rule'}; $old=@(); if($existing){$pf=Get-NetFirewallPortFilter -AssociatedNetFirewallRule $existing -ErrorAction Stop; if($existing.Direction -ne 'Inbound' -or $existing.Action -ne 'Allow' -or $existing.Enabled -ne 'True' -or $pf.Protocol -notin @('TCP','6') -or [string]$pf.LocalPort -ne '%d'){throw 'Existing managed rule has unsupported attributes; review it manually'}; $old=@((Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $existing -ErrorAction Stop).RemoteAddress)}; $saved=@(); foreach($n in %s){if($n -eq $name){continue}; $r=Get-NetFirewallRule -Name $n -ErrorAction Stop; $saved+=@{name=$n;enabled=[string]$r.Enabled}}; $data=@{had=[bool]$existing;addresses=$old;conflicts=@($saved)}; New-Item -ItemType Directory -Path (Split-Path $backup) -Force | Out-Null; $data | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath ($backup+'.tmp') -Encoding UTF8; Move-Item -LiteralPath ($backup+'.tmp') -Destination $backup; foreach($r in $saved){Set-NetFirewallRule -Name $r.name -Enabled False -ErrorAction Stop}; if($existing){Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $existing -ErrorAction Stop | Set-NetFirewallAddressFilter -RemoteAddress %s -ErrorAction Stop}else{New-NetFirewallRule -Name $name -DisplayName $name -Direction Inbound -Action Allow -Enabled True -Profile Any -Protocol TCP -LocalPort %d -RemoteAddress %s -ErrorAction Stop | Out-Null}`, port, psStringArray(conflicts), "'"+strings.Join(scopes, "','")+"'", port, "'"+strings.Join(scopes, "','")+"'")
	// Recovery must stay idempotent: a rule that another rollback already
	// restored or removed is reported, not turned into a failed recovery.
	rollback := prelude + `if(Test-Path -LiteralPath $backup){$data=Get-Content -LiteralPath $backup -Raw | ConvertFrom-Json; $target=Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue; if($data.had){if($target){Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $target -ErrorAction Stop | Set-NetFirewallAddressFilter -RemoteAddress @($data.addresses) -ErrorAction Stop}else{Write-Output 'note: the managed rule was already absent; no previous scope needed restoring'}}else{if($target){$target | Remove-NetFirewallRule -ErrorAction Stop}}; foreach($r in @($data.conflicts)){Get-NetFirewallRule -Name $r.name -ErrorAction SilentlyContinue | Set-NetFirewallRule -Enabled $r.enabled -ErrorAction Stop}}; foreach($f in @($backup,($backup+'.tmp'))){ if(Test-Path -LiteralPath $f){ Remove-Item -LiteralPath $f -Force -ErrorAction Stop } }`
	return psCommand(apply), psCommand(rollback)
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
	name := managedFirewallRuleName(profile.SSH.Port)
	switch snapshot.Platform {
	case PlatformWindows:
		conflicts := append([]string(nil), snapshot.Firewall.ConflictingRules...)
		for _, stale := range snapshot.Firewall.StaleManagedRules {
			if stale != name {
				conflicts = append(conflicts, stale)
			}
		}
		a.Command, a.RollbackCommand = windowsFirewallCommands(name, profile.SSH.Port, scopes, conflicts)
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
