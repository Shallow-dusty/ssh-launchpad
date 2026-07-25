package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageDirectoryAssignsPortableModes(t *testing.T) {
	source := t.TempDir()
	for name, content := range map[string]string{
		"ssh-launchpad":               "binary",
		"Start SSH Launchpad.command": "#!/bin/sh\n",
		"new-offline-pack.sh":         "#!/bin/sh\n",
		"README.md":                   "docs",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := packageDirectory(source, output); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressor, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressor.Close()
	reader := tar.NewReader(compressor)
	modes := map[string]int64{}
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		modes[header.Name] = header.Mode
	}
	for _, name := range []string{"ssh-launchpad", "Start SSH Launchpad.command", "new-offline-pack.sh"} {
		if modes[name] != 0o755 {
			t.Fatalf("%s mode = %o, want 755", name, modes[name])
		}
	}
	if modes["README.md"] != 0o644 {
		t.Fatalf("README.md mode = %o, want 644", modes["README.md"])
	}
}
