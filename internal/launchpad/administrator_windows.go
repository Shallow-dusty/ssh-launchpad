//go:build windows

package launchpad

import (
	"context"

	"golang.org/x/sys/windows"
)

func detectAdministrator(_ context.Context, _ Platform) bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
