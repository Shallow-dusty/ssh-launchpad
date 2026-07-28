//go:build !windows

package launchpad

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func targetUserHome() (string, error) {
	if os.Geteuid() == 0 {
		if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" && sudoUser != "root" {
			account, err := user.Lookup(sudoUser)
			if err == nil && account.HomeDir != "" {
				return account.HomeDir, nil
			}
		}
	}
	return os.UserHomeDir()
}

func targetUserIdentity() (string, bool) {
	name := strings.TrimSpace(os.Getenv("SUDO_USER"))
	if name == "" {
		if account, err := user.Current(); err == nil {
			name = account.Username
		}
	}
	return name, os.Geteuid() == 0
}

func authorizedKeysPath(snapshot Snapshot) (string, error) {
	if snapshot.SSHAuthorizedKeysFile != "" {
		return snapshot.SSHAuthorizedKeysFile, nil
	}
	home, err := targetUserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "authorized_keys"), nil
}
