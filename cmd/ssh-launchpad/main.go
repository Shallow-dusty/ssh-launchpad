package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

type language string

const (
	langAuto language = "auto"
	langZH   language = "zh-CN"
	langEN   language = "en"
)

type globalOptions struct {
	lang           language
	interactive    bool
	nonInteractive bool
	jsonOnly       bool
}

type elevatedRequest struct {
	Profile      launchpad.Profile      `json:"profile"`
	Options      launchpad.ApplyOptions `json:"options"`
	ResponsePath string                 `json:"responsePath"`
}

var currentLanguage = langEN

func main() {
	restore := configureTerminal()
	defer restore()
	os.Exit(run(os.Args[1:]))
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
		Confirmed:      *confirmed,
		AllowSelfCut:   *allowSelfCut,
		ScheduleRisky:  *scheduleRisky,
		AutoRollback:   profile.Safety.AutoRollback,
		JournalDir:     *journalDir,
		ExternalVerify: *externalVerify,
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
	switch stage {
	case launchpad.StageCheck:
		return engine.Check(ctx, profile)
	case launchpad.StagePlan:
		return engine.Plan(ctx, profile)
	case launchpad.StageApply:
		return engine.Apply(ctx, profile, options)
	case launchpad.StageVerify:
		return engine.Verify(ctx, profile)
	default:
		return launchpad.Report{}, errors.New("unsupported stage")
	}
}

func runWizard(options globalOptions) int {
	lock, err := acquireProcessLock()
	if err != nil {
		fmt.Fprintln(os.Stderr, tr("alreadyRunning"))
		return launchpad.ExitConfirmationRequired
	}
	defer lock()
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n%s\n%s\n\n", tr("welcome"), tr("chooseTask"))
	fmt.Printf("  1. %s\n  2. %s\n  3. %s\n\n", tr("setupTask"), tr("repairTask"), tr("checkTask"))
	choice := prompt(reader, tr("choicePrompt"), "1")
	if choice != "1" && choice != "2" && choice != "3" {
		fmt.Fprintln(os.Stderr, tr("invalidChoice"))
		return finishWizard(reader, launchpad.ExitInvalidProfile)
	}
	profile := launchpad.DefaultProfile()
	profile.Name = "guided"
	fmt.Printf("\n%s\n", tr("checking"))
	checkReport, checkErr := executeStage(launchpad.StageCheck, profile, launchpad.ApplyOptions{}, false)
	printSimpleCheck(checkReport)
	if checkErr != nil && checkReport.Snapshot == nil {
		fmt.Fprintln(os.Stderr, friendlyError(checkErr))
		return finishWizard(reader, checkReport.ExitCode)
	}
	if choice == "3" {
		return finishWizard(reader, checkReport.ExitCode)
	}

	if err := configureControllerKey(reader, &profile); err != nil {
		fmt.Fprintln(os.Stderr, friendlyError(err))
		return finishWizard(reader, launchpad.ExitInvalidProfile)
	}
	fmt.Printf("\n%s\n%s\n", tr("recommendTitle"), tr("recommendTailnet"))
	planReport, planErr := executeStage(launchpad.StagePlan, profile, launchpad.ApplyOptions{}, false)
	if planErr != nil {
		fmt.Fprintln(os.Stderr, friendlyError(planErr))
		return finishWizard(reader, planReport.ExitCode)
	}
	if planReport.Plan != nil && planReport.Plan.NoChanges {
		fmt.Printf("\n%s %s\n", glyph("[OK]", "✓"), tr("alreadyReady"))
		verify, _ := executeStage(launchpad.StageVerify, profile, launchpad.ApplyOptions{}, false)
		printVerifyNextSteps(verify, profile)
		return finishWizard(reader, verify.ExitCode)
	}
	printPlainPlan(planReport)
	if planReport.Plan != nil && planReport.Plan.SelfCutDetected {
		fmt.Printf("\n! %s\n", tr("selfCutBlocked"))
		return finishWizard(reader, launchpad.ExitSelfCutBlocked)
	}
	if strings.ToLower(prompt(reader, tr("applyPrompt"), tr("no"))) != strings.ToLower(tr("yes")) {
		fmt.Printf("\n%s\n", tr("noChanges"))
		return finishWizard(reader, launchpad.ExitOK)
	}

	applyOptions := launchpad.ApplyOptions{Confirmed: true, AutoRollback: profile.Safety.AutoRollback}
	apply, applyErr := executeStage(launchpad.StageApply, profile, applyOptions, true)
	code := apply.ExitCode
	if code == launchpad.ExitNeedsElevation {
		fmt.Printf("\n%s\n", tr("permissionPrompt"))
		if strings.ToLower(prompt(reader, tr("continuePrompt"), tr("yes"))) != strings.ToLower(tr("yes")) {
			fmt.Printf("%s\n", tr("permissionCancelled"))
			return finishWizard(reader, launchpad.ExitNeedsElevation)
		}
		_, code, applyErr = elevateAndApply(profile, applyOptions, currentLanguage)
	}
	if applyErr != nil || code != launchpad.ExitOK {
		fmt.Fprintf(os.Stderr, "\n%s\n", friendlyError(applyErr))
		return finishWizard(reader, code)
	}
	verify, verifyErr := executeStage(launchpad.StageVerify, profile, launchpad.ApplyOptions{}, false)
	printVerifyNextSteps(verify, profile)
	if verifyErr != nil {
		fmt.Fprintln(os.Stderr, friendlyError(verifyErr))
	}
	return finishWizard(reader, verify.ExitCode)
}

