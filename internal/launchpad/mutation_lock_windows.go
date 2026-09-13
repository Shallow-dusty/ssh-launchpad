//go:build windows

package launchpad

import (
	"errors"
	"golang.org/x/sys/windows"
	"runtime"
)

func acquireMutationLock() (func(), error) {
	// A Win32 mutex must be released by its owning OS thread.
	runtime.LockOSThread()
	name, err := windows.UTF16PtrFromString(`Global\SSHLaunchpad-System-Mutation-v1`)
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		runtime.UnlockOSThread()
		return nil, err
	}
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || (status != windows.WAIT_OBJECT_0 && status != windows.WAIT_ABANDONED) {
		windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		return nil, errors.New("another SSH Launchpad Apply or Rollback is running")
	}
	return func() { _ = windows.ReleaseMutex(handle); _ = windows.CloseHandle(handle); runtime.UnlockOSThread() }, nil
}
