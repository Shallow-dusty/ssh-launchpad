package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	elevationprotocol "github.com/Shallow-dusty/ssh-launchpad/internal/elevation"
	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

type globalOptions struct {
	lang           language
	interactive    bool
	nonInteractive bool
	jsonOnly       bool
}

func main() {
	os.Exit(runWithTerminal(os.Args[1:]))
}

func runWithTerminal(args []string) int {
	restore := configureTerminal()
	defer restore()
	return run(args)
}

func run(args []string) int {
	if len(args) > 0 && args[0] == "__elevated-apply" {
		return runElevatedApply(args[1:])
	}
	options, args, err := parseGlobalOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return launchpad.ExitInvalidProfile
	}
	currentLanguage = resolveLanguage(options.lang)
	if options.lang == langZH || options.lang == langEN {
		_ = persistLanguage(options.lang)
	}
	if len(args) == 0 {
		if options.nonInteractive || (!options.interactive && !isInteractiveTerminal()) {
			printUsage(os.Stderr)
			return launchpad.ExitInvalidProfile
		}
		return runWizard(options)
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Printf("SSH Launchpad %s\n", launchpad.Version)
		return launchpad.ExitOK
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage(os.Stdout)
		return launchpad.ExitOK
	}
	if args[0] == "update" {
		info, err := launchpad.CheckForUpdate(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, friendlyError(err))
			return launchpad.ExitDownloadFailure
		}
		if options.jsonOnly {
			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(data))
		} else if info.Available {
			fmt.Printf("%s %s\n%s\n", tr("updateAvailable"), info.LatestVersion, info.URL)
		} else {
			fmt.Println(tr("upToDate"))
		}
		return launchpad.ExitOK
	}
	if args[0] == "rollback" {
		return runRollback(args[1:], options)
	}
	stage := launchpad.Stage(args[0])
	switch stage {
	case launchpad.StageCheck, launchpad.StagePlan, launchpad.StageApply, launchpad.StageVerify:
	default:
		fmt.Fprintln(os.Stderr, tr("unknownCommand", args[0]))
		printUsage(os.Stderr)
		return launchpad.ExitInvalidProfile
	}
	return runStage(stage, args[1:], options)
}

func parseGlobalOptions(args []string) (globalOptions, []string, error) {
	options := globalOptions{lang: langAuto}
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--lang":
			if i+1 >= len(args) {
				return options, nil, errors.New("--lang requires auto, zh-CN, or en")
			}
			i++
			options.lang = language(args[i])
		case "--interactive":
			options.interactive = true
		case "--non-interactive":
			options.nonInteractive = true
		case "--json":
			options.jsonOnly = true
		default:
			filtered = append(filtered, args[i])
		}
	}
	if options.lang != langAuto && options.lang != langZH && options.lang != langEN {
		return options, nil, fmt.Errorf("unsupported language %q", options.lang)
	}
	if options.interactive && options.nonInteractive {
		return options, nil, errors.New("--interactive and --non-interactive cannot be used together")
	}
	return options, filtered, nil
}

func runStage(stage launchpad.Stage, args []string, options globalOptions) int {
	flags := flag.NewFlagSet(string(stage), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	profilePath := flags.String("profile", "", tr("profileHelp"))
	outputPath := flags.String("output", "-", tr("outputHelp"))
	confirmed := flags.Bool("yes", false, tr("confirmHelp"))
	planDigest := flags.String("plan-digest", "", tr("planDigestHelp"))
	allowSelfCut := flags.Bool("allow-self-cut", false, tr("selfCutHelp"))
	scheduleRisky := flags.Bool("schedule-risky", false, tr("scheduleHelp"))
	journalDir := flags.String("journal-dir", "", tr("journalHelp"))
	externalVerify := flags.String("external-verify-target", "", tr("externalHelp"))
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, friendlyError(err))
		return launchpad.ExitInvalidProfile
	}
	profile, err := launchpad.LoadProfile(*profilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, friendlyError(err))
		return launchpad.ExitInvalidProfile
	}
	applyOptions := launchpad.ApplyOptions{
		Confirmed:          *confirmed,
		ExpectedPlanDigest: *planDigest,
		AllowSelfCut:       *allowSelfCut,
		ScheduleRisky:      *scheduleRisky,
		AutoRollback:       profile.Safety.AutoRollback,
		JournalDir:         *journalDir,
		ExternalVerify:     *externalVerify,
	}
	report, err := executeStage(stage, profile, applyOptions, *outputPath != "-" && !options.jsonOnly)
	if stage == launchpad.StageApply && report.ExitCode == launchpad.ExitNeedsElevation && *confirmed && isInteractiveTerminal() && !options.nonInteractive {
		ok, code, elevateErr := elevateAndApply(profile, applyOptions, currentLanguage)
		if elevateErr != nil {
			fmt.Fprintln(os.Stderr, friendlyError(elevateErr))
			return code
		}
		if ok {
			fmt.Fprintln(os.Stderr, tr("permissionDone"))
		}
		return code
	}
	if writeErr := writeReport(*outputPath, report); writeErr != nil {
		fmt.Fprintln(os.Stderr, friendlyError(writeErr))
		return launchpad.ExitVerificationFailed
	}
	if err != nil && *outputPath != "-" {
		fmt.Fprintln(os.Stderr, friendlyError(err))
	}
	return report.ExitCode
}

