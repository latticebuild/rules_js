package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/latticebuild/rules_js/go/filetree"
)

// filterDirectory publishes only runtime files from a declared directory input.
func filterDirectory(execroot, source, destination string, inputList ...string) error {
	from, err := filetree.Path(execroot, source)
	if err != nil {
		return err
	}
	to, err := filetree.Path(execroot, destination)
	if err != nil {
		return err
	}
	if filetree.Within(from, to) || filetree.Within(to, from) {
		return errors.New("filter input and output overlap")
	}
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("filter input must be a directory")
	}
	limit := filetree.NewCopier(runtimeEntry)
	protected := []string{}
	for _, list := range inputList {
		if err := limit.ReadInputs(execroot, list, []string{source}); err != nil {
			return err
		}
		name, err := filetree.Path(execroot, list)
		if err != nil {
			return err
		}
		physical, err := filetree.RealPath(name)
		if err != nil {
			return err
		}
		protected = append(protected, physical)
	}
	physical, err := filetree.RealPath(from)
	if err != nil {
		return err
	}
	limit.DeclareRoot(physical)
	if err := limit.CheckDestination(execroot, to, protected...); err != nil {
		return err
	}
	if err := filetree.Prepare(execroot, to); err != nil {
		return err
	}
	return limit.Copy(from, to)
}

func runtimeEntry(path string, directory bool) bool {
	if directory {
		return filepath.Base(path) != "@types" || filepath.Base(filepath.Dir(path)) != "node_modules"
	}
	if strings.Contains("/"+filepath.ToSlash(path), "/node_modules/@types/") {
		return false
	}
	for _, suffix := range []string{".d.ts", ".d.mts", ".d.cts", ".map", ".tsbuildinfo"} {
		if strings.HasSuffix(path, suffix) {
			return false
		}
	}
	return true
}
