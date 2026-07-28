//go:build windows

package launchpad

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func openJournalRead(path string) (*os.File, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		path16,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
		info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		_ = windows.CloseHandle(handle)
		return nil, errors.New("rollback journal must be a regular non-reparse file")
	}
	security, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	owner, _, err := security.Owner()
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	current, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || owner == nil || current.User.Sid == nil || !owner.Equals(current.User.Sid) {
		_ = windows.CloseHandle(handle)
		return nil, errors.New("rollback journal must be owned by the current Windows user")
	}
	return os.NewFile(uintptr(handle), path), nil
}
