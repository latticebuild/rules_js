package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/latticebuild/rules_js/go/filetree"
)

// Manifest is the JSON a build action writes for its runner. It describes one
// command: node Scripts... Args..., where each script is a tree path and each
// argument is literal.
type Manifest struct {
	Executable string            `json:"executable,omitempty"`
	Root       string            `json:"root"`
	Files      [][2]string       `json:"files"`
	InputList  string            `json:"input_list"`
	Links      [][2]string       `json:"links"`
	Cwd        string            `json:"cwd"`
	Scripts    []string          `json:"scripts"`
	Args       []string          `json:"args"`
	Env        map[string]string `json:"env"`
	// StatusFile is Bazel's stable workspace status file, present when an
	// environment value names a {STABLE_KEY} of a stamped build.
	StatusFile string      `json:"status_file,omitempty"`
	Outputs    [][2]string `json:"outputs"`
}

// LoadManifest reads a staging manifest from path.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Stage lays out the tree described by m under execroot.
// Independent copies run concurrently; leaf file I/O is bounded.
func Stage(execroot string, m Manifest) error {
	execroot, err := filepath.Abs(execroot)
	if err != nil {
		return err
	}
	execroot, err = filetree.RealPath(execroot)
	if err != nil {
		return err
	}
	root, err := filetree.Path(execroot, m.Root)
	if err != nil {
		return err
	}
	if err := filetree.CheckParents(execroot, root); err != nil {
		return err
	}
	if err := validateLayout(root, m); err != nil {
		return err
	}
	limit, protected, err := inputCopier(execroot, root, m)
	if err != nil {
		return err
	}
	if err := limit.CheckDestination(execroot, root, protected...); err != nil {
		return err
	}
	return stageAt(execroot, root, m, limit, false)
}

func inputCopier(execroot, root string, m Manifest) (*filetree.Copier, []string, error) {
	if len(m.Files) > 1_000_000 {
		return nil, nil, fmt.Errorf("input declarations exceed filesystem work limit")
	}
	limit := filetree.NewCopier(nil)
	inputs := make([]string, len(m.Files))
	type declaration struct {
		logical, physical string
		regular           bool
	}
	declarations := make([]declaration, len(m.Files))
	var resolutions group
	for i, pair := range m.Files {
		inputs[i] = pair[1]
		resolutions.do(func() error {
			input, err := filetree.Path(execroot, pair[1])
			if err != nil {
				return err
			}
			target, err := filetree.Path(root, pair[0])
			if err != nil {
				return err
			}
			if filetree.PathKey(input) == filetree.PathKey(target) {
				return fmt.Errorf("input is also its staging destination")
			}
			physical, err := filetree.RealPath(input)
			if err != nil {
				return err
			}
			info, err := os.Stat(physical)
			if err != nil {
				return err
			}
			declarations[i] = declaration{input, physical, info.Mode().IsRegular()}
			return nil
		})
	}
	if err := resolutions.wait(); err != nil {
		return nil, nil, err
	}
	for _, d := range declarations {
		limit.DeclareRoot(d.physical)
		if d.regular {
			limit.DeclareFile(d.logical, d.physical)
		}
	}
	if err := limit.ReadInputs(execroot, m.InputList, inputs); err != nil {
		return nil, nil, err
	}
	protected := []string{}
	if m.InputList != "" {
		list, err := filetree.Path(execroot, m.InputList)
		if err != nil {
			return nil, nil, err
		}
		physical, err := filetree.RealPath(list)
		if err != nil {
			return nil, nil, err
		}
		protected = append(protected, physical)
	}
	return limit, protected, nil
}

