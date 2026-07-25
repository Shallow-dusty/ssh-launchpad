//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	elevationprotocol "github.com/Shallow-dusty/ssh-launchpad/internal/elevation"
	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	getConsoleCP             = kernel32.NewProc("GetConsoleCP")
	getConsoleOutputCP       = kernel32.NewProc("GetConsoleOutputCP")
	setConsoleCP             = kernel32.NewProc("SetConsoleCP")
	setConsoleOutputCP       = kernel32.NewProc("SetConsoleOutputCP")
	getUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
)

func configureTerminal() func() {
	input, _, _ := getConsoleCP.Call()
	output, _, _ := getConsoleOutputCP.Call()
	if input != 0 {
		_, _, _ = setConsoleCP.Call(65001)
	}
	if output != 0 {
		_, _, _ = setConsoleOutputCP.Call(65001)
	}
	return func() {
		if input != 0 {
			_, _, _ = setConsoleCP.Call(input)
		}
		if output != 0 {
			_, _, _ = setConsoleOutputCP.Call(output)
		}
	}
}

func systemLanguage() language {
	buffer := make([]uint16, 85)
	result, _, _ := getUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if result > 0 && strings.HasPrefix(strings.ToLower(syscall.UTF16ToString(buffer)), "zh") {
		return langZH
	}
	return langEN
}

func elevateAndApply(profile launchpad.Profile, options launchpad.ApplyOptions, lang language) (bool, int, error) {
	return executeElevatedRequest(profile, options, lang, true)
}

func invokeElevated(executable, requestPath, digest string) error {
	arguments := []string{"__elevated-apply", "--request", requestPath, "--sha256", digest}
	script := elevationprotocol.WindowsStartProcessScript(executable, arguments)
	encoded := elevationprotocol.UTF16LEBase64(script)
	// #nosec G702 -- the executable and switches are constant; each dynamic
	// script value is single-quoted by WindowsStartProcessScript and tested.
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	err := cmd.Run()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == elevationprotocol.WindowsCancelledExitCode {
		return errPermissionCancelled
	}
	return err
}
