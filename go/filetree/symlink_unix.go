//go:build !windows

package filetree

import "os"

// dirSymlink creates a symlink at link pointing at target (always a package dir).
func DirSymlink(target, link string) error {
	return os.Symlink(target, link)
}