// stageAt copies a validated layout into root.
func stageAt(execroot, root string, m Manifest, limit *filetree.Copier, fresh bool) error {
	if info, err := os.Lstat(root); err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("scratch root is a symlink or reparse point: %s", root)
	}
	var files group
	seenFiles := map[string]bool{}
	for _, pair := range m.Files {
		destination, source := pair[0], pair[1]
		canonical, _ := filetree.Path(root, destination)
		canonical = filetree.PathKey(canonical)
		if seenFiles[canonical] {
			continue
		}
		seenFiles[canonical] = true
		files.do(func() error {
			target, err := filetree.Path(root, destination)
			if err != nil {
				return err
			}
			input, err := filetree.Path(execroot, source)
			if err != nil {
				return err
			}
			var errPrepare error
			if fresh {
				errPrepare = os.MkdirAll(filepath.Dir(target), 0o755)
			} else {
				errPrepare = filetree.Prepare(root, target)
			}
			if errPrepare != nil {
				return errPrepare
			}
			return limit.Copy(input, target)
		})
	}
	if err := files.wait(); err != nil {
		return err
	}

	var links group
	seenLinks := map[string]bool{}
	for _, pair := range m.Links {
		destination, linkTarget := pair[0], pair[1]
		canonical, _ := filetree.Path(root, destination)
		canonical = filetree.PathKey(canonical)
		if seenLinks[canonical] {
			continue
		}
		seenLinks[canonical] = true
		links.do(func() error {
			target, err := filetree.Path(root, destination)
			if err != nil {
				return err
			}
			var errPrepare error
			if fresh {
				errPrepare = os.MkdirAll(filepath.Dir(target), 0o755)
			} else {
				errPrepare = filetree.Prepare(root, target)
			}
			if errPrepare != nil {
				return errPrepare
			}
			return filetree.DirSymlink(linkTarget, target)
		})
	}
	return links.wait()
}

// Validate before touching any destination, so overlap errors cannot race copies.
func validateLayout(root string, m Manifest) error {
	paths := map[string]string{}
	add := func(destination, identity string) error {
		target, err := filetree.Path(root, destination)
		if err != nil {
			return err
		}
		target = filetree.PathKey(target)
		if previous, ok := paths[target]; ok {
			if previous != identity {
				return fmt.Errorf("conflicting destinations at %s", destination)
			}
			return nil
		}
		paths[target] = identity
		return nil
	}
	for _, pair := range m.Files {
		if err := add(pair[0], "file:"+filetree.PathKey(filepath.Clean(pair[1]))); err != nil {
			return err
		}
	}
	for _, pair := range m.Links {
		target, err := filetree.Path(root, pair[0])
		if err != nil {
			return err
		}
		// Link targets are slash-separated. A leading slash is absolute on Unix
		// and drive-rooted on Windows, where filepath.IsAbs does not treat it as
		// absolute.
		if strings.HasPrefix(pair[1], "/") || strings.ContainsAny(pair[1], "\\:") {
			return fmt.Errorf("link %s must be relative", pair[0])
		}
		resolved := filepath.Clean(filepath.Join(filepath.Dir(target), filepath.FromSlash(pair[1])))
		if !filetree.Within(root, resolved) {
			return fmt.Errorf("link %s leaves the tree", pair[0])
		}
		if err := add(pair[0], "link:"+filetree.PathKey(resolved)); err != nil {
			return err
		}
	}
	for name := range paths {
		for parent := filepath.Dir(name); parent != root && parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
			if _, ok := paths[parent]; ok {
				return fmt.Errorf("overlapping destinations %s and %s", parent, name)
			}
		}
	}
	return nil
}

type group struct{ tasks []func() error }

func (g *group) do(fn func() error) { g.tasks = append(g.tasks, fn) }

func (g *group) wait() error {
	jobs := make(chan func() error)
	var wg sync.WaitGroup
	var once sync.Once
	var first error
	for range min(filetree.Parallelism(), len(g.tasks)) {
		wg.Go(func() {
			for fn := range jobs {
				if err := fn(); err != nil {
					once.Do(func() { first = err })
				}
			}
		})
	}
	for _, fn := range g.tasks {
		jobs <- fn
	}
	close(jobs)
	wg.Wait()
	return first
}
