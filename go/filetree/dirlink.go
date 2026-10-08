package filetree

import (
	"fmt"
	"os"
	"path/filepath"
)

// DirLink links an existing native directory without requiring Windows symbolic
// link privileges. It never replaces an existing destination.
func DirLink(target, link string) error {
	if !filepath.IsAbs(target) || !filepath.IsAbs(link) {
		return fmt.Errorf("directory link requires absolute target and destination paths")
	}
	target, err := RealPath(target)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("directory link target %s is not a directory", target)
	}
	return dirLink(target, link)
}
