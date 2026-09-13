package launchpad

import (
	"fmt"
	"strings"
)

func windowsFirewallCommands(name string, port int, scopes, conflicts []string) ([]string, []string) {
	backupName := fmt.Sprintf("firewall-%d-%s.json", port, backupStamp())
	prelude := fmt.Sprintf(`$ErrorActionPreference='Stop'; $name='%s'; $backup=Join-Path $env:ProgramData 'SSH Launchpad\%s'; `, name, backupName)
	// Existing managed rules are changed in place: only RemoteAddress is touched,
	// so all other rule/filter attributes survive both Apply and recovery.
	apply := prelude + fmt.Sprintf(`if(Test-Path -LiteralPath $backup){throw 'Firewall backup already exists; review the previous journal first'}; $existing=Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue; if(@($existing).Count -gt 1){throw 'Ambiguous managed firewall rule'}; $old=@(); if($existing){$pf=Get-NetFirewallPortFilter -AssociatedNetFirewallRule $existing -ErrorAction Stop; if($existing.Direction -ne 'Inbound' -or $existing.Action -ne 'Allow' -or $existing.Enabled -ne 'True' -or $pf.Protocol -notin @('TCP','6') -or [string]$pf.LocalPort -ne '%d'){throw 'Existing managed rule has unsupported attributes; review it manually'}; $old=@((Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $existing -ErrorAction Stop).RemoteAddress)}; $saved=@(); foreach($n in %s){if($n -eq $name){continue}; $r=Get-NetFirewallRule -Name $n -ErrorAction Stop; $saved+=@{name=$n;enabled=[string]$r.Enabled}}; $data=@{had=[bool]$existing;addresses=$old;conflicts=@($saved)}; New-Item -ItemType Directory -Path (Split-Path $backup) -Force | Out-Null; $data | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath ($backup+'.tmp') -Encoding UTF8; Move-Item -LiteralPath ($backup+'.tmp') -Destination $backup; foreach($r in $saved){Set-NetFirewallRule -Name $r.name -Enabled False -ErrorAction Stop}; if($existing){Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $existing -ErrorAction Stop | Set-NetFirewallAddressFilter -RemoteAddress %s -ErrorAction Stop}else{New-NetFirewallRule -Name $name -DisplayName $name -Direction Inbound -Action Allow -Enabled True -Profile Any -Protocol TCP -LocalPort %d -RemoteAddress %s -ErrorAction Stop | Out-Null}`, port, psStringArray(conflicts), "'"+strings.Join(scopes, "','")+"'", port, "'"+strings.Join(scopes, "','")+"'")
	rollback := prelude + `if(Test-Path -LiteralPath $backup){$data=Get-Content -LiteralPath $backup -Raw | ConvertFrom-Json; if($data.had){$r=Get-NetFirewallRule -Name $name -ErrorAction Stop; Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $r -ErrorAction Stop | Set-NetFirewallAddressFilter -RemoteAddress @($data.addresses) -ErrorAction Stop}else{Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction Stop}; foreach($r in @($data.conflicts)){Set-NetFirewallRule -Name $r.name -Enabled $r.enabled -ErrorAction Stop}}`
	return psCommand(apply), psCommand(rollback)
}
