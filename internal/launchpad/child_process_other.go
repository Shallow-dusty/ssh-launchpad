//go:build !windows

package launchpad

import "os/exec"

func configureChildProcess(_ *exec.Cmd) {}
