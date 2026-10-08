package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func TestFixtureLibcSelection(t *testing.T) {
	for _, test := range []struct {
		name, platform, architecture, report, libc, version string
		selected, incompatible                              int
		failure, required, none                             bool
	}{
		{"glibc", "linux", "x64", `return {header:{glibcVersionRuntime:'2.39'},sharedObjects:[]}`, "glibc", "2.39", 2, 1, false, false, false},
		{"musl", "linux", "x64", `return {header:{},sharedObjects:['/lib/ld-musl-x86_64.so.1']}`, "musl", "", 2, 1, false, false, false},
		{"unknown", "linux", "x64", `return {header:{},sharedObjects:[]}`, "", "", 0, 0, true, false, false},
		{"empty version", "linux", "x64", `return {header:{glibcVersionRuntime:''},sharedObjects:[]}`, "", "", 0, 0, true, false, false},
		{"broken header with musl", "linux", "x64", `return {header:{glibcVersionRuntime:0},sharedObjects:['/lib/ld-musl-x86_64.so.1']}`, "", "", 0, 0, true, false, false},
		{"conflicting evidence", "linux", "x64", `return {header:{glibcVersionRuntime:'2.39'},sharedObjects:['/lib/ld-musl-x86_64.so.1']}`, "", "", 0, 0, true, false, false},
		{"required incompatible", "linux", "x64", `return {header:{glibcVersionRuntime:'2.39'},sharedObjects:[]}`, "", "", 0, 0, true, true, false},
		{"other OS", "darwin", "arm64", `throw new Error('report must be lazy')`, "", "", 1, 2, false, false, false},
		{"other CPU", "linux", "arm64", `throw new Error('report must be lazy')`, "", "", 1, 2, false, false, false},
		{"no dependencies", "linux", "x64", `throw new Error('report must be lazy')`, "", "", 0, 0, false, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			action, err := fixtures.ReadFile("fixtures/action.mjs")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "action.mjs"), action, 0o644); err != nil {
				t.Fatal(err)
			}
			prelude := "Object.defineProperty(process, 'platform', {value:" + quote(test.platform) + "});\n" +
				"Object.defineProperty(process, 'arch', {value:" + quote(test.architecture) + "});\n" +
				"process.report.getReport = () => {" + test.report + "};\n"
			if err := os.WriteFile(filepath.Join(root, "prelude.cjs"), []byte(prelude), 0o644); err != nil {
				t.Fatal(err)
			}
			tool := map[string]any{"name": "tool", "version": "1.0.0", "optionalDependencies": map[string]string{"gnu": "1.0.0", "musl": "1.0.0"}}
			if test.required {
				delete(tool, "optionalDependencies")
				tool["dependencies"] = map[string]string{"musl": "1.0.0"}
			}
			for name, manifest := range map[string]any{
				"tool": tool,
				"gnu":  map[string]any{"name": "gnu", "version": "1.0.0", "os": []string{"linux"}, "cpu": []string{"x64"}, "libc": []string{"glibc"}},
				"musl": map[string]any{"name": "musl", "version": "1.0.0", "os": []string{"linux"}, "cpu": []string{"x64"}, "libc": []string{"!glibc"}},
			} {
				directory := filepath.Join(root, "node_modules", name)
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := saveJSON(filepath.Join(directory, "package.json"), manifest); err != nil {
					t.Fatal(err)
				}
			}
			dependencies := []string{"tool"}
			if test.none {
				dependencies = []string{}
			}
			if err := saveJSON(filepath.Join(root, "input.json"), map[string]any{"nonce": "test", "dependencies": dependencies}); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(fixtureNode(t), "--require", filepath.Join(root, "prelude.cjs"), filepath.Join(root, "action.mjs"), "--input", filepath.Join(root, "input.json"), "--output", filepath.Join(root, "output.json"), "--inventory")
			cmd.Dir = root
			output, err := cmd.CombinedOutput()
			if test.failure {
				if err == nil {
					t.Fatal("incompatible or unknown ABI succeeded")
				}
				if _, err := os.Stat(filepath.Join(root, "output.json")); !os.IsNotExist(err) {
					t.Fatal("failed ABI check published output")
				}
				return
			}
			if err != nil {
				t.Fatalf("fixture: %v\n%s", err, output)
			}
			data, err := os.ReadFile(filepath.Join(root, "output.json"))
			if err != nil {
				t.Fatal(err)
			}
			var value inventory
			if err := json.Unmarshal(data, &value); err != nil {
				t.Fatal(err)
			}
			if value.Libc != test.libc || value.LibcVersion != test.version || len(value.Instances) != test.selected || len(value.IncompatibleInstances) != test.incompatible {
				t.Fatalf("incorrect ABI selection: %+v", value)
			}
			identities := map[string]bool{}
			for _, instance := range value.Instances {
				identities[instance.Name+"@"+instance.Version] = true
			}
			for _, instance := range value.Instances {
				for _, edge := range instance.Bindings {
					if !identities[edge[2]] {
						t.Fatal("binding retained an incompatible ABI:", edge)
					}
				}
			}
		})
	}
}