func configureControllerKey(reader *bufio.Reader, profile *launchpad.Profile) error {
	keys := discoverPublicKeys()
	fmt.Printf("\n%s\n%s\n", tr("keyTitle"), tr("keyExplain"))
	if len(keys) > 0 {
		fmt.Printf("%s %s\n", glyph("[OK]", "✓"), tr("foundKey", len(keys)))
		profile.SSH.PublicKeys = []string{keys[0]}
		return nil
	}
	fmt.Printf("%s\n", tr("noKey"))
	fmt.Printf("  1. %s\n  2. %s\n", tr("pasteKey"), tr("generateKey"))
	choice := prompt(reader, tr("choicePrompt"), "1")
	if choice == "2" {
		publicKey, err := generatePublicKey()
		if err != nil {
			return err
		}
		profile.SSH.PublicKeys = []string{publicKey}
		fmt.Printf("%s %s\n", glyph("[OK]", "✓"), tr("generatedKey"))
		return nil
	}
	value := prompt(reader, tr("pastePrompt"), "")
	if err := launchpad.ValidatePublicKey(value); err != nil {
		return errors.New(tr("publicOnly"))
	}
	profile.SSH.PublicKeys = []string{strings.TrimSpace(value)}
	return nil
}

func discoverPublicKeys() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
	var keys []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil && launchpad.ValidatePublicKey(string(data)) == nil {
			keys = append(keys, strings.TrimSpace(string(data)))
		}
	}
	return keys
}

func generatePublicKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(directory, "id_ed25519")
	if _, err := os.Stat(path); err == nil {
		return "", errors.New(tr("privateExists"))
	}
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", path, "-N", "", "-C", "ssh-launchpad-controller")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s: %s", tr("keygenFailed"), strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(path + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), launchpad.ValidatePublicKey(string(data))
}

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

func finishWizard(reader *bufio.Reader, code int) int {
	if isInteractiveTerminal() && os.Getenv("SSH_LAUNCHPAD_LAUNCHER") == "" {
		fmt.Printf("\n%s", tr("pressEnter"))
		_, _ = reader.ReadString('\n')
	}
	return code
}

