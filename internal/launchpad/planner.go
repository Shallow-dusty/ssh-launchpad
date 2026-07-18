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
			plan.Actions = append(plan.Actions, installTailscaleAction(profile, snapshot))
		} else {
			plan.Warnings = append(plan.Warnings, "Tailnet exposure is selected, but Tailscale is not online. Apply will not open SSH exposure until transport verification succeeds.")
		}
	}
	if !snapshot.SSHClient.Installed || !snapshot.SSHServer.Installed {
		plan.Actions = append(plan.Actions, installSSHAction(profile, snapshot))
	}
	if snapshot.SSHPort != profile.SSH.Port || snapshot.SSHPort == 0 {
		plan.Actions = append(plan.Actions, configureSSHAction(profile, snapshot))
	}
	if len(profile.SSH.PublicKeys) > 0 {
		plan.Actions = append(plan.Actions, configureKeysAction(profile, snapshot))
	}
	if !snapshot.SSHService.Running || snapshot.SSHService.StartPolicy == "disabled" {
		plan.Actions = append(plan.Actions, enableSSHAction(profile, snapshot))
	}
	if profile.Exposure.Mode != "none" && !firewallMatches(snapshot.Firewall, profile) {
		if profile.Exposure.Mode == "tailnet" && !snapshot.Tailscale.Online {
			plan.Warnings = append(plan.Warnings, "Firewall opening is deferred because the requested Tailnet transport is not verified online.")
		} else {
			plan.Actions = append(plan.Actions, configureFirewallAction(profile, snapshot))
		}
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
	plan.NoChanges = len(plan.Actions) == 0
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
		} else {
			a.Command = []string{"winget.exe", "install", "--id", "Tailscale.Tailscale", "--exact", "--accept-package-agreements", "--accept-source-agreements"}
		}
	case PlatformMacOS:
		a.Command = []string{"brew", "install", "--cask", "tailscale-app"}
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
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $p='%s'; $b='%s'; if(!(Test-Path $p)){Copy-Item "$env:WINDIR\System32\OpenSSH\sshd_config_default" $p}; Copy-Item $p $b -Force; $raw=Get-Content $p -Raw; $raw=[regex]::Replace($raw,'(?ms)^# BEGIN SSH-LAUNCHPAD\r?\n.*?^# END SSH-LAUNCHPAD\r?\n?',''); $block=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); [IO.File]::WriteAllText($p,($raw.TrimEnd()+[Environment]::NewLine+$block),[Text.ASCIIEncoding]::new()); & "$env:WINDIR\System32\OpenSSH\sshd.exe" -t -f $p; if($LASTEXITCODE -ne 0){Copy-Item $b $p -Force; throw 'sshd_config validation failed; backup restored'}`, config, backup, encoded)
		a.Command = psCommand(script)
		a.RollbackCommand = psCommand(fmt.Sprintf(`Copy-Item '%s' '%s' -Force; Restart-Service sshd`, backup, config))
	case PlatformLinux, PlatformWSL:
		path := "/etc/ssh/sshd_config.d/90-ssh-launchpad.conf"
		backup := path + "." + stamp + ".bak"
		script := fmt.Sprintf("set -eu; if [ -f %s ]; then cp -p %s %s; fi; printf %%s %s | base64 -d > %s; sshd -t", shQuote(path), shQuote(path), shQuote(backup), shQuote(encoded), shQuote(path))
		a.Command = unixCommand(script)
		a.RollbackCommand = unixCommand(fmt.Sprintf("if [ -f %s ]; then cp -p %s %s; else rm -f %s; fi; systemctl reload %s", shQuote(backup), shQuote(backup), shQuote(path), shQuote(path), shQuote(serviceName(profile, snapshot))))
	case PlatformMacOS:
		path := "/etc/ssh/sshd_config.d/90-ssh-launchpad.conf"
		backup := path + "." + stamp + ".bak"
		a.Command = unixCommand(fmt.Sprintf("set -eu; if [ -f %s ]; then cp -p %s %s; fi; printf %%s %s | base64 -D > %s; /usr/sbin/sshd -t", shQuote(path), shQuote(path), shQuote(backup), shQuote(encoded), shQuote(path)))
		a.RollbackCommand = unixCommand(fmt.Sprintf("if [ -f %s ]; then cp -p %s %s; else rm -f %s; fi", shQuote(backup), shQuote(backup), shQuote(path), shQuote(path)))
	}
	a.Params = map[string]string{"port": strconv.Itoa(profile.SSH.Port), "managedBlock": "SSH-LAUNCHPAD"}
	return a
}

func configureKeysAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("configure-authorized-keys", "configure_keys", "authentication", RiskHigh, "Install the declared SSH public keys", "The profile declares controller public keys. Existing files are backed up before replacement.")
	a.RequiresElevation = snapshot.Platform == PlatformWindows
	a.Reversible = true
	content := strings.Join(profile.SSH.PublicKeys, "\n") + "\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	stamp := time.Now().UTC().Format("20060102T150405Z")
	switch snapshot.Platform {
	case PlatformWindows:
		path := `C:\ProgramData\ssh\administrators_authorized_keys`
		backup := path + ".ssh-launchpad-" + stamp + ".bak"
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $p='%s'; $b='%s'; if(Test-Path $p){Copy-Item $p $b -Force}; $v=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); [IO.File]::WriteAllText($p,$v,[Text.ASCIIEncoding]::new()); & icacls.exe $p /inheritance:r | Out-Null; & icacls.exe $p /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null`, path, backup, encoded)
		a.Command = psCommand(script)
		a.RollbackCommand = psCommand(fmt.Sprintf(`if(Test-Path '%s'){Copy-Item '%s' '%s' -Force}else{Remove-Item '%s' -Force -ErrorAction SilentlyContinue}`, backup, backup, path, path))
	default:
		path := "~/.ssh/authorized_keys"
		backup := path + ".ssh-launchpad-" + stamp + ".bak"
		a.Command = unixCommand(fmt.Sprintf("set -eu; mkdir -p ~/.ssh; chmod 700 ~/.ssh; if [ -f %s ]; then cp -p %s %s; fi; printf %%s %s | base64 -d > %s; chmod 600 %s", path, path, backup, shQuote(encoded), path, path))
		a.RollbackCommand = unixCommand(fmt.Sprintf("if [ -f %s ]; then cp -p %s %s; else rm -f %s; fi", backup, backup, path, path))
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

