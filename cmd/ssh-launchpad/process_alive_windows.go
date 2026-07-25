//go:build windows

package main

import (
	"errors"
	"math"

	"golang.org/x/sys/windows"
)

func processRunning(pid int) bool {
	if pid <= 0 || uint64(pid) > math.MaxUint32 {
		return false
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return true
	}
	return status == uint32(windows.WAIT_TIMEOUT)
}
