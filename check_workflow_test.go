package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type checkWorkflow struct {
	Jobs map[string]struct {
		Steps []struct {
			Name             string         `yaml:"name"`
			Uses             string         `yaml:"uses"`
			Run              string         `yaml:"run"`
			With             map[string]any `yaml:"with"`
			WorkingDirectory string         `yaml:"working-directory"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func TestSharedChecksAreExecutableWorkflowSteps(t *testing.T) {
	for _, fixture := range []struct {
		path   string
		groups map[string][]string
	}{
		{".github/workflows/ci.yml", map[string][]string{
			"go": {"go", "docs"}, "vuln": {"vuln"}, "scripts": {"scripts"}, "ui": {"ui"},
		}},
		{".github/workflows/release.yml", map[string][]string{
			"quality": {"go", "race", "docs", "scripts", "security"}, "desktop-windows": {"scripts", "ui"},
		}},
	} {
		data, err := os.ReadFile(fixture.path)
		if err != nil {
			t.Fatal(err)
		}
		var workflow checkWorkflow
		if err := yaml.Unmarshal(data, &workflow); err != nil {
			t.Fatalf("invalid workflow YAML %s: %v", fixture.path, err)
		}
		for name, groups := range fixture.groups {
			job, exists := workflow.Jobs[name]
			if !exists {
				t.Fatalf("missing job %s in %s", name, fixture.path)
			}
			nodeReady := false
			seen := map[string]bool{}
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "actions/setup-node@") && step.With["node-version"] == "24" {
					nodeReady = true
				}
				if !strings.Contains(step.Run, "node scripts/check.mjs ") {
					continue
				}
				if !nodeReady || step.WorkingDirectory != "" || strings.Count(step.Run, "node scripts/check.mjs ") != 1 {
					t.Fatalf("non-executable or failure-masking shared step in %s/%s: %+v", fixture.path, name, step)
				}
				seen[strings.TrimPrefix(strings.TrimSpace(step.Run), "node scripts/check.mjs ")] = true
			}
			for _, group := range groups {
				if !seen[group] {
					t.Errorf("%s/%s is missing an executable %s gate", fixture.path, name, group)
				}
			}
		}
	}
}
