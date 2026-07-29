//go:build !windows

package launchpad

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGeneratedUnixCommandSyntaxAndShellCheck(t *testing.T) {
	for _, platform := range []Platform{PlatformLinux, PlatformMacOS} {
		for _, action := range generatedSyntaxTestActions(t, platform) {
			for _, command := range [][]string{action.Command, action.RollbackCommand} {
				if len(command) < 3 || filepath.Base(command[0]) != "sh" || command[1] != "-c" {
					continue
				}
				script := command[2]
				if output, err := exec.Command("sh", "-n", "-c", script).CombinedOutput(); err != nil {
					t.Fatalf("%s %s shell syntax failed: %v\n%s", platform, action.ID, err, output)
				}
				if shellcheck, err := exec.LookPath("shellcheck"); err == nil {
					cmd := exec.Command(shellcheck, "--shell=sh", "-")
					cmd.Stdin = bytes.NewBufferString(script)
					if output, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("%s %s shellcheck failed: %v\n%s", platform, action.ID, err, output)
					}
				}
			}
		}
	}
}
