//go:build !windows && !darwin && !linux

package filetree

import "path/filepath"

// RealPath resolves file and directory aliases.
func RealPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