func quote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func fixtureNode(t *testing.T) string {
	t.Helper()
	for _, location := range strings.Fields(os.Getenv("NODE_RUNFILES")) {
		if name := filepath.Base(location); name != "node" && name != "node.exe" {
			continue
		}
		file, err := runfiles.Rlocation(location)
		if err != nil {
			t.Fatal(err)
		}
		return file
	}
	t.Fatal("declared Node runtime is missing")
	return ""
}

func TestFrozenIncompatibleABIInputs(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*inventory, *inventory)
	}{
		{"missing", func(_, a *inventory) { a.IncompatibleInstances = a.IncompatibleInstances[:3] }},
		{"duplicate", func(_, a *inventory) { a.IncompatibleInstances[1] = a.IncompatibleInstances[0] }},
		{"unlisted", func(_, a *inventory) { a.IncompatibleInstances[0].Name = "unlisted" }},
		{"changed version", func(_, a *inventory) { a.IncompatibleInstances[0].Version = "next" }},
		{"wrong OS", func(_, a *inventory) { a.IncompatibleInstances[0].OS = []string{"darwin"} }},
		{"wrong CPU", func(_, a *inventory) { a.IncompatibleInstances[0].CPU = []string{"arm64"} }},
		{"compatible libc", func(_, a *inventory) { a.IncompatibleInstances[0].Libc = []string{"glibc"} }},
		{"non-leaf declarations", func(_, a *inventory) { a.IncompatibleInstances[0].DeclarationCounts[0] = 1 }},
		{"non-leaf binding", func(_, a *inventory) {
			a.IncompatibleInstances[0].Bindings = [][]string{{"dependencies", "missing", "missing@1"}}
		}},
		{"changed payload", func(_, a *inventory) { a.IncompatibleInstances[0].Files[0].SHA256 = strings.Repeat("b", 64) }},
		{"missing payload", func(_, a *inventory) { a.IncompatibleInstances[0].Files = nil }},
		{"duplicate payload", func(_, a *inventory) { a.IncompatibleInstances[0].Files[1] = a.IncompatibleInstances[0].Files[0] }},
		{"layout directory", func(_, a *inventory) { a.IncompatibleInstances[0].LayoutDirectories = []string{"node_modules/"} }},
		{"other platform", func(l, a *inventory) {
			l.Platform, a.Platform = "darwin", "darwin"
			l.Libc, a.Libc, l.LibcVersion, a.LibcVersion = "", "", "", ""
		}},
		{"other ABI", func(l, a *inventory) { l.Libc, a.Libc = "musl", "musl"; l.LibcVersion, a.LibcVersion = "", "" }},
		{"unknown ABI", func(l, a *inventory) { l.Libc, a.Libc, l.LibcVersion, a.LibcVersion = "", "", "", "" }},
		{"pnpm delta", func(l, a *inventory) { l.IncompatibleInstances = slices.Clone(a.IncompatibleInstances) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			l, a := sampleInventory(), sampleInventory()
			l.Platform, a.Platform = "linux", "linux"
			l.Architecture, a.Architecture = "x64", "x64"
			l.Libc, a.Libc = "glibc", "glibc"
			l.LibcVersion, a.LibcVersion = "2.39", "2.39"
			data, err := fixtures.ReadFile("fixtures/linux-glibc-exclusions.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &a.IncompatibleInstances); err != nil {
				t.Fatal(err)
			}
			r, err := inventoryComparison(l, a)
			if err != nil || r.PhysicalInstances["lattice"] != 40 || r.PhysicalInstances["aspect"] != 44 || r.Instances != 40 || r.ABIExtraFiles != 9 || r.ABIExtraBytes != 33_833_135 {
				t.Fatalf("frozen ABI delta: %+v %v", r, err)
			}
			test.edit(&l, &a)
			if _, err := inventoryComparison(l, a); err == nil {
				t.Fatal("invalid ABI delta accepted")
			}
		})
	}
}

func TestReportPreservesIncompatiblePayloadRecords(t *testing.T) {
	data, err := fixtures.ReadFile("fixtures/linux-glibc-exclusions.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected []packageInstance
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	value := report{Schema: 2, Qualified: true, Inventory: inventoryParity{ABIExtras: expected}}
	data, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal("benchmark cannot read its own report:", err)
	}
	if !reflect.DeepEqual(decoded.Inventory.ABIExtras, expected) {
		t.Fatal("report changed the frozen package identities or payload hashes")
	}
}
