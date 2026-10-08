//go:build windows

package filetree

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Node's fs.symlinkSync(..., "dir") also creates unprivileged links where
// Developer Mode allows them. x/sys/windows does not name this flag.
const symbolicLinkFlagAllowUnprivilegedCreate = 0x2

// dirSymlink creates a directory symlink at link pointing at target, a package
// directory that may not exist yet. Windows cannot follow a relative link whose
// content uses forward slashes, so target is converted as os.Symlink does.
func DirSymlink(target, link string) error {
	linkp, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	targetp, err := windows.UTF16PtrFromString(filepath.FromSlash(target))
	if err != nil {
		return err
	}
	if err := windows.CreateSymbolicLink(linkp, targetp, windows.SYMBOLIC_LINK_FLAG_DIRECTORY|symbolicLinkFlagAllowUnprivilegedCreate); err != nil {
		return &os.LinkError{Op: "symlink", Old: target, New: link, Err: err}
	}
	return nil
}
