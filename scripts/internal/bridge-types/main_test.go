package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateActualBridgeIsDeterministicAndComplete(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	first, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate(root)
	if err != nil || first != second {
		t.Fatalf("non-deterministic output: %v", err)
	}
	for _, expected := range []string{
		"export interface DesktopBridge", "Run(request: WireDesktopRequest): Promise<WireReport>",
		"ValidatePublicKey(value: string): Promise<void>", "DismissElevatedJob(id: string): Promise<void>",
		"schemaVersion: number", "binaryExists?: boolean", "managedScopes?: Array<string>",
		"thirdPartyBroadRules?: Array<string>", "sshPolicyNotInitialized?: boolean",
		"publicKeys: Array<string> | null", "labels: Record<string, string> | null",
		"export type WireJobState", "\"rollback\"", "platform: WirePlatform",
		"export const wireDefaultProfile", "\"preventSelfCut\": true", "\"mirrorBaseUrl\": \"\"", "satisfies WireProfile",
	} {
		if !strings.Contains(first, expected) {
			t.Errorf("missing contract: %s", expected)
		}
	}
}

func TestWireOptionalityAndUnsupportedShapes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "launchpad"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := "package main\n" +
		"type App struct{}\n" +
		"type Shape struct { Required *bool `json:\"required\"`; Optional *bool `json:\"optional,omitempty\"`; Values []string `json:\"values\"`; Hidden string `json:\"-\"` }\n" +
		"func (*App) Shape() (Shape, error) { return Shape{}, nil }\n" +
		"func (*App) PrivateOnly() error { return nil }\n"
	path := filepath.Join(root, "app.go")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"required: boolean | null", "optional?: boolean", "values: Array<string> | null", "PrivateOnly(): Promise<void>"} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %s in %s", expected, text)
		}
	}
	if strings.Contains(text, "Hidden") {
		t.Fatal("ignored field leaked into wire contract")
	}
	fixture = strings.Replace(fixture, "Required *bool", "Required chan bool", 1)
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := generate(root); err == nil {
		t.Fatal("unsupported wire shape silently accepted")
	}
}
