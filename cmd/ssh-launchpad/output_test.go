package main

import (
	"context"
	"errors"
	"testing"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

func TestCLIUsesReasonMetadataRatherThanErrorWords(t *testing.T) {
	previous := currentLanguage
	currentLanguage = langZH
	defer func() { currentLanguage = previous }()
	cause := errors.New("unrelated checksum and network words")
	for reason, message := range map[launchpad.FailureReason]string{
		launchpad.ReasonConfirmationRequired: tr("confirmationRequired"),
		launchpad.ReasonPlanChanged:          tr("planChanged"),
		launchpad.ReasonMutationBusy:         tr("alreadyRunning"),
		launchpad.ReasonDownloadFailed:       tr("checksumFailed"),
	} {
		err := &reportError{report: launchpad.Report{ReasonCode: reason, Error: cause.Error()}, cause: cause}
		if got := friendlyError(err); got != message {
			t.Fatalf("%s: got %q, want %q", reason, got, message)
		}
	}
	if got := friendlyError(cause); got != cause.Error() {
		t.Fatalf("untyped error was guessed from prose: %q", got)
	}
	wrapped := &reportError{report: launchpad.Report{Error: "cancelled"}, cause: context.Canceled}
	if !errors.Is(wrapped, context.Canceled) {
		t.Fatal("report wrapper lost the original error identity")
	}
}
