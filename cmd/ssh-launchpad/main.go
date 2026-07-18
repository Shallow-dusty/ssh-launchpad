package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return launchpad.ExitInvalidProfile
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Printf("ssh-launchpad %s\n", launchpad.Version)
		return launchpad.ExitOK
	}
	if args[0] == "rollback" {
		return runRollback(args[1:])
	}
	stage := launchpad.Stage(args[0])
	switch stage {
	case launchpad.StageCheck, launchpad.StagePlan, launchpad.StageApply, launchpad.StageVerify:
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		printUsage()
		return launchpad.ExitInvalidProfile
	}
	flags := flag.NewFlagSet(string(stage), flag.ContinueOnError)
	profilePath := flags.String("profile", "", "JSON or YAML profile path")
	outputPath := flags.String("output", "-", "report path, or - for stdout")
	confirmed := flags.Bool("yes", false, "confirm Apply")
	allowSelfCut := flags.Bool("allow-self-cut", false, "allow changes that can cut the active control channel")
	scheduleRisky := flags.Bool("schedule-risky", false, "schedule risky control-plane actions after a delay")
	journalDir := flags.String("journal-dir", "", "directory for rollback journals")
	externalVerify := flags.String("external-verify-target", "", "controller-visible host:port used after scheduled Apply")
	if err := flags.Parse(args[1:]); err != nil {
		return launchpad.ExitInvalidProfile
	}
	profile, err := launchpad.LoadProfile(*profilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return launchpad.ExitInvalidProfile
	}
	ctx := context.Background()
	engine := launchpad.NewEngine(func(event launchpad.Event) {
		if *outputPath != "-" {
			fmt.Fprintf(os.Stderr, "[%s] %s\n", event.Kind, event.Message)
		}
	})
	var report launchpad.Report
	switch stage {
	case launchpad.StageCheck:
		report, err = engine.Check(ctx, profile)
	case launchpad.StagePlan:
		report, err = engine.Plan(ctx, profile)
	case launchpad.StageApply:
		report, err = engine.Apply(ctx, profile, launchpad.ApplyOptions{
			Confirmed:      *confirmed,
			AllowSelfCut:   *allowSelfCut,
			ScheduleRisky:  *scheduleRisky,
			AutoRollback:   profile.Safety.AutoRollback,
			JournalDir:     *journalDir,
			ExternalVerify: *externalVerify,
		})
	case launchpad.StageVerify:
		report, err = engine.Verify(ctx, profile)
	}
	if writeErr := writeReport(*outputPath, report); writeErr != nil {
		fmt.Fprintln(os.Stderr, writeErr)
		return launchpad.ExitVerificationFailed
	}
	if err != nil {
		if *outputPath != "-" {
			fmt.Fprintln(os.Stderr, err)
		}
		return report.ExitCode
	}
	return report.ExitCode
}

func runRollback(args []string) int {
	flags := flag.NewFlagSet("rollback", flag.ContinueOnError)
	journal := flags.String("journal", "", "rollback journal path")
	output := flags.String("output", "-", "report path, or - for stdout")
	if err := flags.Parse(args); err != nil || *journal == "" {
		fmt.Fprintln(os.Stderr, "rollback requires --journal")
		return launchpad.ExitInvalidProfile
	}
	engine := launchpad.NewEngine(nil)
	report, err := engine.Executor.Rollback(context.Background(), *journal)
	if writeErr := writeReport(*output, report); writeErr != nil {
		fmt.Fprintln(os.Stderr, writeErr)
		return launchpad.ExitVerificationFailed
	}
	if err != nil {
		return report.ExitCode
	}
	return report.ExitCode
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `SSH Launchpad

Usage:
  ssh-launchpad check  [--profile profile.yaml] [--output report.json]
  ssh-launchpad plan   [--profile profile.yaml] [--output plan.json]
  ssh-launchpad apply  --profile profile.yaml --yes [--schedule-risky]
  ssh-launchpad verify [--profile profile.yaml] [--output verify.json]
  ssh-launchpad rollback --journal artifacts/<id>.journal.json
  ssh-launchpad version

Check and Plan are strictly read-only. Verify never elevates. Apply requires
explicit confirmation and blocks active-channel self-cut by default.`)
}
