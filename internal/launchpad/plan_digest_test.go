package launchpad

import (
	"strings"
	"testing"
)

func TestPlanDigestIsStableAcrossGeneratedBackupTimestamps(t *testing.T) {
	profile := DefaultProfile()
	profile.SSH.Port = 2222
	first := (Planner{}).Build(profile, healthySnapshot(PlatformLinux))
	if len(first.Digest) != 64 || len(first.Actions) == 0 {
		t.Fatalf("planner did not produce a bound digest: %#v", first)
	}

	second := first
	second.Actions = append([]Action(nil), first.Actions...)
	second.Actions[0].Command = append([]string(nil), first.Actions[0].Command...)
	for index := range second.Actions[0].Command {
		second.Actions[0].Command[index] = generatedPlanTimestamp.ReplaceAllString(second.Actions[0].Command[index], "20990101T010101Z")
	}
	if got := PlanDigest(profile, second); got != first.Digest {
		t.Fatalf("generated backup timestamp changed semantic plan digest: %s != %s", got, first.Digest)
	}
	for _, argument := range first.Actions[0].Command {
		if strings.Contains(argument, "<generated-timestamp>") {
			t.Fatal("digest calculation mutated the executable plan")
		}
	}
}

func TestPlanDigestChangesWithProfileOrCommand(t *testing.T) {
	profile := DefaultProfile()
	profile.SSH.Port = 2222
	plan := (Planner{}).Build(profile, healthySnapshot(PlatformLinux))

	changedProfile := profile
	changedProfile.SSH.Port = 2200
	if PlanDigest(changedProfile, plan) == plan.Digest {
		t.Fatal("profile change did not invalidate plan digest")
	}

	changedPlan := plan
	changedPlan.Actions = append([]Action(nil), plan.Actions...)
	changedPlan.Actions[0].Command = append([]string(nil), plan.Actions[0].Command...)
	changedPlan.Actions[0].Command = append(changedPlan.Actions[0].Command, "unexpected")
	if PlanDigest(profile, changedPlan) == plan.Digest {
		t.Fatal("command change did not invalidate plan digest")
	}
}
