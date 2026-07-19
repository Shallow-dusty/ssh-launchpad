package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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

type helperResponse struct {
	Report launchpad.Report `json:"report"`
	Error  string           `json:"error,omitempty"`
}

func newJobID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func requestDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func verifyRequestDigest(path, expected string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(requestDigest(data), expected) {
		return nil, errors.New("elevated request integrity check failed")
	}
	return data, nil
}

func helperPowerShell(executable, requestPath, responsePath, eventsPath, digest string) string {
	args := []string{
		"--elevated-helper",
		"--request", requestPath,
		"--response", responsePath,
		"--events", eventsPath,
		"--sha256", digest,
	}
	quoted := make([]string, len(args))
	for i, value := range args {
		quoted[i] = psSingleQuote(value)
	}
	return fmt.Sprintf(
		"$p=Start-Process -FilePath %s -ArgumentList @(%s) -Verb RunAs -Wait -PassThru; exit $p.ExitCode",
		psSingleQuote(executable),
		strings.Join(quoted, ","),
	)
}

func psSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func launchElevatedHelper(ctx context.Context, executable, requestPath, responsePath, eventsPath, digest string) error {
	if runtime.GOOS != "windows" {
		return errors.New("Windows UAC helper is only available on Windows")
	}
	command := helperPowerShell(executable, requestPath, responsePath, eventsPath, digest)
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	return cmd.Run()
}

func runElevatedHelper(args []string) int {
	values, err := parseHelperArgs(args)
	if err != nil {
		return launchpad.ExitInvalidProfile
	}
	data, err := verifyRequestDigest(values["request"], values["sha256"])
	if err != nil {
		_ = writeHelperResponse(values["response"], helperResponse{Error: err.Error()})
		return launchpad.ExitInvalidProfile
	}
	var request DesktopRequest
	if err := json.Unmarshal(data, &request); err != nil {
		_ = writeHelperResponse(values["response"], helperResponse{Error: err.Error()})
		return launchpad.ExitInvalidProfile
	}
	if request.Stage != launchpad.StageApply || !request.Confirmed {
		err = errors.New("elevated helper accepts only explicitly confirmed Apply requests")
		_ = writeHelperResponse(values["response"], helperResponse{Error: err.Error()})
		return launchpad.ExitConfirmationRequired
	}
	if err := request.Profile.Validate(); err != nil {
		_ = writeHelperResponse(values["response"], helperResponse{Error: err.Error()})
		return launchpad.ExitInvalidProfile
	}
	eventFile, err := os.OpenFile(values["events"], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		_ = writeHelperResponse(values["response"], helperResponse{Error: err.Error()})
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
	report, applyErr := engine.Apply(context.Background(), request.Profile, launchpad.ApplyOptions{
		Confirmed:      true,
		AllowSelfCut:   request.AllowSelfCut,
		ScheduleRisky:  request.ScheduleRisky,
		AutoRollback:   request.Profile.Safety.AutoRollback,
		ExternalVerify: request.ExternalVerify,
	})
	response := helperResponse{Report: report}
	if applyErr != nil {
		response.Error = applyErr.Error()
	}
	if err := writeHelperResponse(values["response"], response); err != nil {
		return launchpad.ExitVerificationFailed
	}
	return report.ExitCode
}

func parseHelperArgs(args []string) (map[string]string, error) {
	values := map[string]string{}
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "--") || i+1 >= len(args) {
			return nil, errors.New("invalid elevated helper arguments")
		}
		values[strings.TrimPrefix(args[i], "--")] = args[i+1]
		i++
	}
	for _, key := range []string{"request", "response", "events", "sha256"} {
		if strings.TrimSpace(values[key]) == "" {
			return nil, fmt.Errorf("missing --%s", key)
		}
	}
	return values, nil
}

func writeHelperResponse(path string, response helperResponse) error {
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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

func readHelperResponse(path string) (helperResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return helperResponse{}, err
	}
	var response helperResponse
	err = json.Unmarshal(data, &response)
	return response, err
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
