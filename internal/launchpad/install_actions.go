package launchpad

import ()

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
