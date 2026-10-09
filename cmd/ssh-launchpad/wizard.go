package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

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
	if checkReport.Snapshot != nil {
		profile.Transport.Install = !checkReport.Snapshot.Tailscale.Installed
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
	if planReport.Plan != nil && len(planReport.Plan.Blockers) > 0 {
		for _, blocker := range planReport.Plan.Blockers {
			fmt.Fprintf(os.Stderr, "! %s\n", blocker)
		}
		return finishWizard(reader, launchpad.ExitVerificationFailed)
	}
	if planReport.Plan != nil && planReport.Plan.SelfCutDetected {
		fmt.Printf("\n! %s\n", tr("selfCutBlocked"))
		return finishWizard(reader, launchpad.ExitSelfCutBlocked)
	}
	if !strings.EqualFold(prompt(reader, tr("applyPrompt"), tr("no")), tr("yes")) {
		fmt.Printf("\n%s\n", tr("noChanges"))
		return finishWizard(reader, launchpad.ExitOK)
	}

	applyOptions := launchpad.ApplyOptions{
		Confirmed:          true,
		ExpectedPlanDigest: planReport.Plan.Digest,
		AutoRollback:       profile.Safety.AutoRollback,
	}
	apply, applyErr := executeStage(launchpad.StageApply, profile, applyOptions, true)
	code := apply.ExitCode
	if code == launchpad.ExitNeedsElevation {
		fmt.Printf("\n%s\n", tr("permissionPrompt"))
		if !strings.EqualFold(prompt(reader, tr("continuePrompt"), tr("yes")), tr("yes")) {
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
		for index, key := range keys {
			fmt.Printf("  %d. %s\n", index+1, key.label)
		}
		fmt.Printf("  %d. %s\n  %d. %s\n", len(keys)+1, tr("pasteKey"), len(keys)+2, tr("generateKey"))
		choiceText := prompt(reader, tr("choicePrompt"), "")
		choice, parseErr := strconv.Atoi(choiceText)
		if parseErr != nil || choice < 1 || choice > len(keys)+2 {
			return errors.New(tr("invalidChoice"))
		}
		if choice <= len(keys) {
			profile.SSH.PublicKeys = []string{keys[choice-1].value}
			return nil
		}
		if choice == len(keys)+2 {
			publicKey, err := generatePublicKey()
			if err != nil {
				return err
			}
			profile.SSH.PublicKeys = []string{publicKey}
			fmt.Printf("%s %s\n", glyph("[OK]", "✓"), tr("generatedKey"))
			return nil
		}
	}
	if len(keys) == 0 {
		fmt.Printf("%s\n", tr("noKey"))
		fmt.Printf("  1. %s\n  2. %s\n", tr("pasteKey"), tr("generateKey"))
	}
	choice := "1"
	if len(keys) == 0 {
		choice = prompt(reader, tr("choicePrompt"), "1")
	}
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

func finishWizard(reader *bufio.Reader, code int) int {
	if isInteractiveTerminal() && os.Getenv("SSH_LAUNCHPAD_LAUNCHER") == "" {
		fmt.Printf("\n%s", tr("pressEnter"))
		_, _ = reader.ReadString('\n')
	}
	return code
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
