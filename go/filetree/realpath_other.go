//go:build !windows

package filetree

import "path/filepath"

// RealPath resolves file and directory aliases, including Windows junctions.
func RealPath(directory string) (string, error) {
	return filepath.EvalSymlinks(directory)
}
