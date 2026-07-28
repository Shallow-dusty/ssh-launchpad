package main

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsInstallerCannotRecursivelyDeleteInstallDirectory(t *testing.T) {
	data, err := os.ReadFile("packaging/windows/installer/project.nsi")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, forbidden := range []string{
		"MUI_PAGE_DIRECTORY",
		"RMDir /r $INSTDIR",
		`RMDir /r "$INSTDIR"`,
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("installer retains unsafe custom-directory deletion: %s", forbidden)
		}
	}
	for _, required := range []string{
		`Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"`,
		`Delete "$INSTDIR\.ssh-launchpad-install"`,
		`RMDir "$INSTDIR"`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("installer is missing bounded uninstall step: %s", required)
		}
	}
}
