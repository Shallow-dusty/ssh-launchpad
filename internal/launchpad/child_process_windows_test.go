//go:build windows

package launchpad

import (
	"os/exec"
	"testing"
)

func TestConfigureChildProcessHidesConsoleWindow(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "exit", "0")
	configureChildProcess(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("expected Windows process attributes")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("expected child process window to be hidden")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatal("expected CREATE_NO_WINDOW")
	}
}
