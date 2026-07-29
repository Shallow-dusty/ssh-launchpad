package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	elevationprotocol "github.com/Shallow-dusty/ssh-launchpad/internal/elevation"
	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

type ElevatedJob struct {
	ID     string            `json:"id"`
	State  string            `json:"state"`
	Report *launchpad.Report `json:"report,omitempty"`
	Error  string            `json:"error,omitempty"`
	Events []launchpad.Event `json:"events,omitempty"`
}

type elevatedJobRecord struct {
	mu           sync.Mutex
	status       ElevatedJob
	directory    string
	responsePath string
	eventsPath   string
}

var errUACCancelled = elevationprotocol.ErrCancelled

func newJobID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", value[:]), nil
}

func helperPowerShell(executable, requestPath, digest string) string {
	return elevationprotocol.WindowsStartProcessScript(executable, []string{
		"--elevated-helper",
		"--request", requestPath,
		"--sha256", digest,
	})
}

func launchElevatedHelper(ctx context.Context, executable, requestPath, digest string) error {
	if runtime.GOOS != "windows" {
		return errors.New("windows UAC helper is only available on Windows")
	}
	command := helperPowerShell(executable, requestPath, digest)
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == elevationprotocol.WindowsCancelledExitCode {
		return errUACCancelled
	}
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("windows UAC helper process failed: %w", err)
	}
	return fmt.Errorf("windows UAC helper process failed: %s: %w", detail, err)
}

func runElevatedHelper(args []string) int {
	requestPath, digest, err := elevationprotocol.ParseArguments(args)
	if err != nil {
		return launchpad.ExitInvalidProfile
	}
	request, err := elevationprotocol.ConsumeRequest(requestPath, digest)
	if err != nil {
		return launchpad.ExitInvalidProfile
	}
	eventFile, err := elevationprotocol.OpenEventFile(request.EventsPath)
	if err != nil {
		_ = elevationprotocol.WriteResponse(request.ResponsePath, elevationprotocol.Response{Error: err.Error()})
		return launchpad.ExitVerificationFailed
	}
	defer eventFile.Close()
	engine := launchpad.NewEngine(func(event launchpad.Event) {
		encoded, marshalErr := json.Marshal(event)
		if marshalErr == nil {
			_, _ = eventFile.Write(append(encoded, '\n'))
			_ = eventFile.Sync()
		}
	})
	report, applyErr := engine.Apply(context.Background(), request.Profile, request.Options)
	response := elevationprotocol.Response{Report: report}
	if applyErr != nil {
		response.Error = applyErr.Error()
	}
	if err := elevationprotocol.WriteResponse(request.ResponsePath, response); err != nil {
		return launchpad.ExitVerificationFailed
	}
	return report.ExitCode
}

func readJobEvents(path string) []launchpad.Event {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var events []launchpad.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event launchpad.Event
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			events = append(events, event)
		}
	}
	return events
}

func jobRoot() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "SSH Launchpad", "jobs")
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func pruneOldJobs(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}
