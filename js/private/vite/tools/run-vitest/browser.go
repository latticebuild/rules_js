package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/latticebuild/rules_js/go/filetree"
)

// Keep the declared native payload at a stable path while Playwright validates
// each private installation, writing DEPENDENCIES_VALIDATED beside its private
// marker. Unix symlinks and Windows junctions provide the same runtime layout.
func prepareBrowser(layoutFile string) (string, error) {
	contents, err := os.ReadFile(layoutFile)
	if err != nil {
		return "", err
	}
	var layout struct {
		Marker string   `json:"marker"`
		Files  []string `json:"files"`
	}
	if err := json.Unmarshal(contents, &layout); err != nil {
		return "", err
	}
	if !browserPath(layout.Marker) || path.Base(layout.Marker) != "INSTALLATION_COMPLETE" || len(layout.Files) == 0 {
		return "", errors.New("run-vitest: invalid declared browser layout")
	}
	installation := path.Base(path.Dir(layout.Marker))
	name, revision, found := strings.Cut(installation, "-")
	if !found || name != "chromium" || revision == "" || strings.Trim(revision, "0123456789") != "" || path.Base(path.Dir(path.Dir(layout.Marker))) != ".playwright" {
		return "", errors.New("run-vitest: invalid declared browser installation")
	}
	r, err := runfiles.New()
	if err != nil {
		return "", err
	}
	marker, err := r.Rlocation(layout.Marker)
	if err != nil {
		return "", err
	}
	marker, err = filetree.RealPath(marker)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(marker)
	if err != nil {
		return "", err
	}
	source := filepath.Dir(marker)
	if !filepath.IsAbs(marker) || !info.Mode().IsRegular() || info.Size() != 0 || filepath.Base(marker) != "INSTALLATION_COMPLETE" || filepath.Base(source) != installation || filepath.Base(filepath.Dir(source)) != ".playwright" {
		return "", errors.New("run-vitest: browser marker does not resolve to its declared installation")
	}
	files, directories, roots := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, file := range layout.Files {
		key := filetree.PathKey(filepath.FromSlash(file))
		if !browserPath(file) || file == "INSTALLATION_COMPLETE" || file == "DEPENDENCIES_VALIDATED" || files[key] {
			return "", fmt.Errorf("run-vitest: invalid browser payload entry %q", file)
		}
		files[key] = true
		for directory := path.Dir(file); directory != "."; directory = path.Dir(directory) {
			directories[filetree.PathKey(filepath.FromSlash(directory))] = true
		}
		root, _, _ := strings.Cut(file, "/")
		roots[root] = true
		resolved, err := r.Rlocation(path.Dir(layout.Marker) + "/" + file)
		if err != nil {
			return "", err
		}
		resolved, err = filetree.RealPath(resolved)
		if err != nil {
			return "", err
		}
		expected, err := filetree.RealPath(filepath.Join(source, filepath.FromSlash(file)))
		if err != nil {
			return "", err
		}
		if !filetree.Within(source, resolved) || filetree.PathKey(resolved) != filetree.PathKey(expected) {
			return "", fmt.Errorf("run-vitest: browser payload %s leaves its declared installation", file)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("run-vitest: browser payload %s must be a regular file", file)
		}
	}
	var payloads []string
	for root := range roots {
		payloads = append(payloads, root)
		info, err := os.Lstat(filepath.Join(source, root))
		if err != nil {
			return "", err
		}
		if info.Mode().Type() != fs.ModeDir {
			return "", fmt.Errorf("run-vitest: native payload %s must be a real directory", root)
		}
		if err := filepath.WalkDir(filepath.Join(source, root), func(file string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(source, file)
			if err != nil {
				return err
			}
			if entry.IsDir() && directories[filetree.PathKey(relative)] {
				return nil
			}
			// Framework aliases must resolve to declared entries in this installation.
			// Walk the physical target separately instead of recursing through links.
			if entry.Type()&fs.ModeSymlink != 0 {
				resolved, err := filetree.RealPath(file)
				if err != nil {
					return err
				}
				if !filetree.Within(source, resolved) {
					return fmt.Errorf("run-vitest: browser link %s leaves its declared installation", relative)
				}
				target, err := filepath.Rel(source, resolved)
				if err != nil {
					return err
				}
				info, err := os.Stat(resolved)
				if err != nil {
					return err
				}
				if info.IsDir() && filetree.Within(resolved, filepath.Dir(file)) {
					return fmt.Errorf("run-vitest: browser link %s creates an ancestor cycle", relative)
				}
				if info.IsDir() && directories[filetree.PathKey(relative)] && directories[filetree.PathKey(target)] {
					return nil
				}
				if info.Mode().IsRegular() && files[filetree.PathKey(relative)] && files[filetree.PathKey(target)] {
					return nil
				}
			}
			if !entry.Type().IsRegular() || !files[filetree.PathKey(relative)] {
				return fmt.Errorf("run-vitest: undeclared or linked browser payload %s", relative)
			}
			return nil
		}); err != nil {
			return "", err
		}
	}
	// Bound owns this fresh writable tree and removes it after the process exits.
	bundle := os.Getenv("BOUND_ROOT")
	relative, err := filepath.Rel(bundle, layoutFile)
	if !filepath.IsAbs(bundle) || err != nil || relative == "." || !filepath.IsLocal(relative) {
		return "", errors.New("run-vitest: browser layout must belong to the private bound tree")
	}
	if filetree.Within(bundle, source) {
		return "", errors.New("run-vitest: native browser payload must be outside the private bound tree")
	}
	root := filepath.Join(bundle, "tree", ".playwright")
	if err := filetree.CheckParents(bundle, root); err != nil {
		return "", err
	}
	tree := filepath.Dir(root)
	if err := os.Mkdir(tree, 0o700); err != nil && !os.IsExist(err) {
		return "", err
	}
	info, err = os.Lstat(tree)
	if err != nil {
		return "", err
	}
	if info.Mode().Type() != fs.ModeDir {
		return "", errors.New("run-vitest: private application tree must be a real directory")
	}
	// Refuse every existing root, including Windows junctions. Following an
	// alias here would publish writable metadata outside the owned bundle.
	if err := os.Mkdir(root, 0o700); err != nil {
		return "", err
	}
	destination := filepath.Join(root, installation)
	if err := os.Mkdir(destination, 0o700); err != nil {
		return "", err
	}
	slices.Sort(payloads)
	for _, payload := range payloads {
		if err := filetree.DirLink(filepath.Join(source, payload), filepath.Join(destination, payload)); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(destination, "INSTALLATION_COMPLETE"), nil, 0o600); err != nil {
		return "", err
	}
	return root, nil
}

func browserPath(value string) bool {
	return value != "" && value != "." && !path.IsAbs(value) && !strings.ContainsAny(value, "\\:\x00") && path.Clean(value) == value && !strings.HasPrefix(value, "../")
}
