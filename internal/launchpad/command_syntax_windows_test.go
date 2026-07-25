//go:build windows

package launchpad

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGeneratedWindowsPowerShellParses(t *testing.T) {
	for _, action := range generatedSyntaxTestActions(t, PlatformWindows) {
		for _, command := range [][]string{action.Command, action.RollbackCommand} {
			script, ok := powerShellScript(command)
			if !ok {
				continue
			}
			cmd := exec.Command(
				"powershell.exe",
				"-NoProfile",
				"-NonInteractive",
				"-Command",
				`[void][scriptblock]::Create($env:SSH_LAUNCHPAD_SCRIPT)`,
			)
			cmd.Env = append(os.Environ(), "SSH_LAUNCHPAD_SCRIPT="+script)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s PowerShell syntax failed: %v\n%s", action.ID, err, output)
			}
		}
	}
}

func powerShellScript(command []string) (string, bool) {
	if len(command) < 2 || !strings.Contains(strings.ToLower(command[0]), "powershell") {
		return "", false
	}
	for index, argument := range command {
		if strings.EqualFold(argument, "-Command") && index+1 < len(command) {
			return command[index+1], true
		}
	}
	return "", false
}
