package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWorkflowToolchainsFollowProjectSecurityMinimum(t *testing.T) {
	module, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	patchMinimum := false
	for _, line := range strings.Split(string(module), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			patchMinimum = len(strings.Split(fields[1], ".")) == 3
		}
	}
	if !patchMinimum {
		t.Fatal("go.mod must require a patch-level security minimum, not just a Go minor release")
	}
	for _, path := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var workflow checkWorkflow
		if err := yaml.Unmarshal(data, &workflow); err != nil {
			t.Fatal(err)
		}
		setups := 0
		for name, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "actions/setup-go@") {
					setups++
					if step.With["go-version-file"] != "go.mod" || step.With["go-version"] != nil {
						t.Errorf("%s/%s must derive its Go version from go.mod, without a second pin", path, name)
					}
				}
			}
		}
		if setups == 0 {
			t.Errorf("%s has no Go setup to validate", path)
		}
	}
}
