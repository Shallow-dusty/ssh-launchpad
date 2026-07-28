//go:build !windows

package launchpad

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func openJournalRead(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	if int(stat.Uid) != os.Geteuid() || stat.Mode&0o022 != 0 {
		_ = unix.Close(fd)
		return nil, errors.New("rollback journal must be owned by the current user and not writable by group or others")
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.New("rollback journal must be a regular non-symlink file")
	}
	return file, nil
}
