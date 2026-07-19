//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

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

func invokeElevated(executable, requestPath, digest string, lang language) error {
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	script := fmt.Sprintf(
		"$p=Start-Process -FilePath %s -Verb RunAs -Wait -PassThru -ArgumentList @('__elevated-apply','--request',%s,'--sha256',%s,'--lang',%s); exit $p.ExitCode",
		quote(executable), quote(requestPath), quote(digest), quote(string(lang)),
	)
	encoded := base64.StdEncoding.EncodeToString([]byte(utf16LE(script)))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func utf16LE(value string) string {
	encoded := syscall.StringToUTF16(value)
	bytes := make([]byte, 0, len(encoded)*2)
	for _, unit := range encoded[:len(encoded)-1] {
		bytes = append(bytes, byte(unit), byte(unit>>8))
	}
	return string(bytes)
}
