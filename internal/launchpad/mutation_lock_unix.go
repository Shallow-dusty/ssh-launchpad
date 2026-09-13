//go:build !windows

package launchpad

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func acquireMutationLock() (func(), error) {
	return acquireMutationLockAt("/tmp/ssh-launchpad-mutation.lock")
}
func acquireMutationLockAt(path string) (func(), error) {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0644)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("invalid mutation lock file")
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("another SSH Launchpad Apply or Rollback is running; wait for it to finish")
	}
	return func() { _ = unix.Flock(fd, unix.LOCK_UN); _ = f.Close() }, nil
}
