package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func executeElevatedRequest(profile launchpad.Profile, options launchpad.ApplyOptions, lang language, windows bool) (bool, int, error) {
	directory, err := os.MkdirTemp("", "ssh-launchpad-elevate-*")
	if err != nil {
		return false, launchpad.ExitNeedsElevation, err
	}
	defer os.RemoveAll(directory)
	if windows && profile.Advanced.StateDir == "" {
		profile.Advanced.StateDir = filepath.Join(os.Getenv("ProgramData"), "SSH Launchpad")
	}
	request := elevatedRequest{
		Profile:      profile,
		Options:      options,
		ResponsePath: filepath.Join(directory, "response.json"),
	}
	data, err := json.Marshal(request)
	if err != nil {
		return false, launchpad.ExitInvalidProfile, err
	}
	requestPath := filepath.Join(directory, "request.json")
	if err := os.WriteFile(requestPath, data, 0o600); err != nil {
		return false, launchpad.ExitNeedsElevation, err
	}
	digest := sha256.Sum256(data)
	executable, err := os.Executable()
	if err != nil {
		return false, launchpad.ExitNeedsElevation, err
	}
	err = invokeElevated(executable, requestPath, hex.EncodeToString(digest[:]), lang)
	response, readErr := os.ReadFile(request.ResponsePath)
	if readErr != nil {
		if err != nil {
			return false, launchpad.ExitNeedsElevation, fmt.Errorf("%s", tr("permissionCancelled"))
		}
		return false, launchpad.ExitVerificationFailed, readErr
	}
	var report launchpad.Report
	if json.Unmarshal(response, &report) != nil {
		return false, launchpad.ExitVerificationFailed, errors.New(tr("operationFailed"))
	}
	if err != nil || !report.Success {
		if report.Error != "" {
			return false, report.ExitCode, errors.New(report.Error)
		}
		return false, report.ExitCode, err
	}
	return true, report.ExitCode, nil
}
