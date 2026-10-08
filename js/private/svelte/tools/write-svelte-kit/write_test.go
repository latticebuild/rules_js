package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/latticebuild/rules_js/go/filetree"
)

func TestWriteRefreshesOwnedOutputs(t *testing.T) {
	for _, pkg := range []string{".", "web/app"} {
		t.Run(pkg, func(t *testing.T) {
			workspace, source := t.TempDir(), t.TempDir()
			t.Setenv("BUILD_WORKSPACE_DIRECTORY", workspace)
			destination := filepath.Join(workspace, pkg)
			generated(t, source)
			writeFile(t, filepath.Join(destination, "node_modules/$app/tsconfig.json"), "old config")
			writeFile(t, filepath.Join(destination, ".svelte-kit/types/stale/$types.d.ts"), "stale route")
			writeFile(t, filepath.Join(destination, ".svelte-kit/output/client/app.js"), "keep build")
			writeFile(t, filepath.Join(destination, "node_modules/other/index.js"), "keep dependency")
			writeFile(t, filepath.Join(destination, "node_modules/$app/other.json"), "keep unrelated")
			writeFile(t, filepath.Join(source, ".svelte-kit/types/current/$types.d.ts"), "new route")
			for range 2 {
				if err := run([]string{pkg, source}); err != nil {
					t.Fatal(err)
				}
				assertFile(t, filepath.Join(destination, "node_modules/$app/tsconfig.json"), "new config")
				assertFile(t, filepath.Join(destination, "node_modules/$app/tsconfig/service-worker.json"), "worker config")
				assertFile(t, filepath.Join(destination, "node_modules/$app/types/index.d.ts"), "app types")
				assertFile(t, filepath.Join(destination, "node_modules/$app/types/env.d.ts"), "env types")
				assertFile(t, filepath.Join(destination, ".svelte-kit/types/current/$types.d.ts"), "new route")
				assertFile(t, filepath.Join(destination, ".svelte-kit/output/client/app.js"), "keep build")
				assertFile(t, filepath.Join(destination, "node_modules/other/index.js"), "keep dependency")
				assertFile(t, filepath.Join(destination, "node_modules/$app/other.json"), "keep unrelated")
				if _, err := os.Stat(filepath.Join(destination, ".svelte-kit/types/stale")); !os.IsNotExist(err) {
					t.Fatalf("stale route remains: %v", err)
				}
			}
			writeFile(t, filepath.Join(destination, "node_modules/$app/tsconfig.json"), "editor change")
			assertFile(t, filepath.Join(source, "node_modules/$app/tsconfig.json"), "new config")
			matches, err := filepath.Glob(filepath.Join(destination, ".svelte-kit-write-*"))
			if err != nil || len(matches) != 0 {
				t.Fatalf("staging directories remain: %v, %v", matches, err)
			}
		})
	}
}

