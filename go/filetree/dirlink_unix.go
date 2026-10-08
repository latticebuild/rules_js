//go:build !windows

package filetree

import "os"

func dirLink(target, link string) error {
	return os.Symlink(target, link)
}
