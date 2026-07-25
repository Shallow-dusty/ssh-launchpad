//go:build !windows

package launchpad

import (
	"context"
	"os"
)

func detectAdministrator(_ context.Context, _ Platform) bool {
	return os.Geteuid() == 0
}
