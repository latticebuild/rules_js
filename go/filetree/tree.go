package filetree

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
)

func (limit *Copier) Copy(from, to string) error {
	return copyEntry(from, to, limit, map[string]bool{}, 0)
}

func copyEntry(from, to string, limit *Copier, ancestors map[string]bool, depth int) error {
	if depth > 128 || limit.entries.Add(1) > 1_000_000 {
		return fmt.Errorf("copy exceeds filesystem work limit at %s", from)
	}
	resolved, err := RealPath(from)
	if err != nil {
		return err
	}
	if len(limit.roots) > 0 {
		declared := false
		for current := resolved; ; current = filepath.Dir(current) {
			if _, ok := limit.roots[PathKey(current)]; ok {
				declared = true
				break
			}
			if current == filepath.Dir(current) {
				break
			}
		}
		if !declared {
			return fmt.Errorf("input symlink %s resolves outside declared files and directories", from)
		}
	}
	if ancestors[PathKey(resolved)] {
		return fmt.Errorf("filesystem symlink cycle at %s", from)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.Mkdir(to, 0o755); err != nil {
			return err
		}
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return err
		}
		ancestors[PathKey(resolved)] = true
		defer delete(ancestors, PathKey(resolved))
		for _, entry := range entries {
			if limit.keep != nil && entry.IsDir() && !limit.keep(filepath.Join(resolved, entry.Name()), true) {
				continue
			}
			if err := copyEntry(filepath.Join(resolved, entry.Name()), filepath.Join(to, entry.Name()), limit, ancestors, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("input %s is not a regular file or directory", from)
	}
	if limit.keep != nil && !limit.keep(from, false) {
		return nil
	}
	if limit.bytes.Add(info.Size()) > 64<<30 {
		return errors.New("copy exceeds 64 GiB filesystem work limit")
	}
	return limit.run(func() error {
		if err := copyFile(resolved, to, cloneFile); err != nil {
			return err
		}
		return os.Chmod(to, (info.Mode() | 0o200).Perm())
	})
}

type Copier struct {
	sem     chan struct{}
	entries atomic.Int64
	bytes   atomic.Int64
	keep    func(path string, directory bool) bool
	roots   map[string]struct{}
}

// NewCopier shares filesystem work limits across all copies in one operation.
// keep can prune physical directories and exclude files; nil copies every entry.
func NewCopier(keep func(path string, directory bool) bool) *Copier {
	return &Copier{sem: make(chan struct{}, Parallelism()), roots: map[string]struct{}{}, keep: keep}
}

// CheckDestination refuses writes that overlap canonical input storage. Extra
// protected paths are read-only tools or manifests, not authorized copy roots.
func (l *Copier) CheckDestination(root, target string, protected ...string) error {
	if err := CheckParents(root, target); err != nil {
		return err
	}
	existing := target
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return err
		}
		if existing == root || existing == filepath.Dir(existing) {
			return fmt.Errorf("destination root %s does not exist", root)
		}
		existing = filepath.Dir(existing)
	}
	physical, err := RealPath(existing)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(existing, target)
	if err != nil {
		return err
	}
	physical = PathKey(filepath.Join(physical, relative))
	for input := range l.roots {
		if Within(input, physical) || Within(physical, input) {
			return fmt.Errorf("destination %s overlaps input storage %s", target, input)
		}
	}
	for _, input := range protected {
		input = PathKey(input)
		if Within(input, physical) || Within(physical, input) {
			return fmt.Errorf("destination %s overlaps protected input %s", target, input)
		}
	}
	return nil
}

// Directory artifacts may be sandbox directories with symlink leaves. Only
// Bazel's expanded input inventory can authorize their backing file locations.
func (l *Copier) ReadInputs(execroot, list string, inputs []string) error {
	if list == "" {
		return nil
	}
	declared := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		path, err := Path(execroot, input)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		declared[PathKey(path)] = info.IsDir()
	}
	name, err := Path(execroot, list)
	if err != nil {
		return err
	}
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for count := 0; scanner.Scan(); count++ {
		if count > 1_000_000 {
			return errors.New("input inventory exceeds filesystem work limit")
		}
		var input string
		if err := json.Unmarshal(scanner.Bytes(), &input); err != nil {
			return err
		}
		path, err := Path(execroot, input)
		if err != nil {
			return err
		}
		member := false
		for current := path; ; current = filepath.Dir(current) {
			if directory, exists := declared[PathKey(current)]; exists && (current == path || directory) {
				member = true
				break
			}
			if current == filepath.Dir(current) {
				break
			}
		}
		if !member {
			return fmt.Errorf("input inventory path %s is not declared", input)
		}
		physical, err := RealPath(path)
		if err != nil {
			return err
		}
		l.roots[PathKey(physical)] = struct{}{}
	}
	return scanner.Err()
}

func (l *Copier) run(fn func() error) error {
	l.sem <- struct{}{}
	defer func() { <-l.sem }()
	return fn()
}

func Parallelism() int {
	n := runtime.GOMAXPROCS(0) * 4
	if n < 4 {
		n = 4
	}
	if n > 64 {
		n = 64
	}
	return n
}

// DeclareRoot authorizes a filesystem location before concurrent copying starts.
// Input callers resolve symlinks first; output callers validate their own roots.
func (l *Copier) DeclareRoot(path string) { l.roots[PathKey(path)] = struct{}{} }
