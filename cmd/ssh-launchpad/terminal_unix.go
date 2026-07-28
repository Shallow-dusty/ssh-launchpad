//go:build !windows

package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func configureTerminal() func() { return func() {} }

func systemLanguage() language {
	locale := strings.ToLower(os.Getenv("LC_ALL") + " " + os.Getenv("LANG"))
	if strings.Contains(locale, "zh") && (strings.Contains(locale, "utf-8") || strings.Contains(locale, "utf8")) {
		return langZH
	}
	return langEN
}

func elevateAndApply(profile launchpad.Profile, options launchpad.ApplyOptions, lang language) (bool, int, error) {
	return executeElevatedRequest(profile, options, lang, false)
}

func invokeElevated(executable, requestPath, digest string) error {
	// #nosec G702 -- executable comes from os.Executable, requestPath is a
	// freshly created protocol file, and every value is passed as a separate
	// argv element rather than through a shell.
	cmd := exec.Command("sudo", executable, "__elevated-apply", "--request", requestPath, "--sha256", digest)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
