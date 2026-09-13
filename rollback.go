package main

import (
	"context"
	"os"
	"path/filepath"

	elevationprotocol "github.com/Shallow-dusty/ssh-launchpad/internal/elevation"
	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

// The GUI has already requested explicit confirmation. Reuse the same fixed,
// access-restricted elevation protocol, binding the journal bytes as well.
func (a *App) elevatedRollback(path string) (launchpad.Report, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return launchpad.Report{}, err
	}
	digest, err := launchpad.FileSHA256(path)
	if err != nil {
		return launchpad.Report{}, err
	}
	root, err := jobRoot()
	if err != nil {
		return launchpad.Report{}, err
	}
	directory, err := os.MkdirTemp(root, "rollback-")
	if err != nil {
		return launchpad.Report{}, err
	}
	defer os.RemoveAll(directory)
	responsePath := filepath.Join(directory, "response.json")
	eventsPath := filepath.Join(directory, "events.jsonl")
	for _, file := range []string{responsePath, eventsPath} {
		if err := elevationprotocol.PrecreateFile(file); err != nil {
			return launchpad.Report{}, err
		}
	}
	requestPath := filepath.Join(directory, "request.json")
	request := elevationprotocol.NewRollbackRequest(path, digest, responsePath, eventsPath)
	requestDigest, err := elevationprotocol.WriteRequest(requestPath, request)
	if err != nil {
		return launchpad.Report{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return launchpad.Report{}, err
	}
	launchErr := launchElevatedHelper(context.Background(), exe, requestPath, requestDigest)
	response, err := elevationprotocol.ReadResponse(responsePath)
	if err != nil {
		if launchErr != nil {
			return launchpad.Report{}, launchErr
		}
		return launchpad.Report{}, err
	}
	if response.Report.Error == "" {
		response.Report.Error = response.Error
	}
	return response.Report, nil
}
