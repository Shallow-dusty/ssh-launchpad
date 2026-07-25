//go:build !windows

package elevation

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func openExistingWriteFile(path string, appendMode bool) (*os.File, error) {
	flags := unix.O_WRONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	if appendMode {
		flags |= unix.O_APPEND
	} else {
		flags |= unix.O_TRUNC
	}
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.New("elevation protocol path must be a regular file")
	}
	return file, nil
}
