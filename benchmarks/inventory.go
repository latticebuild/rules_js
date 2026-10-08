package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
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
	if _, err := hex.DecodeString(f.SHA256); err != nil {
		return errors.New("invalid payload SHA-256")
	}
	return nil
}

type packageInstance struct {
	Name              string        `json:"name"`
	Version           string        `json:"version"`
	Bindings          [][]string    `json:"bindings"`
	Files             []payloadFile `json:"files"`
	LayoutDirectories []string      `json:"layoutDirectories"`
	OS                []string      `json:"os"`
	CPU               []string      `json:"cpu"`
	Libc              []string      `json:"libc"`
	DeclarationCounts [3]int        `json:"declarationCounts"`
}

type inventory struct {
	Node                  string            `json:"node"`
	Platform              string            `json:"platform"`
	Architecture          string            `json:"architecture"`
	Libc                  string            `json:"libc"`
	LibcVersion           string            `json:"libcVersion"`
	Instances             []packageInstance `json:"instances"`
	IncompatibleInstances []packageInstance `json:"incompatibleInstances"`
	ExecArgv              []string          `json:"execArgv"`
	NodeOptions           string            `json:"nodeOptions"`
}

type excludedFile struct {
	Package string `json:"package"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type inventoryParity struct {
	Instances         int                     `json:"packageInstances"`
	CommonFiles       int                     `json:"commonPayloadFiles"`
	CommonBytes       int64                   `json:"commonPayloadBytes"`
	LatticeFiles      int                     `json:"latticePayloadFiles"`
	AspectFiles       int                     `json:"aspectPayloadFiles"`
	Excluded          []excludedFile          `json:"aspectDefaultExclusions"`
	LayoutDirectories map[string]int          `json:"dependencyLayoutDirectories"`
	NodeDefaults      map[string]nodeDefaults `json:"backendNodeDefaults"`
	PhysicalInstances map[string]int          `json:"physicalPackageInstances"`
	ABIExtras         []packageInstance       `json:"aspectIncompatibleLibcInputs"`
	ABIExtraFiles     int                     `json:"aspectIncompatibleLibcFiles"`
	ABIExtraBytes     int64                   `json:"aspectIncompatibleLibcBytes"`
	Libc              string                  `json:"libc"`
	LibcVersion       string                  `json:"libcVersion"`
}

type nodeDefaults struct {
	ExecArgv    []string `json:"execArgv"`
	NodeOptions string   `json:"nodeOptions"`
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
		platform, architecture := runtime.GOOS, runtime.GOARCH
		if platform == "windows" {
			platform = "win32"
		}
		if architecture == "amd64" {
			architecture = "x64"
		}
		if values[i].Platform != platform || values[i].Architecture != architecture {
			return inventoryParity{}, errors.New("inventory does not match the native host")
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
	r := inventoryParity{Instances: len(lattice.Instances), LayoutDirectories: map[string]int{}, NodeDefaults: map[string]nodeDefaults{"lattice": {lattice.ExecArgv, lattice.NodeOptions}, "aspect": {aspect.ExecArgv, aspect.NodeOptions}}, PhysicalInstances: map[string]int{}, Libc: lattice.Libc, LibcVersion: lattice.LibcVersion}
	if lattice.Node != "v26.8.2" || aspect.Node != lattice.Node || len(lattice.Instances) < 40 || len(lattice.Instances) != len(aspect.Instances) {
		return r, errors.New("Node version or physical package instance count differs")
	}
	if lattice.Platform != aspect.Platform || lattice.Architecture != aspect.Architecture || lattice.Libc != aspect.Libc || lattice.LibcVersion != aspect.LibcVersion {
		return r, errors.New("Node platform, architecture or libc differs")
	}
	if !slices.Contains([]string{"linux", "darwin", "win32"}, lattice.Platform) || !slices.Contains([]string{"x64", "arm64"}, lattice.Architecture) {
		return r, errors.New("unsupported inventory platform or architecture")
	}
	if lattice.Platform == "linux" {
		if (lattice.Libc != "glibc" || lattice.LibcVersion == "") && (lattice.Libc != "musl" || lattice.LibcVersion != "") {
			return r, errors.New("Linux inventory lacks positive libc evidence")
		}
	} else if lattice.Libc != "" || lattice.LibcVersion != "" {
		return r, errors.New("non-Linux inventory reports libc filtering")
	}
	if err := compareABIExtras(lattice, aspect, &r); err != nil {
		return r, err
	}
	for backend, value := range map[string]inventory{"lattice": lattice, "aspect": aspect} {
		r.PhysicalInstances[backend] = len(value.Instances) + len(value.IncompatibleInstances)
		identities := map[string]bool{}
		for _, instance := range append(slices.Clone(value.Instances), value.IncompatibleInstances...) {
			identity := instance.Name + "@" + instance.Version
			if identities[identity] {
				return r, fmt.Errorf("pinned fixture has ambiguous physical peer instances for %s", identity)
			}
			identities[identity] = true
		}
		for _, instance := range value.Instances {
			for _, directory := range instance.LayoutDirectories {
				if directory != "node_modules/" {
					return r, errors.New("unexpected ignored package payload directory")
				}
				r.LayoutDirectories[backend]++
			}
		}
		// Bindings must resolve within the compatible graph, never to an
		// optional native package excluded by the host's libc.
		for _, instance := range value.IncompatibleInstances {
			delete(identities, instance.Name+"@"+instance.Version)
		}
		for _, instance := range value.Instances {
			for _, edge := range instance.Bindings {
				if len(edge) != 3 || !slices.Contains([]string{"dependencies", "optionalDependencies", "peerDependencies"}, edge[0]) || !identities[edge[2]] {
					return r, errors.New("package binding does not resolve to a unique inventoried instance")
				}
			}
		}
	}
	// Keep every physical peer instance. Sorting does not collapse duplicates.
	left, right := slices.Clone(lattice.Instances), slices.Clone(aspect.Instances)
	less := func(a, b packageInstance) int { return strings.Compare(instanceKey(a), instanceKey(b)) }
	slices.SortStableFunc(left, less)
	slices.SortStableFunc(right, less)
	for i, l := range left {
		a := right[i]
		if instanceKey(l) != instanceKey(a) || !slices.Equal(l.OS, a.OS) || !slices.Equal(l.CPU, a.CPU) || !slices.Equal(l.Libc, a.Libc) || l.DeclarationCounts != a.DeclarationCounts {
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

// Aspect's pinned fixture links both Linux libc variants. pnpm installs only
// the host variant. Keep and disclose those extra physical inputs without
// treating an incompatible ABI as part of the common runtime graph.
func compareABIExtras(lattice, aspect inventory, r *inventoryParity) error {
	if len(lattice.IncompatibleInstances) != 0 {
		return errors.New("unexpected incompatible pnpm package")
	}
	if lattice.Platform != "linux" || lattice.Architecture != "x64" || lattice.Libc != "glibc" {
		if len(aspect.IncompatibleInstances) != 0 {
			return errors.New("unexpected incompatible Aspect package on this host")
		}
		return nil
	}
	data, err := fixtures.ReadFile("fixtures/linux-glibc-exclusions.json")
	if err != nil {
		return err
	}
	var expected []packageInstance
	if err := json.Unmarshal(data, &expected); err != nil {
		return err
	}
	if len(expected) != 4 || len(aspect.IncompatibleInstances) != len(expected) {
		return errors.New("Linux libc input delta does not match the four pinned leaves")
	}
	want := map[string]packageInstance{}
	for _, instance := range expected {
		want[instance.Name+"@"+instance.Version] = instance
	}
	for _, instance := range aspect.IncompatibleInstances {
		key := instance.Name + "@" + instance.Version
		match, exists := want[key]
		if !exists || len(instance.Bindings) != 0 || instance.DeclarationCounts != [3]int{} || len(instance.LayoutDirectories) != 0 || !slices.Equal(instance.OS, []string{"linux"}) || !slices.Equal(instance.CPU, []string{"x64"}) || !slices.Equal(instance.Libc, []string{"musl"}) {
			return fmt.Errorf("unexpected non-leaf or incompatible ABI input: %s", key)
		}
		files := map[string]payloadFile{}
		for _, file := range match.Files {
			files[file.Path] = file
		}
		if len(instance.Files) != len(files) {
			return fmt.Errorf("incompatible ABI payload count differs: %s", key)
		}
		for _, file := range instance.Files {
			if value, exists := files[file.Path]; !exists || value != file {
				return fmt.Errorf("incompatible ABI payload differs: %s/%s", key, file.Path)
			}
			delete(files, file.Path)
			r.ABIExtraFiles++
			r.ABIExtraBytes += file.Size
		}
		delete(want, key)
		r.ABIExtras = append(r.ABIExtras, instance)
	}
	return nil
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