func runElevatedApply(args []string) int {
	flags := flag.NewFlagSet("__elevated-apply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	requestPath := flags.String("request", "", "")
	digest := flags.String("sha256", "", "")
	lang := flags.String("lang", "en", "")
	if flags.Parse(args) != nil || *requestPath == "" || *digest == "" {
		return launchpad.ExitInvalidProfile
	}
	currentLanguage = resolveLanguage(language(*lang))
	data, err := os.ReadFile(*requestPath)
	if err != nil {
		return launchpad.ExitInvalidProfile
	}
	actual := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(actual[:]), *digest) {
		return launchpad.ExitInvalidProfile
	}
	var request elevatedRequest
	if json.Unmarshal(data, &request) != nil || request.ResponsePath == "" {
		return launchpad.ExitInvalidProfile
	}
	report, runErr := executeStage(launchpad.StageApply, request.Profile, request.Options, false)
	if err := writeReport(request.ResponsePath, report); err != nil {
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
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o600)
}

func acquireProcessLock() (func(), error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return func() {}, nil
	}
	directory := filepath.Join(cache, "ssh-launchpad")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "interactive.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 4*time.Hour {
			_ = os.Remove(path)
			file, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		}
	}
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
	_ = file.Close()
	return func() { _ = os.Remove(path) }, nil
}

func prompt(reader *bufio.Reader, label, fallback string) string {
	if fallback != "" {
		fmt.Printf("%s [%s]: ", label, fallback)
	} else {
		fmt.Printf("%s: ", label)
	}
	value, _ := reader.ReadString('\n')
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func resolveLanguage(requested language) language {
	if requested == langAuto {
		if env := os.Getenv("SSH_LAUNCHPAD_LANG"); env != "" {
			requested = language(env)
		}
	}
	if requested == langAuto {
		if saved := savedLanguage(); saved != langAuto {
			requested = saved
		}
	}
	if requested == langAuto {
		if runtime.GOOS != "windows" {
			locale := strings.ToLower(os.Getenv("LC_ALL") + " " + os.Getenv("LANG"))
			if locale != " " && !strings.Contains(locale, "utf-8") && !strings.Contains(locale, "utf8") {
				return langEN
			}
			if strings.Contains(locale, "zh") {
				return langZH
			}
		}
		if strings.HasPrefix(strings.ToLower(os.Getenv("LANG")), "zh") {
			return langZH
		}
		return systemLanguage()
	}
	return requested
}

func persistLanguage(value language) error {
	directory, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(directory, "SSH Launchpad", "language")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(value+"\n"), 0o600)
}

func savedLanguage() language {
	directory, err := os.UserConfigDir()
	if err != nil {
		return langAuto
	}
	data, err := os.ReadFile(filepath.Join(directory, "SSH Launchpad", "language"))
	if err != nil {
		return langAuto
	}
	value := language(strings.TrimSpace(string(data)))
	if value == langZH || value == langEN {
		return value
	}
	return langAuto
}

func glyph(ascii, unicode string) string {
	if runtime.GOOS != "windows" {
		locale := strings.ToLower(os.Getenv("LC_ALL") + " " + os.Getenv("LANG"))
		if locale != " " && !strings.Contains(locale, "utf-8") && !strings.Contains(locale, "utf8") {
			return ascii
		}
	}
	return unicode
}

func isInteractiveTerminal() bool {
	in, inErr := os.Stdin.Stat()
	out, outErr := os.Stdout.Stat()
	return inErr == nil && outErr == nil && in.Mode()&os.ModeCharDevice != 0 && out.Mode()&os.ModeCharDevice != 0 && os.Getenv("CI") == ""
}

func friendlyError(err error) string {
	if err == nil {
		return tr("operationFailed")
	}
	text := err.Error()
	switch {
	case strings.Contains(strings.ToLower(text), "checksum"):
		return tr("checksumFailed")
	case strings.Contains(strings.ToLower(text), "network"), strings.Contains(strings.ToLower(text), "timeout"):
		return tr("networkFailed")
	case strings.Contains(strings.ToLower(text), "private key"):
		return tr("publicOnly")
	default:
		return text
	}
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
