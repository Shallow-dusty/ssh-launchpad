package launchpad

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func configCommands(profile Profile, snapshot Snapshot) ([]string, []string) {
	path := sshConfigPath(snapshot.Platform)
	backup := path + ".ssh-launchpad-" + backupStamp() + ".bak"
	block := fmt.Sprintf("# BEGIN SSH-LAUNCHPAD\nPort %d\nPubkeyAuthentication yes\nPasswordAuthentication %s\nKbdInteractiveAuthentication no\nChallengeResponseAuthentication no\n# END SSH-LAUNCHPAD\n", profile.SSH.Port, yesNo(profile.SSH.PasswordAuthentication))
	encoded := base64.StdEncoding.EncodeToString([]byte(block))
	if snapshot.Platform == PlatformWindows {
		prelude := fmt.Sprintf(`$ErrorActionPreference='Stop'; $p='%s'; $b='%s'; `, strings.ReplaceAll(path, "'", "''"), strings.ReplaceAll(backup, "'", "''"))
		apply := prelude + fmt.Sprintf(`if((Test-Path $b) -or (Test-Path ($b+'.created')) -or (Test-Path ($b+'.tmp'))){throw 'SSH config backup already exists'}; $had=Test-Path -LiteralPath $p; if($had){Copy-Item -LiteralPath $p -Destination ($b+'.tmp'); Move-Item -LiteralPath ($b+'.tmp') -Destination $b; $raw=Get-Content -LiteralPath $p -Raw}else{$raw=Get-Content "$env:WINDIR\System32\OpenSSH\sshd_config_default" -Raw; [IO.File]::WriteAllText(($b+'.created'),'created')}; $raw=[regex]::Replace($raw,'(?ms)^# BEGIN SSH-LAUNCHPAD\r?\n.*?^# END SSH-LAUNCHPAD\r?\n?',''); $raw=[regex]::Replace($raw,'(?im)^\s*Port(?:\s+|=)[^\r\n]*\r?\n?',''); $block=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')); [IO.File]::WriteAllText($p,($block+$raw.TrimStart()),[Text.UTF8Encoding]::new($false)); & "$env:WINDIR\System32\OpenSSH\sshd.exe" -t -f $p; if($LASTEXITCODE -ne 0){throw 'sshd_config validation failed'}; `, encoded)
		rollback := prelude + `$restored=$false; if(Test-Path -LiteralPath $b){Copy-Item -LiteralPath $b -Destination $p -Force; $restored=$true}elseif(Test-Path -LiteralPath ($b+'.created')){Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue}; `
		if snapshot.SSHService.Running {
			apply += `Restart-Service sshd -ErrorAction Stop`
			rollback += `if($restored){Restart-Service sshd -ErrorAction Stop}`
		}
		return psCommand(apply), psCommand(rollback)
	}
	prelude := fmt.Sprintf(`set -eu; path=%s; backup=%s; `, shQuote(path), shQuote(backup))
	decode := "-d"
	validate := "sshd -t"
	restart := "systemctl restart " + shQuote(serviceName(profile, snapshot))
	if snapshot.Platform == PlatformMacOS {
		decode = "-D"
		validate = "/usr/sbin/sshd -t"
		restart = "launchctl kickstart -k system/com.openssh.sshd"
	}
	apply := prelude + `if [ -L "$path" ] || [ ! -f "$path" ] || [ -e "$backup" ] || [ -L "$backup" ] || [ -e "$backup.tmp" ] || [ -L "$backup.tmp" ]; then echo 'unsupported config or existing backup' >&2; exit 1; fi; cp -p "$path" "$backup.tmp"; mv "$backup.tmp" "$backup"; tmp="$(mktemp)"; trap 'rm -f "$tmp"' EXIT HUP INT TERM; printf %s ` + shQuote(encoded) + ` | base64 ` + decode + ` > "$tmp"; awk 'BEGIN{managed=0} /^# BEGIN SSH-LAUNCHPAD$/{managed=1;next} /^# END SSH-LAUNCHPAD$/{managed=0;next} tolower($1) ~ /^port(=|$)/{next} !managed{print}' "$path" >> "$tmp"; cat "$tmp" > "$path"; ` + validate
	rollback := prelude + `if [ -L "$path" ] || [ -L "$backup" ]; then exit 1; fi; if [ -f "$backup" ]; then cp -p "$backup" "$path"; `
	if snapshot.SSHService.Running {
		apply += "; " + restart
		rollback += restart + "; "
	}
	rollback += "fi"
	return unixCommand(apply), unixCommand(rollback)
}
