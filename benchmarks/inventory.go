package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type payloadFile struct {
	Path   string
	SHA256 string
	Size   int64
}

func (f *payloadFile) UnmarshalJSON(data []byte) error {
	var row []json.RawMessage
	if err := json.Unmarshal(data, &row); err != nil || len(row) != 3 {
		return errors.New("payload entry must contain path, digest and size")
	}
	for i, value := range []any{&f.Path, &f.SHA256, &f.Size} {
		if err := json.Unmarshal(row[i], value); err != nil {
			return err
		}
	}
	if f.Path == "." || path.Clean(f.Path) != f.Path || !filepath.IsLocal(filepath.FromSlash(f.Path)) || strings.ContainsAny(f.Path, "\\:") || len(f.SHA256) != 64 || f.Size < 0 {
		return errors.New("invalid payload entry")
	}
	return nil
}

type packageInstance struct {
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	Bindings [][]string    `json:"bindings"`
	Files    []payloadFile `json:"files"`
}

type inventory struct {
	Node      string            `json:"node"`
	Instances []packageInstance `json:"instances"`
}

type excludedFile struct {
	Package string `json:"package"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type inventoryParity struct {
	Instances    int            `json:"packageInstances"`
	CommonFiles  int            `json:"commonPayloadFiles"`
	CommonBytes  int64          `json:"commonPayloadBytes"`
	LatticeFiles int            `json:"latticePayloadFiles"`
	AspectFiles  int            `json:"aspectPayloadFiles"`
	Excluded     []excludedFile `json:"aspectDefaultExclusions"`
}

func compareInventories(lattice, aspect *backend) (inventoryParity, error) {
	values := make([]inventory, 2)
	for i, b := range []*backend{lattice, aspect} {
		file := filepath.Join(b.directory, "bazel-bin", "many", "inventory.json")
		info, err := os.Stat(file)
		if err != nil || info.Size() > 64<<20 {
			return inventoryParity{}, errors.New("missing or oversized package inventory")
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return inventoryParity{}, err
		}
		if err := json.Unmarshal(data, &values[i]); err != nil {
			return inventoryParity{}, err
		}
		if err := os.WriteFile(filepath.Join(b.results, b.name+"-inventory.json"), data, 0o644); err != nil {
			return inventoryParity{}, err
		}
	}
	return inventoryComparison(values[0], values[1])
}

func instanceKey(instance packageInstance) string {
	key, _ := json.Marshal([]any{instance.Name, instance.Version, instance.Bindings})
	return string(key)
}

func inventoryComparison(lattice, aspect inventory) (inventoryParity, error) {
	r := inventoryParity{Instances: len(lattice.Instances)}
	if lattice.Node != "v26.8.2" || aspect.Node != lattice.Node || len(lattice.Instances) < 40 || len(lattice.Instances) != len(aspect.Instances) {
		return r, errors.New("Node version or physical package instance count differs")
	}
	// Keep every physical peer instance. Sorting does not collapse duplicates.
	left, right := slices.Clone(lattice.Instances), slices.Clone(aspect.Instances)
	less := func(a, b packageInstance) int { return strings.Compare(instanceKey(a), instanceKey(b)) }
	slices.SortStableFunc(left, less)
	slices.SortStableFunc(right, less)
	for i, l := range left {
		a := right[i]
		if instanceKey(l) != instanceKey(a) {
			return r, fmt.Errorf("package or dependency/peer binding differs for %s@%s", l.Name, l.Version)
		}
		files := map[string]payloadFile{}
		for _, file := range l.Files {
			if _, exists := files[file.Path]; exists {
				return r, errors.New("duplicate package payload path")
			}
			files[file.Path] = file
			r.LatticeFiles++
		}
		seen := map[string]bool{}
		for _, file := range a.Files {
			if seen[file.Path] {
				return r, errors.New("duplicate Aspect package payload path")
			}
			seen[file.Path] = true
			match, exists := files[file.Path]
			if !exists || match != file {
				return r, fmt.Errorf("runtime payload differs: %s@%s/%s", l.Name, l.Version, file.Path)
			}
			delete(files, file.Path)
			r.AspectFiles++
			r.CommonFiles++
			r.CommonBytes += file.Size
		}
		for _, file := range files {
			if !aspectBasicExclusion(file.Path) {
				return r, fmt.Errorf("unexpected missing Aspect payload: %s@%s/%s", l.Name, l.Version, file.Path)
			}
			r.Excluded = append(r.Excluded, excludedFile{l.Name + "@" + l.Version, file.Path, file.SHA256, file.Size})
		}
	}
	slices.SortFunc(r.Excluded, func(a, b excludedFile) int {
		return strings.Compare(a.Package+"/"+a.Path, b.Package+"/"+b.Path)
	})
	return r, nil
}

// The pinned Aspect 3.5.0 basic preset excludes these package-root files.
// https://github.com/aspect-build/rules_js/blob/v3.5.0/npm/private/exclude_package_contents_presets.bzl
func aspectBasicExclusion(name string) bool {
	if strings.Contains(name, "/") {
		return false
	}
	if strings.HasSuffix(name, ".md") || (strings.HasPrefix(name, ".") && strings.HasSuffix(name, "ignore")) {
		return true
	}
	return slices.Contains([]string{
		"Makefile", "Gulpfile.js", "Gruntfile.js", "appveyor.yml", "circle.yml",
		"codeship-services.yml", "codeship-steps.yml", "wercker.yml", ".tern-project",
		".gitattributes", ".editorconfig", ".eslintrc", ".jshintrc", ".flowconfig",
		".documentup.json", ".yarn-metadata.json", ".travis.yml",
	}, name)
}
