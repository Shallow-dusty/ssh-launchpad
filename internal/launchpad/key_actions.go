package launchpad

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func keyCommands(profile Profile, snapshot Snapshot) ([]string, []string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Join(profile.SSH.PublicKeys, "\n") + "\n"))
	suffix := ".ssh-launchpad-" + backupStamp() + ".bak"
	if snapshot.Platform == PlatformWindows {
		path, _ := authorizedKeysPath(snapshot)
		p := strings.ReplaceAll(path, "'", "''")
		b := p + suffix
		prelude := fmt.Sprintf(`$ErrorActionPreference='Stop'; $p='%s'; $b='%s'; `, p, b)
		grantees := `@('*S-1-5-18:F','*S-1-5-32-544:F')`
		if !snapshot.TargetUserIsAdmin {
			grantees = `@('*S-1-5-18:F',('*'+[Security.Principal.WindowsIdentity]::GetCurrent().User.Value+':F'))`
		}
		apply := prelude + fmt.Sprintf(`if((Test-Path $b) -or (Test-Path ($b+'.ready'))){throw 'Key backup exists'}; $dir=Split-Path $p; New-Item -ItemType Directory -Path $dir -Force | Out-Null; foreach($f in @($dir,$p)){if((Test-Path $f) -and ((Get-Item -LiteralPath $f -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Key path must not be a reparse point'}}; $had=Test-Path $p; if($had){Get-Acl -LiteralPath $p | Export-Clixml -LiteralPath ($b+'.acl'); Copy-Item -LiteralPath $p -Destination ($b+'.tmp'); Move-Item -LiteralPath ($b+'.tmp') -Destination $b}; [IO.File]::WriteAllText(($b+'.ready'),[string]$had); $existing=[object[]]@(if($had){Get-Content -LiteralPath $p | ForEach-Object {$_.Trim()} | Where-Object {$_}}); $wanted=[object[]]@([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('%s')) -split '\r?\n' | Where-Object {$_}); $merged=[object[]]@($existing+$wanted | Select-Object -Unique); [IO.File]::WriteAllLines($p,$merged,[Text.ASCIIEncoding]::new()); & icacls.exe $p /inheritance:r | Out-Null; if($LASTEXITCODE -ne 0){throw 'failed to disable inherited ACLs'}; & icacls.exe $p /grant:r %s | Out-Null; if($LASTEXITCODE -ne 0){throw 'failed to set authorized_keys ACLs'}`, encoded, grantees)
		rollback := prelude + `if(Test-Path -LiteralPath ($b+'.ready')){if((Get-Content -LiteralPath ($b+'.ready') -Raw) -eq 'True'){if(!(Test-Path -LiteralPath $b)){throw 'Original key backup is missing'}; Copy-Item -LiteralPath $b -Destination $p -Force; Set-Acl -LiteralPath $p -AclObject (Import-Clixml -LiteralPath ($b+'.acl'))}else{Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue}}`
		return psCommand(apply), psCommand(rollback)
	}
	target := `target_user="${SUDO_USER:-$(id -un)}"; if command -v getent >/dev/null 2>&1; then target_home="$(getent passwd "$target_user" | cut -d: -f6)"; elif command -v dscacheutil >/dev/null 2>&1; then target_home="$(dscacheutil -q user -a name "$target_user" | awk '/^dir:/{print $2; exit}')"; else target_home="$HOME"; fi; [ -n "$target_home" ]; `
	prelude := `set -eu; ` + target + `ssh_dir="$target_home/.ssh"; path="$ssh_dir/authorized_keys"; backup="$path` + suffix + `"; if [ -L "$ssh_dir" ] || [ -L "$path" ] || [ -L "$backup" ] || [ -L "$backup.created" ]; then echo 'key paths must not be symlinks' >&2; exit 1; fi; `
	decode := "-d"
	if snapshot.Platform == PlatformMacOS {
		decode = "-D"
	}
	apply := prelude + `if [ -e "$ssh_dir" ] && [ ! -d "$ssh_dir" ]; then exit 1; fi; if [ ! -d "$ssh_dir" ]; then mkdir -m 700 "$ssh_dir"; if [ "$(id -u)" -eq 0 ]; then chown "$target_user" "$ssh_dir"; fi; fi; if [ -e "$path" ] && [ ! -f "$path" ]; then exit 1; fi; if [ -e "$backup" ] || [ -e "$backup.created" ] || [ -e "$backup.tmp" ] || [ -L "$backup.tmp" ]; then echo 'key backup already exists' >&2; exit 1; fi; if [ -f "$path" ]; then cp -p "$path" "$backup.tmp"; mv "$backup.tmp" "$backup"; else (set -C; : > "$backup.created"); fi; tmp="$(mktemp "$ssh_dir/.ssh-launchpad.tmp.XXXXXX")"; trap 'rm -f "$tmp"' EXIT HUP INT TERM; if [ -f "$path" ]; then cat "$path" > "$tmp"; fi; printf '\n' >> "$tmp"; printf %s ` + shQuote(encoded) + ` | base64 ` + decode + ` >> "$tmp"; chmod 600 "$tmp"; if [ "$(id -u)" -eq 0 ]; then chown "$target_user" "$tmp"; fi; mv "$tmp" "$path"`
	rollback := prelude + `if [ -e "$path" ] && [ ! -f "$path" ]; then exit 1; fi; if [ -f "$backup" ]; then cp -p "$backup" "$path"; elif [ -f "$backup.created" ]; then rm -f "$path"; fi`
	return unixCommand(apply), unixCommand(rollback)
}
