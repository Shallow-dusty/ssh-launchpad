//go:build windows

package launchpad

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func targetUserHome() (string, error) {
	return os.UserHomeDir()
}

func targetUserIdentity() (string, bool) {
	name := os.Getenv("USERNAME")
	sid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return name, false
	}
	groups, err := windows.GetCurrentProcessToken().GetTokenGroups()
	if err != nil {
		return name, false
	}
	for _, group := range groups.AllGroups() {
		if group.Sid != nil && group.Sid.Equals(sid) {
			return name, true
		}
	}
	return name, false
}

func authorizedKeysPath(snapshot Snapshot) (string, error) {
	if snapshot.SSHAuthorizedKeysFile != "" {
		return snapshot.SSHAuthorizedKeysFile, nil
	}
	if snapshot.TargetUserIsAdmin {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return filepath.Join(programData, "ssh", "administrators_authorized_keys"), nil
	}
	home, err := targetUserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "authorized_keys"), nil
}
