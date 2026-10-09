package launchpad

import (
	"fmt"
	"strings"
)

func enableSSHAction(profile Profile, snapshot Snapshot) Action {
	a := baseAction("enable-sshd", "enable_sshd", "ssh-service", RiskMedium, "Enable and start the SSH service", "The SSH service is missing from the desired running state.")
	a.RequiresElevation = true
	a.Reversible = true
	switch snapshot.Platform {
	case PlatformWindows:
		// A fresh capability install ships no sshd_config and no host keys; the
		// official first-run bootstrap is starting sshd once. Generate the host
		// keys explicitly (restricted ACLs) so Enable and Start cannot fail on a
		// host whose policy file this run just created from the stock template.
		a.Command = psCommand(`$ErrorActionPreference='Stop'; $d=Join-Path $env:ProgramData 'ssh'; if(-not (Test-Path -LiteralPath (Join-Path $d 'ssh_host_ed25519_key'))){ if(-not (Test-Path -LiteralPath $d)){New-Item -ItemType Directory -Force -Path $d | Out-Null}; & "$env:WINDIR\System32\OpenSSH\ssh-keygen.exe" -A; foreach($f in Get-ChildItem -Path (Join-Path $d 'ssh_host_*_key')){ $acl=New-Object System.Security.AccessControl.FileSecurity; $acl.SetAccessRuleProtection($true,$false); $acl.AddAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule('NT AUTHORITY\SYSTEM','FullControl','Allow'))); $acl.AddAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule('BUILTIN\Administrators','FullControl','Allow'))); Set-Acl -LiteralPath $f.FullName -AclObject $acl; & icacls.exe $f.FullName /setowner '*S-1-5-18' | Out-Null } }; Set-Service sshd -StartupType Automatic; if((Get-Service sshd).Status -ne 'Running'){Start-Service sshd}`)
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

func serviceName(profile Profile, snapshot Snapshot) string {
	if profile.Advanced.LinuxSSHService != "" && profile.Advanced.LinuxSSHService != "auto" {
		return profile.Advanced.LinuxSSHService
	}
	if snapshot.SSHService.Name != "" && snapshot.SSHService.Name != "auto" {
		return snapshot.SSHService.Name
	}
	return "sshd"
}
