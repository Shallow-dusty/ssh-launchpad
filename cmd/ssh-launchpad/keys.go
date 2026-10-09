package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

type discoveredPublicKey struct {
	label string
	value string
}

func discoverPublicKeys() []discoveredPublicKey {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
	var keys []discoveredPublicKey
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil && launchpad.ValidatePublicKey(string(data)) == nil {
			keys = append(keys, discoveredPublicKey{label: filepath.Base(path), value: strings.TrimSpace(string(data))})
		}
	}
	return keys
}

func generatePublicKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(directory, launchpad.ControllerKeyBaseName)
	if _, err := os.Stat(path); err == nil {
		return "", errors.New(tr("privateExists"))
	}
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", path, "-N", "", "-C", "ssh-launchpad-controller")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s: %s", tr("keygenFailed"), strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(path + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), launchpad.ValidatePublicKey(string(data))
}
