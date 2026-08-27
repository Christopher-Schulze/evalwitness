//go:build windows

package capsule

import "golang.org/x/sys/windows"

// RenameDirectoryNoReplace atomically publishes a staged directory only when
// the destination is still absent.
func RenameDirectoryNoReplace(source, destination string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFile(from, to)
}