func configureFirewallAction(profile Profile, snapshot Snapshot) Action {
	scopes := exposureScopes(profile)
	a := baseAction("configure-firewall", "configure_firewall", "firewall", RiskHigh, fmt.Sprintf("Allow TCP %d from %s only", profile.SSH.Port, strings.Join(scopes, ", ")), "No port-and-scope-aware firewall rule matches the profile.")
	a.RequiresElevation = true
	a.Reversible = true
	name := fmt.Sprintf("SSH-Launchpad-TCP-%d", profile.SSH.Port)
	switch snapshot.Platform {
	case PlatformWindows:
		quotedScopes := "'" + strings.Join(scopes, "','") + "'"
		a.Command = psCommand(fmt.Sprintf(`Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule; New-NetFirewallRule -Name '%s' -DisplayName '%s' -Direction Inbound -Action Allow -Enabled True -Profile Any -Protocol TCP -LocalPort %d -RemoteAddress %s | Out-Null`, name, name, name, profile.SSH.Port, quotedScopes))
		a.RollbackCommand = psCommand(fmt.Sprintf(`Get-NetFirewallRule -Name '%s' -ErrorAction SilentlyContinue | Remove-NetFirewallRule`, name))
	case PlatformMacOS:
		a.Command = nil
		a.Risk = RiskMedium
		a.Summary = "Review the macOS application firewall or upstream packet filter"
		a.Reason = "The macOS application firewall does not provide a portable port-and-CIDR rule interface. SSH Launchpad will not claim a rule it cannot verify."
		a.Reversible = false
	default:
		switch snapshot.Firewall.Provider {
		case "firewall-cmd":
			rich := fmt.Sprintf(`rule family="ipv4" source address="%s" port port="%d" protocol="tcp" accept`, scopes[0], profile.SSH.Port)
			a.Command = []string{"firewall-cmd", "--permanent", "--add-rich-rule", rich}
			a.RollbackCommand = []string{"firewall-cmd", "--permanent", "--remove-rich-rule", rich}
		default:
			rule := fmt.Sprintf("%d/tcp", profile.SSH.Port)
			a.Command = []string{"ufw", "allow", "from", scopes[0], "to", "any", "port", strconv.Itoa(profile.SSH.Port), "proto", "tcp"}
			a.RollbackCommand = []string{"ufw", "delete", "allow", rule}
		}
	}
	a.Params = map[string]string{"ruleName": name, "scopes": strings.Join(scopes, ",")}
	return a
}

func baseAction(id, operation, layer string, risk Risk, summary, reason string) Action {
	return Action{ID: id, Operation: operation, Layer: layer, Risk: risk, Summary: summary, Reason: reason, Mutating: true}
}

func firewallMatches(state FirewallState, profile Profile) bool {
	if !containsInt(state.Ports, profile.SSH.Port) {
		return false
	}
	text := strings.ToLower(strings.Join(state.Scopes, " "))
	for _, scope := range exposureScopes(profile) {
		if !strings.Contains(text, strings.ToLower(scope)) {
			return false
		}
	}
	return true
}

func exposureScopes(profile Profile) []string {
	switch profile.Exposure.Mode {
	case "tailnet":
		return []string{"100.64.0.0/10", "fd7a:115c:a1e0::/48"}
	case "lan":
		return []string{"LocalSubnet"}
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