func executeStage(stage launchpad.Stage, profile launchpad.Profile, options launchpad.ApplyOptions, showEvents bool) (launchpad.Report, error) {
	engine := launchpad.NewEngine(func(event launchpad.Event) {
		if showEvents {
			fmt.Fprintf(os.Stderr, "%s %s\n", glyph("*", "•"), localEvent(event))
		}
	})
	ctx := context.Background()
	var report launchpad.Report
	var stageErr error
	switch stage {
	case launchpad.StageCheck:
		report, stageErr = engine.Check(ctx, profile)
	case launchpad.StagePlan:
		report, stageErr = engine.Plan(ctx, profile)
	case launchpad.StageApply:
		report, stageErr = engine.Apply(ctx, profile, options)
	case launchpad.StageVerify:
		report, stageErr = engine.Verify(ctx, profile)
	default:
		return launchpad.Report{}, errors.New("unsupported stage")
	}
	if stageErr != nil && report.Error != "" {
		stageErr = &reportError{report: report, cause: stageErr}
	}
	return report, stageErr
}

func runElevatedApply(args []string) int {
	requestPath, digest, err := elevationprotocol.ParseArguments(args)
	if err != nil {
		return launchpad.ExitInvalidProfile
	}
	request, err := elevationprotocol.ConsumeRequest(requestPath, digest)
	if err != nil {
		return launchpad.ExitInvalidProfile
	}
	currentLanguage = resolveLanguage(language(request.Language))
	if currentLanguage == langAuto {
		currentLanguage = langEN
	}
	var report launchpad.Report
	var runErr error
	if request.Operation == launchpad.StageRollback {
		report, runErr = (launchpad.Executor{}).RollbackVerified(context.Background(), request.JournalPath, request.JournalDigest)
	} else {
		report, runErr = executeStage(launchpad.StageApply, request.Profile, request.Options, false)
	}
	response := elevationprotocol.Response{Report: report}
	if runErr != nil {
		response.Error = runErr.Error()
	}
	if err := elevationprotocol.WriteResponse(request.ResponsePath, response); err != nil {
		return launchpad.ExitVerificationFailed
	}
	if runErr != nil {
		return report.ExitCode
	}
	return report.ExitCode
}

func runRollback(args []string, options globalOptions) int {
	flags := flag.NewFlagSet("rollback", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	journal := flags.String("journal", "", tr("journalHelp"))
	output := flags.String("output", "-", tr("outputHelp"))
	if err := flags.Parse(args); err != nil || *journal == "" {
		fmt.Fprintln(os.Stderr, tr("rollbackRequiresJournal"))
		return launchpad.ExitInvalidProfile
	}
	engine := launchpad.NewEngine(nil)
	report, err := engine.Executor.Rollback(context.Background(), *journal)
	if writeErr := writeReport(*output, report); writeErr != nil {
		fmt.Fprintln(os.Stderr, friendlyError(writeErr))
		return launchpad.ExitVerificationFailed
	}
	if err != nil && !options.jsonOnly {
		fmt.Fprintln(os.Stderr, friendlyError(err))
	}
	return report.ExitCode
}

func isInteractiveTerminal() bool {
	in, inErr := os.Stdin.Stat()
	out, outErr := os.Stdout.Stat()
	return inErr == nil && outErr == nil && in.Mode()&os.ModeCharDevice != 0 && out.Mode()&os.ModeCharDevice != 0 && os.Getenv("CI") == ""
}
