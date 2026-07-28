package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	elevationprotocol "github.com/Shallow-dusty/ssh-launchpad/internal/elevation"
	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func executeElevatedRequest(profile launchpad.Profile, options launchpad.ApplyOptions, lang language, windows bool) (bool, int, error) {
	directory, err := os.MkdirTemp("", "ssh-launchpad-elevate-*")
	if err != nil {
		return false, launchpad.ExitNeedsElevation, err
	}
	defer os.RemoveAll(directory)
	if windows && options.JournalDir == "" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		options.JournalDir = filepath.Join(programData, "SSH Launchpad")
	}
	responsePath := filepath.Join(directory, "response.json")
	if err := elevationprotocol.PrecreateFile(responsePath); err != nil {
		return false, launchpad.ExitNeedsElevation, err
	}
	request := elevationprotocol.NewRequest(profile, options, responsePath, "", string(lang))
	requestPath := filepath.Join(directory, "request.json")
	digest, err := elevationprotocol.WriteRequest(requestPath, request)
	if err != nil {
		return false, launchpad.ExitNeedsElevation, err
	}
	executable, err := os.Executable()
	if err != nil {
		return false, launchpad.ExitNeedsElevation, err
	}
	err = invokeElevated(executable, requestPath, digest)
	response, readErr := elevationprotocol.ReadResponse(responsePath)
	if readErr != nil {
		if errors.Is(err, errPermissionCancelled) {
			return false, launchpad.ExitNeedsElevation, fmt.Errorf("%s", tr("permissionCancelled"))
		}
		if err != nil {
			return false, launchpad.ExitNeedsElevation, fmt.Errorf("elevated helper failed: %w", err)
		}
		return false, launchpad.ExitVerificationFailed, readErr
	}
	report := response.Report
	if response.Error != "" && report.Error == "" {
		report.Error = response.Error
	}
	if err != nil || !report.Success {
		if report.Error != "" {
			return false, report.ExitCode, errors.New(report.Error)
		}
		return false, report.ExitCode, err
	}
	return true, report.ExitCode, nil
}

var errPermissionCancelled = errors.New("elevation request cancelled")
