//go:build darwin

package capsule

import "golang.org/x/sys/unix"

// RenameDirectoryNoReplace atomically publishes a staged directory only when
// the destination is still absent.
func RenameDirectoryNoReplace(source, destination string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, source, unix.AT_FDCWD, destination, unix.RENAME_EXCL)
}
