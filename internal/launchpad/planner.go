package launchpad

import (
	"encoding/base64"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Planner struct{}

func (Planner) Build(profile Profile, snapshot Snapshot) Plan {
	plan := Plan{
		Timestamp:   time.Now().UTC(),
		ProfileName: profile.Name,
		Platform:    snapshot.Platform,
		ReadOnly:    true,
		HighestRisk: RiskLow,
	}
	if !profile.SSH.Enabled {
		plan.NoChanges = true
		plan.Warnings = append(plan.Warnings, "SSH is disabled in the selected profile; no SSH mutations were planned.")
		return plan
	}
	if profile.Transport.Mode == "tailnet" && !snapshot.Tailscale.Online {
		if !snapshot.Tailscale.Installed && profile.Transport.Install {
			action := installTailscaleAction(profile, snapshot)
			if len(action.Command) == 0 {
				plan.Blockers = append(plan.Blockers, action.Reason)
			} else {
				plan.Actions = append(plan.Actions, action)
				plan.Warnings = append(plan.Warnings, "This is a phased setup. After Tailscale is installed, sign in and run Check again; SSH and firewall changes are intentionally deferred.")
			}
		} else {
			plan.Blockers = append(plan.Blockers, "Tailnet exposure is selected, but Tailscale is not online. Install/sign in to Tailscale, then run Check again.")
		}
		plan.NoChanges = len(plan.Actions) == 0 && len(plan.Blockers) == 0
		return plan
	}
	if !snapshot.SSHClient.Installed || !snapshot.SSHServer.Installed {
		plan.Actions = append(plan.Actions, installSSHAction(profile, snapshot))
	}
	if snapshot.SSHPort != profile.SSH.Port || snapshot.SSHPort == 0 {
		plan.Actions = append(plan.Actions, configureSSHAction(profile, snapshot))
	}
	if len(profile.SSH.PublicKeys) > 0 && !snapshot.AuthorizedKeysChecked {
		plan.Blockers = append(plan.Blockers, "The target authorized_keys file could not be verified. Fix its permissions or run Check with sufficient access before applying.")
	} else if len(profile.SSH.PublicKeys) > 0 && !snapshot.AuthorizedKeysMatch {
		plan.Actions = append(plan.Actions, configureKeysAction(profile, snapshot))
	}
	if profile.Exposure.Mode != "none" && !firewallMatches(snapshot.Firewall, profile, snapshot) {
		scopes := exposureScopes(profile, snapshot)
		if len(snapshot.Firewall.UnresolvedBroadRules) > 0 {
			plan.Blockers = append(plan.Blockers, "Broad inbound firewall rules that cover multiple ports also expose SSH. Review them manually before Apply: "+strings.Join(snapshot.Firewall.UnresolvedBroadRules, ", "))
		} else if len(scopes) == 0 {
			plan.Blockers = append(plan.Blockers, "No safe source network could be detected for the requested exposure mode. Choose explicit CIDRs or connect the target to the intended network.")
		} else {
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
	for i := range plan.Actions {
		if snapshot.SessionTransport == "ssh" && isSelfCutOperation(plan.Actions[i].Operation) {
			plan.Actions[i].SelfCutRisk = true
			if plan.Actions[i].Risk == RiskLow || plan.Actions[i].Risk == RiskMedium {
				plan.Actions[i].Risk = RiskHigh
			}
			plan.SelfCutDetected = true
		}
		if riskRank(plan.Actions[i].Risk) > riskRank(plan.HighestRisk) {
			plan.HighestRisk = plan.Actions[i].Risk
		}
	}
	plan.NoChanges = len(plan.Actions) == 0 && len(plan.Blockers) == 0
	return plan
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

func installTailscaleAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("install-tailscale", "install_tailscale", "transport", RiskMedium, "Install Tailscale as the optional secure transport", "Tailnet mode is requested and Tailscale is missing.")
	a.RequiresElevation = true
	a.Reversible = false
	switch snapshot.Platform {
	case PlatformWindows:
		if profile.Download.Strategy == "offline" {
			a.Command = []string{profile.Download.OfflineBundle, "/quiet"}
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
	a := baseAction("configure-sshd", "configure_sshd", "ssh-config", RiskHigh, fmt.Sprintf("Set SSH port %d and key-oriented authentication", profile.SSH.Port), "The current listener does not match the profile.")
	a.RequiresElevation = true
	a.Reversible = true
	stamp := time.Now().UTC().Format("20060102T150405Z")
	block := fmt.Sprintf("# BEGIN SSH-LAUNCHPAD\nPort %d\nPubkeyAuthentication yes\nPasswordAuthentication %s\n# END SSH-LAUNCHPAD\n", profile.SSH.Port, yesNo(profile.SSH.PasswordAuthentication))
	encoded := base64.StdEncoding.EncodeToString([]byte(block))
	switch snapshot.Platform {
	case PlatformWindows:
		config := `C:\ProgramData\ssh\sshd_config`
		backup := config + ".ssh-launchpad-" + stamp + ".bak"
		wasRunning := psBool(snapshot.SSHService.Running)
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $p='%s'; $b='%s'; $wasRunning=%s; if(!(Test-Path $p)){Copy-Item "$env:WINDIR\System32\OpenSSH\sshd_config_default" $p}; Copy-Item $p $b -Force; try{$raw=Get-Content $p -Raw; $raw=[regex]::Replace($raw,'(?ms)^# BEGIN SSH-LAUNCHPAD\r?\n.*?^# END SSH-LAUNCHPAD\r?\n?',''); $block=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); [IO.File]::WriteAllText($p,($block+$raw.TrimStart()),[Text.ASCIIEncoding]::new()); & "$env:WINDIR\System32\OpenSSH\sshd.exe" -t -f $p; if($LASTEXITCODE -ne 0){throw 'sshd_config validation failed'}; if($wasRunning){Restart-Service sshd -ErrorAction Stop}}catch{Copy-Item $b $p -Force; if($wasRunning){Restart-Service sshd -ErrorAction SilentlyContinue}; throw}`, config, backup, wasRunning, encoded)
		a.Command = psCommand(script)
		rollback := fmt.Sprintf(`Copy-Item '%s' '%s' -Force`, backup, config)
		if snapshot.SSHService.Running {
			rollback += `; Restart-Service sshd`
		}
		a.RollbackCommand = psCommand(rollback)
	case PlatformLinux, PlatformWSL:
		path := "/etc/ssh/sshd_config"
		backup := path + "." + stamp + ".bak"
		wasRunning := shellBool(snapshot.SSHService.Running)
		service := shQuote(serviceName(profile, snapshot))
		script := fmt.Sprintf(`set -eu; path=%s; backup=%s; was_running=%s; service=%s; cp -p "$path" "$backup"; tmp="$(mktemp)"; trap 'rm -f "$tmp"' EXIT HUP INT TERM; printf %%s %s | base64 -d > "$tmp"; awk 'BEGIN{managed=0} /^# BEGIN SSH-LAUNCHPAD$/{managed=1;next} /^# END SSH-LAUNCHPAD$/{managed=0;next} !managed{print}' "$path" >> "$tmp"; cat "$tmp" > "$path"; if ! sshd -t; then cp -p "$backup" "$path"; exit 1; fi; if [ "$was_running" = true ] && ! systemctl restart "$service"; then cp -p "$backup" "$path"; systemctl restart "$service" || true; exit 1; fi`, shQuote(path), shQuote(backup), wasRunning, service, shQuote(encoded))
		a.Command = unixCommand(script)
		rollback := fmt.Sprintf("cp -p %s %s", shQuote(backup), shQuote(path))
		if snapshot.SSHService.Running {
			rollback += fmt.Sprintf("; systemctl restart %s", service)
		}
		a.RollbackCommand = unixCommand(rollback)
	case PlatformMacOS:
		path := "/etc/ssh/sshd_config"
		backup := path + "." + stamp + ".bak"
		wasRunning := shellBool(snapshot.SSHService.Running)
		a.Command = unixCommand(fmt.Sprintf(`set -eu; path=%s; backup=%s; was_running=%s; cp -p "$path" "$backup"; tmp="$(mktemp)"; trap 'rm -f "$tmp"' EXIT HUP INT TERM; printf %%s %s | base64 -D > "$tmp"; awk 'BEGIN{managed=0} /^# BEGIN SSH-LAUNCHPAD$/{managed=1;next} /^# END SSH-LAUNCHPAD$/{managed=0;next} !managed{print}' "$path" >> "$tmp"; cat "$tmp" > "$path"; if ! /usr/sbin/sshd -t; then cp -p "$backup" "$path"; exit 1; fi; if [ "$was_running" = true ] && ! launchctl kickstart -k system/com.openssh.sshd; then cp -p "$backup" "$path"; launchctl kickstart -k system/com.openssh.sshd || true; exit 1; fi`, shQuote(path), shQuote(backup), wasRunning, shQuote(encoded)))
		rollback := fmt.Sprintf("cp -p %s %s", shQuote(backup), shQuote(path))
		if snapshot.SSHService.Running {
			rollback += "; launchctl kickstart -k system/com.openssh.sshd"
		}
		a.RollbackCommand = unixCommand(rollback)
	}
	a.Params = map[string]string{"port": strconv.Itoa(profile.SSH.Port), "managedBlock": "SSH-LAUNCHPAD"}
	return a
}

func configureKeysAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("configure-authorized-keys", "configure_keys", "authentication", RiskHigh, "Merge the declared SSH public keys", "One or more declared controller public keys are not present. Existing keys are preserved and the file is backed up.")
	a.RequiresElevation = snapshot.Platform == PlatformWindows && snapshot.TargetUserIsAdmin
	a.Reversible = true
	content := strings.Join(profile.SSH.PublicKeys, "\n") + "\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	stamp := time.Now().UTC().Format("20060102T150405Z")
	switch snapshot.Platform {
	case PlatformWindows:
		path, _ := authorizedKeysPath(snapshot)
		backup := path + ".ssh-launchpad-" + stamp + ".bak"
		grantees := `@('*S-1-5-18:F','*S-1-5-32-544:F')`
		if !snapshot.TargetUserIsAdmin {
			grantees = `@('*S-1-5-18:F',('*'+[Security.Principal.WindowsIdentity]::GetCurrent().User.Value+':F'))`
		}
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $p='%s'; $b='%s'; $dir=Split-Path $p; New-Item -ItemType Directory -Path $dir -Force | Out-Null; $had=Test-Path $p; if($had){Copy-Item $p $b -Force}; try{$existing=if($had){@(Get-Content $p | ForEach-Object {$_.Trim()} | Where-Object {$_})}else{@()}; $wanted=([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')) -split '\r?\n' | ForEach-Object {$_.Trim()} | Where-Object {$_}); $merged=@($existing+$wanted | Select-Object -Unique); [IO.File]::WriteAllLines($p,$merged,[Text.ASCIIEncoding]::new()); & icacls.exe $p /inheritance:r | Out-Null; if($LASTEXITCODE -ne 0){throw 'failed to disable inherited ACLs'}; $grantees=%s; & icacls.exe $p /grant:r $grantees | Out-Null; if($LASTEXITCODE -ne 0){throw 'failed to set authorized_keys ACLs'}}catch{if(Test-Path $b){Copy-Item $b $p -Force}elseif(!$had){Remove-Item $p -Force -ErrorAction SilentlyContinue}; throw}`, path, backup, encoded, grantees)
		a.Command = psCommand(script)
		a.RollbackCommand = psCommand(fmt.Sprintf(`if(Test-Path '%s'){Copy-Item '%s' '%s' -Force}else{Remove-Item '%s' -Force -ErrorAction SilentlyContinue}`, backup, backup, path, path))
	default:
		backupSuffix := ".ssh-launchpad-" + stamp + ".bak"
		targetPrelude := `target_user="${SUDO_USER:-$(id -un)}"; if command -v getent >/dev/null 2>&1; then target_home="$(getent passwd "$target_user" | cut -d: -f6)"; elif command -v dscacheutil >/dev/null 2>&1; then target_home="$(dscacheutil -q user -a name "$target_user" | awk '/^dir:/{print $2; exit}')"; else target_home="$HOME"; fi; [ -n "$target_home" ]`
		script := fmt.Sprintf(`set -eu; %s; ssh_dir="$target_home/.ssh"; path="$ssh_dir/authorized_keys"; backup="$path%s"; mkdir -p "$ssh_dir"; chmod 700 "$ssh_dir"; if [ -f "$path" ]; then cp -p "$path" "$backup"; else : > "$path"; fi; tmp="$path.ssh-launchpad.tmp"; { cat "$path"; printf %%s %s | base64 -d; } | awk 'NF && !seen[$0]++' > "$tmp"; mv "$tmp" "$path"; chmod 600 "$path"; if [ "$(id -u)" -eq 0 ] && [ "$target_user" != root ]; then chown -R "$target_user" "$ssh_dir"; fi`, targetPrelude, backupSuffix, shQuote(encoded))
		a.Command = unixCommand(script)
		a.RollbackCommand = unixCommand(fmt.Sprintf(`set -eu; %s; path="$target_home/.ssh/authorized_keys"; backup="$path%s"; if [ -f "$backup" ]; then cp -p "$backup" "$path"; else rm -f "$path"; fi; if [ "$(id -u)" -eq 0 ] && [ "$target_user" != root ]; then chown "$target_user" "$path" 2>/dev/null || true; fi`, targetPrelude, backupSuffix))
	}
	a.Params = map[string]string{"keyCount": strconv.Itoa(len(profile.SSH.PublicKeys))}
	return a
}

func enableSSHAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("enable-sshd", "enable_sshd", "ssh-service", RiskMedium, "Enable and start the SSH service", "The SSH service is missing from the desired running state.")
	a.RequiresElevation = true
	a.Reversible = true
	switch snapshot.Platform {
	case PlatformWindows:
		a.Command = psCommand(`Set-Service sshd -StartupType Automatic; if((Get-Service sshd).Status -ne 'Running'){Start-Service sshd}`)
		a.RollbackCommand = psCommand(`Stop-Service sshd -ErrorAction SilentlyContinue`)
	case PlatformMacOS:
		a.Command = []string{"systemsetup", "-setremotelogin", "on"}
		a.RollbackCommand = []string{"systemsetup", "-setremotelogin", "off"}
	default:
		service := serviceName(profile, snapshot)
		a.Command = []string{"systemctl", "enable", "--now", service}
		a.RollbackCommand = []string{"systemctl", "disable", "--now", service}
	}
	return a
}

func configureFirewallAction(profile Profile, snapshot Snapshot, scopes []string) Action {
	a := baseAction("configure-firewall", "configure_firewall", "firewall", RiskHigh, fmt.Sprintf("Allow TCP %d from %s only", profile.SSH.Port, strings.Join(scopes, ", ")), "No port-and-scope-aware firewall rule matches the profile.")
	a.RequiresElevation = true
	a.Reversible = true
	name := fmt.Sprintf("SSH-Launchpad-TCP-%d", profile.SSH.Port)
	switch snapshot.Platform {
	case PlatformWindows:
		quotedScopes := "'" + strings.Join(scopes, "','") + "'"
		conflicts := psStringArray(snapshot.Firewall.ConflictingRules)
		backupName := fmt.Sprintf("firewall-%d-%s.json", profile.SSH.Port, time.Now().UTC().Format("20060102T150405Z"))
		a.Command = psCommand(fmt.Sprintf(`$ErrorActionPreference='Stop'; $backup=Join-Path $env:ProgramData 'SSH Launchpad\%s'; $conflicts=%s; $dynamic=Get-NetFirewallPortFilter -Protocol TCP -ErrorAction SilentlyContinue | Where-Object {[string]$_.LocalPort -eq '%d'} | ForEach-Object {Get-NetFirewallRule -AssociatedNetFirewallPortFilter $_ -ErrorAction SilentlyContinue} | Where-Object {$_.Name -ne '%s' -and $_.Enabled -eq 'True' -and $_.Direction -eq 'Inbound' -and $_.Action -eq 'Allow'} | Where-Object {$a=Get-NetFirewallAddressFilter -AssociatedNetFirewallRule $_ -ErrorAction SilentlyContinue; @($a.RemoteAddress | Where-Object {$_ -in @('Any','*','0.0.0.0/0','::/0')}).Count -gt 0} | Select-Object -ExpandProperty Name; $conflicts=@($conflicts+$dynamic | Select-Object -Unique); New-Item -ItemType Directory -Path (Split-Path $backup) -Force | Out-Null; ConvertTo-Json -InputObject @($conflicts) | Set-Content -LiteralPath $backup -Encoding UTF8; try{foreach($rule in $conflicts){Get-NetFirewallRule -Name $rule -ErrorAction SilentlyContinue | Set-NetFirewallRule -Enabled False}; Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule; New-NetFirewallRule -Name '%s' -DisplayName '%s' -Direction Inbound -Action Allow -Enabled True -Profile Any -Protocol TCP -LocalPort %d -RemoteAddress %s | Out-Null}catch{Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule; foreach($rule in $conflicts){Get-NetFirewallRule -Name $rule -ErrorAction SilentlyContinue | Set-NetFirewallRule -Enabled True}; Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue; throw}`, backupName, conflicts, profile.SSH.Port, name, name, name, name, profile.SSH.Port, quotedScopes, name))
		a.RollbackCommand = psCommand(fmt.Sprintf(`$backup=Join-Path $env:ProgramData 'SSH Launchpad\%s'; $conflicts=if(Test-Path -LiteralPath $backup){@(Get-Content -LiteralPath $backup -Raw | ConvertFrom-Json)}else{%s}; Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule; foreach($rule in $conflicts){Get-NetFirewallRule -Name $rule -ErrorAction SilentlyContinue | Set-NetFirewallRule -Enabled True}; Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue`, backupName, conflicts, name))
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
			a.Command = unixCommand(strings.Join(add, "; "))
			a.RollbackCommand = unixCommand(strings.Join(remove, "; "))
		default:
			var add, remove []string
			for _, scope := range scopes {
				args := "allow from " + shQuote(scope) + " to any port " + strconv.Itoa(profile.SSH.Port) + " proto tcp"
				add = append(add, "ufw "+args)
				remove = append(remove, "ufw --force delete "+args)
			}
			a.Command = unixCommand(strings.Join(add, "; "))
			a.RollbackCommand = unixCommand(strings.Join(remove, "; "))
		}
	}
	a.Params = map[string]string{"ruleName": name, "scopes": strings.Join(scopes, ",")}
	return a
}

func baseAction(id, operation, layer string, risk Risk, summary, reason string) Action {
	return Action{ID: id, Operation: operation, Layer: layer, Risk: risk, Summary: summary, Reason: reason, Mutating: true}
}

func firewallMatches(state FirewallState, profile Profile, snapshot Snapshot) bool {
	if state.BroadExposure || !containsInt(state.Ports, profile.SSH.Port) {
		return false
	}
	text := strings.ToLower(strings.Join(state.Scopes, " "))
	for _, scope := range exposureScopes(profile, snapshot) {
		if !strings.Contains(text, strings.ToLower(scope)) {
			return false
		}
	}
	return true
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

func psBool(value bool) string {
	if value {
		return "$true"
	}
	return "$false"
}

func shellBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
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
	case "configure_sshd", "enable_sshd", "configure_firewall", "install_tailscale":
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
