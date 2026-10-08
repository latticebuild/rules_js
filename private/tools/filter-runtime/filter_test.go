package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/latticebuild/rules_js/go/filetree"
	"github.com/latticebuild/rules_js/private/tools/internal/testfs"
)

func TestRuntimeFilterTraversesDirectoryArtifacts(t *testing.T) {
	root := testfs.Root(t)
	source := filepath.Join(root, "artifact")
	for _, rel := range []string{"index.js", "nested/a.cjs", "nested/a.d.cts", "nested/a.cjs.map", "build.tsbuildinfo", "index.d.ts", "node_modules/@types/a/index.d.ts"} {
		file := filepath.Join(source, rel)
		testfs.Mkdir(t, filepath.Dir(file))
		testfs.Write(t, file, rel)
	}
	if err := filterDirectory(root, "artifact", "runtime"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"index.js", "nested/a.cjs"} {
		if _, err := os.Stat(filepath.Join(root, "runtime", rel)); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{"nested/a.d.cts", "nested/a.cjs.map", "build.tsbuildinfo", "index.d.ts", "node_modules/@types"} {
		if _, err := os.Stat(filepath.Join(root, "runtime", rel)); !os.IsNotExist(err) {
			t.Fatalf("filtered path %s remains: %v", rel, err)
		}
		if _, err := os.Stat(filepath.Join(source, rel)); err != nil {
			t.Fatal("filter changed source")
		}
	}
}

func TestDeclaredDirectoryLeavesWorkThroughSandboxSymlinks(t *testing.T) {
	t.Run("filter", func(t *testing.T) {
		root := testfs.Root(t)
		artifact := "artifact space"
		testfs.Mkdir(t, filepath.Join(root, artifact, "nested"))
		var inventory strings.Builder
		for _, name := range []string{"value.js", "value.d.ts"} {
			backing := filepath.Join(root, "backing-"+name)
			testfs.Write(t, backing, name)
			leaf := filepath.ToSlash(filepath.Join(artifact, "nested", name))
			if err := os.Symlink(backing, filepath.Join(root, leaf)); err != nil {
				t.Fatal(err)
			}
			line, err := json.Marshal(leaf)
			if err != nil {
				t.Fatal(err)
			}
			inventory.Write(line)
			inventory.WriteByte('\n')
		}
		testfs.Write(t, filepath.Join(root, "inputs.jsonl"), inventory.String())

		if err := filterDirectory(root, artifact, "scratch/artifact", "inputs.jsonl"); err != nil {
			t.Fatal(err)
		}

		body, err := os.ReadFile(filepath.Join(root, "scratch/artifact/nested/value.js"))
		if err != nil || string(body) != "value.js" {
			t.Fatalf("staged file = %s, %v", body, err)
		}

		if _, err := os.Stat(filepath.Join(root, "scratch/artifact/nested/value.d.ts")); !os.IsNotExist(err) {
			t.Fatalf("declaration survived: %v", err)
		}
	})
}

func TestRuntimeFilterCopiesAliasedDirectoryInputs(t *testing.T) {
	root, physical := t.TempDir(), t.TempDir()
	testfs.Write(t, filepath.Join(physical, "index.js"), "runtime\n")
	testfs.Write(t, filepath.Join(physical, "index.d.ts"), "declaration\n")
	if err := filetree.DirLink(physical, filepath.Join(root, "artifact")); err != nil {
		t.Fatal(err)
	}
	if err := filterDirectory(root, "artifact", "runtime"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "runtime/index.js"))
	if err != nil || string(body) != "runtime\n" {
		t.Fatalf("runtime file: %q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(root, "runtime/index.d.ts")); !os.IsNotExist(err) {
		t.Fatalf("declaration survived: %v", err)
	}
	testfs.Write(t, filepath.Join(root, "runtime/index.js"), "changed\n")
	body, err = os.ReadFile(filepath.Join(physical, "index.js"))
	if err != nil || string(body) != "runtime\n" {
		t.Fatalf("input changed: %q %v", body, err)
	}
}

func TestRuntimeFilterRefusesInputAliasesToOutputStorage(t *testing.T) {
	for _, destination := range []string{"backing/input", "backing", "backing/input/new/runtime"} {
		t.Run(destination, func(t *testing.T) {
			root := t.TempDir()
			testfs.Mkdir(t, filepath.Join(root, "backing/input"))
			testfs.Write(t, filepath.Join(root, "backing/input/index.js"), "original\n")
			testfs.Write(t, filepath.Join(root, "backing/input/index.d.ts"), "declaration\n")
			if err := filetree.DirLink(filepath.Join(root, "backing/input"), filepath.Join(root, "artifact")); err != nil {
				t.Fatal(err)
			}
			err := filterDirectory(root, "artifact", destination)
			if err == nil || !strings.Contains(err.Error(), "overlaps input storage") {
				t.Fatalf("input alias accepted: %v", err)
			}
			for name, expected := range map[string]string{"index.js": "original\n", "index.d.ts": "declaration\n"} {
				body, err := os.ReadFile(filepath.Join(root, "backing/input", name))
				if err != nil || string(body) != expected {
					t.Fatalf("input %s changed: %q %v", name, body, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(root, "backing/input/new")); !os.IsNotExist(err) {
				t.Fatalf("output parent created: %v", err)
			}
		})
	}
}

func TestRuntimeFilterProtectsInputInventory(t *testing.T) {
	root := t.TempDir()
	testfs.Mkdir(t, filepath.Join(root, "artifact"))
	testfs.Write(t, filepath.Join(root, "artifact/index.js"), "original\n")
	inventory := filepath.Join(root, "runtime/inputs.jsonl")
	testfs.Mkdir(t, filepath.Dir(inventory))
	testfs.Write(t, inventory, "\"artifact/index.js\"\n")
	err := filterDirectory(root, "artifact", "runtime", "runtime/inputs.jsonl")
	if err == nil || !strings.Contains(err.Error(), "overlaps protected input") {
		t.Fatalf("inventory overwrite accepted: %v", err)
	}
	body, err := os.ReadFile(inventory)
	if err != nil || string(body) != "\"artifact/index.js\"\n" {
		t.Fatalf("inventory changed: %q %v", body, err)
	}
}
