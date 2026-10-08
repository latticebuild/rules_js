//go:build darwin || linux

package filetree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Resolve through a metadata handle rather than walking every path component
// in userspace. Relative paths keep EvalSymlinks' existing result spelling.
func resolveMetadataPath(path string, open func(string) (*os.File, error), physical func(*os.File) (string, error), regularOnly bool) (result string, err error) {
	if !filepath.IsAbs(path) {
		return filepath.EvalSymlinks(path)
	}
	var original os.FileInfo
	if regularOnly {
		original, err = os.Stat(path)
		if err != nil {
			return "", err
		}
		// Darwin event-only opening still calls the filesystem's open callback.
		// FIFO, socket and device paths must stay on metadata-only resolution.
		if !original.IsDir() && !original.Mode().IsRegular() {
			return filepath.EvalSymlinks(path)
		}
	}
	file, err := open(path)
	if err != nil {
		// Permission to resolve a name does not require permission to open it.
		return filepath.EvalSymlinks(path)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	held, err := file.Stat()
	if err != nil {
		return "", err
	}
	if original != nil && (!os.SameFile(original, held) || (!held.IsDir() && !held.Mode().IsRegular())) {
		return "", fmt.Errorf("input %s changed during path resolution", path)
	}
	result, err = physical(file)
	if err != nil {
		// Native lookup can be unavailable, for example without Linux procfs.
		// Verify this fallback against the handle; File.Name is not authority.
		result, err = filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
	}
	if !filepath.IsAbs(result) || filepath.Clean(result) != result {
		return "", fmt.Errorf("invalid physical path for %s", path)
	}
	current, err := os.Stat(result)
	if err != nil {
		return "", err
	}
	if !os.SameFile(held, current) {
		return "", fmt.Errorf("resolved input %s changed", path)
	}
	return result, nil
}
