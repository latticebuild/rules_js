package filetree

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Path(root, destination string) (string, error) {
	if filepath.IsAbs(destination) || strings.ContainsAny(destination, "\\:") {
		return "", fmt.Errorf("tree path %s leaves the tree", destination)
	}
	target := filepath.Join(root, filepath.FromSlash(destination))
	target, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("tree path %s leaves the tree", destination)
	}
	return target, nil
}

func Prepare(root, target string) error {
	if err := CheckParents(root, target); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.RemoveAll(target)
}

// checkParents refuses a target outside root or under an aliased directory.
// It walks target's path relative to root, so it ends at root however the two
// spellings differ, as with Windows separators or case.
func CheckParents(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("path %s leaves the tree %s", target, root)
	}
	for directory := filepath.Dir(rel); directory != "."; directory = filepath.Dir(directory) {
		info, err := os.Lstat(filepath.Join(root, directory))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return fmt.Errorf("%s is a symlink or reparse point; the tree cannot write under it", directory)
		}
	}
	return nil
}

func PathKey(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func Within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}
