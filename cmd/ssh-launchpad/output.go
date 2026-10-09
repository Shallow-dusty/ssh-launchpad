package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func printSimpleCheck(report launchpad.Report) {
	if report.Snapshot == nil {
		fmt.Printf("%s %s\n", glyph("[X]", "✗"), tr("checkFailed"))
		return
	}
	s := report.Snapshot
	missing := 0
	if !s.SSHServer.Installed {
		missing++
	}
	if !s.SSHService.Running {
		missing++
	}
	if missing == 0 {
		fmt.Printf("%s %s\n", glyph("[OK]", "✓"), tr("ready"))
	} else {
		fmt.Printf("! %s\n", tr("missingSteps", missing))
	}
	fmt.Printf("  %s: %s\n  OpenSSH: %s\n  Tailscale: %s\n", tr("computer"), s.Hostname, yesNo(s.SSHServer.Installed), yesNo(s.Tailscale.Online))
}

func printPlainPlan(report launchpad.Report) {
	if report.Plan == nil {
		return
	}
	fmt.Printf("\n%s\n", tr("willChange"))
	for _, action := range report.Plan.Actions {
		fmt.Printf("  %s %s\n", glyph("*", "•"), humanAction(action))
	}
	fmt.Printf("  %s %s\n", glyph("*", "•"), tr("whoCanConnect"))
}

func printVerifyNextSteps(report launchpad.Report, profile launchpad.Profile) {
	if report.Success {
		fmt.Printf("\n%s %s\n", glyph("[OK]", "✓"), tr("ready"))
	} else {
		fmt.Printf("\n! %s\n", tr("verifyNeedsOtherDevice"))
	}
	host := "this-computer"
	if report.Snapshot != nil && report.Snapshot.Hostname != "" {
		host = report.Snapshot.Hostname
	}
	fmt.Printf("%s\n  ssh -p %d <username>@%s\n%s\n", tr("copyCommand"), profile.SSH.Port, host, tr("fingerprintWarning"))
}

func writeReport(path string, report launchpad.Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "" || path == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	parent := filepath.Dir(path)
	if parent != "." {
		// #nosec G301 G703 -- --output intentionally allows the caller to select a report directory.
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	// #nosec G703 -- --output is an explicit caller-selected destination, not an archive member or server-controlled path.
	return os.WriteFile(path, data, 0o600)
}

type reportError struct {
	report launchpad.Report
	cause  error
}

func (e *reportError) Error() string { return e.report.Error }
func (e *reportError) Unwrap() error { return e.cause }

func friendlyError(err error) string {
	if err == nil {
		return tr("operationFailed")
	}
	var reported *reportError
	if errors.As(err, &reported) {
		switch reported.report.ReasonCode {
		case launchpad.ReasonDownloadFailed:
			return tr("checksumFailed")
		case launchpad.ReasonSelfCutBlocked:
			return tr("selfCutBlocked")
		case launchpad.ReasonElevationRequired:
			return tr("permissionPrompt")
		case launchpad.ReasonConfirmationRequired:
			return tr("confirmationRequired")
		case launchpad.ReasonPlanChanged:
			return tr("planChanged")
		case launchpad.ReasonMutationBusy:
			return tr("alreadyRunning")
		}
	}
	// Untyped file/runtime errors retain their actual detail, not a guessed
	// classification based on words such as network, checksum or private key.
	return err.Error()
}

func yesNo(value bool) string {
	if value {
		return tr("yes")
	}
	return tr("no")
}

func humanAction(action launchpad.Action) string {
	switch action.Operation {
	case "install_ssh":
		return tr("installSSH")
	case "configure_sshd":
		return tr("configureSSH")
	case "configure_keys":
		return tr("configureKeys")
	case "enable_sshd":
		return tr("enableSSH")
	case "configure_firewall":
		return tr("configureFirewall", action.Params["port"])
	case "install_tailscale":
		return tr("installTailscale")
	default:
		return tr("systemChange")
	}
}

func localEvent(event launchpad.Event) string {
	switch event.Kind {
	case "started":
		return tr("working")
	case "completed":
		return tr("completed")
	case "rollback":
		return tr("rollingBack")
	default:
		return event.Message
	}
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, tr("usage"))
}