func TestWritePreservesEmptyTypesDirectory(t *testing.T) {
	workspace, source := t.TempDir(), t.TempDir()
	t.Setenv("BUILD_WORKSPACE_DIRECTORY", workspace)
	generated(t, source)
	writeFile(t, filepath.Join(workspace, ".svelte-kit/types/stale.d.ts"), "stale")
	if err := run([]string{".", source}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(workspace, ".svelte-kit/types"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("expected empty route types directory: %v, %v", entries, err)
	}
}

func TestWriteRefusesSourceAliasToOwnedOutputStorage(t *testing.T) {
	workspace := t.TempDir()
	destination := filepath.Join(workspace, "web/app")
	generated(t, destination)
	source := filepath.Join(t.TempDir(), "source")
	if err := filetree.DirLink(destination, source); err != nil {
		t.Fatal(err)
	}
	err := write(workspace, "web/app", source)
	if err == nil || !strings.Contains(err.Error(), "overlaps input storage") {
		t.Fatalf("source alias accepted: %v", err)
	}
	assertFile(t, filepath.Join(destination, "node_modules/$app/tsconfig.json"), "new config")
	entries, err := os.ReadDir(filepath.Join(destination, ".svelte-kit/types"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("input types directory changed: %v %v", entries, err)
	}
	matches, err := filepath.Glob(filepath.Join(destination, ".svelte-kit-write-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staging started: %v %v", matches, err)
	}
}

func TestWriteStagesBeforePublication(t *testing.T) {
	for _, kind := range []string{"missing directory", "regular file", "missing output", "escaping input link"} {
		t.Run(kind, func(t *testing.T) {
			workspace, source := t.TempDir(), t.TempDir()
			t.Setenv("BUILD_WORKSPACE_DIRECTORY", workspace)
			output := filepath.Join(workspace, "node_modules/$app/tsconfig.json")
			writeFile(t, output, "original config")
			generated(t, source)
			switch kind {
			case "missing directory":
				source = filepath.Join(source, "missing")
			case "regular file":
				source = filepath.Join(source, "file")
				writeFile(t, source, "not a directory")
			case "missing output":
				if err := os.RemoveAll(filepath.Join(source, ".svelte-kit/types")); err != nil {
					t.Fatal(err)
				}
			case "escaping input link":
				if err := os.RemoveAll(filepath.Join(source, ".svelte-kit/types")); err != nil {
					t.Fatal(err)
				}
				if err := filetree.DirSymlink(t.TempDir(), filepath.Join(source, ".svelte-kit/types")); err != nil {
					t.Fatal(err)
				}
			}
			if err := run([]string{".", source}); err == nil {
				t.Fatal("invalid input was accepted")
			}
			assertFile(t, output, "original config")
		})
	}
}

func TestWriteRefusesSymlinkedParents(t *testing.T) {
	for _, name := range []string{"package", ".svelte-kit", "node_modules", "node_modules/$app", "node_modules/$app/tsconfig"} {
		t.Run(name, func(t *testing.T) {
			workspace, source, outside := t.TempDir(), t.TempDir(), t.TempDir()
			t.Setenv("BUILD_WORKSPACE_DIRECTORY", workspace)
			generated(t, source)
			writeFile(t, filepath.Join(outside, "tsconfig.json"), "untouched")
			pkg := "."
			if name == "package" {
				pkg = name
			}
			link := filepath.Join(workspace, name)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := filetree.DirSymlink(outside, link); err != nil {
				t.Fatal(err)
			}
			if err := run([]string{pkg, source}); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("symlinked destination accepted: %v", err)
			}
			assertFile(t, filepath.Join(outside, "tsconfig.json"), "untouched")
		})
	}
}

func generated(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "node_modules/$app/tsconfig.json"), "new config")
	writeFile(t, filepath.Join(root, "node_modules/$app/tsconfig/service-worker.json"), "worker config")
	writeFile(t, filepath.Join(root, "node_modules/$app/types/index.d.ts"), "app types")
	writeFile(t, filepath.Join(root, "node_modules/$app/types/env.d.ts"), "env types")
	if err := os.MkdirAll(filepath.Join(root, ".svelte-kit/types"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, expected string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != expected {
		t.Fatalf("%s = %q, %v; want %q", path, contents, err, expected)
	}
}

func TestWriteRefreshesOutputsThroughRootAliases(t *testing.T) {
	workspace, source, aliases := t.TempDir(), t.TempDir(), t.TempDir()
	generated(t, source)
	writeFile(t, filepath.Join(workspace, "node_modules/$app/tsconfig.json"), "old config")
	workspaceAlias, sourceAlias := filepath.Join(aliases, "workspace"), filepath.Join(aliases, "source")
	if err := filetree.DirLink(workspace, workspaceAlias); err != nil {
		t.Fatal(err)
	}
	if err := filetree.DirLink(source, sourceAlias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BUILD_WORKSPACE_DIRECTORY", workspaceAlias)
	if err := run([]string{".", sourceAlias}); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(workspace, "node_modules/$app/tsconfig.json"), "new config")
	writeFile(t, filepath.Join(workspace, "node_modules/$app/tsconfig.json"), "editor change")
	assertFile(t, filepath.Join(source, "node_modules/$app/tsconfig.json"), "new config")
	matches, err := filepath.Glob(filepath.Join(workspace, ".svelte-kit-write-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staging directories remain: %v, %v", matches, err)
	}
}
