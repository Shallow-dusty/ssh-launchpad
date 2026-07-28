package launchpad

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"time"
)

var generatedPlanTimestamp = regexp.MustCompile(`\d{8}T\d{6}Z`)

// PlanDigest binds confirmation to the complete profile and executable plan.
// Generated backup timestamps are normalized so an unchanged re-plan has the
// same digest while every security-relevant command and parameter remains bound.
func PlanDigest(profile Profile, plan Plan) string {
	plan.Timestamp = time.Time{}
	plan.Digest = ""
	plan.Actions = append([]Action(nil), plan.Actions...)
	for index := range plan.Actions {
		plan.Actions[index].Command = append([]string(nil), plan.Actions[index].Command...)
		plan.Actions[index].RollbackCommand = append([]string(nil), plan.Actions[index].RollbackCommand...)
		for argument := range plan.Actions[index].Command {
			plan.Actions[index].Command[argument] = generatedPlanTimestamp.ReplaceAllString(plan.Actions[index].Command[argument], "<generated-timestamp>")
		}
		for argument := range plan.Actions[index].RollbackCommand {
			plan.Actions[index].RollbackCommand[argument] = generatedPlanTimestamp.ReplaceAllString(plan.Actions[index].RollbackCommand[argument], "<generated-timestamp>")
		}
	}
	payload := struct {
		Profile Profile `json:"profile"`
		Plan    Plan    `json:"plan"`
	}{Profile: profile, Plan: plan}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
