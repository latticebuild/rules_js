package action

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/latticebuild/rules_js/go/filetree"
	"github.com/latticebuild/rules_js/private/tools/internal/testfs"
)

func TestStageRefusesInputAliasesToScratchStorage(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "scratch/value.txt")
	testfs.Mkdir(t, filepath.Dir(input))
	testfs.Write(t, input, "original\n")
	if err := filetree.DirLink(filepath.Join(root, "scratch"), filepath.Join(root, "input")); err != nil {
		t.Fatal(err)
	}
	err := Stage(root, Manifest{Root: "scratch", Files: [][2]string{{"copy", "input"}}})
	if err == nil || !strings.Contains(err.Error(), "overlaps input storage") {
		t.Fatalf("input alias accepted: %v", err)
	}
	body, err := os.ReadFile(input)
	if err != nil || string(body) != "original\n" {
		t.Fatalf("input changed: %q %v", body, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "scratch/copy")); !os.IsNotExist(err) {
		t.Fatalf("staging started: %v", err)
	}
}

func TestStageProtectsInputInventory(t *testing.T) {
	root := t.TempDir()
	testfs.Write(t, filepath.Join(root, "input.txt"), "original\n")
	inventory := filepath.Join(root, "scratch/inputs.jsonl")
	testfs.Mkdir(t, filepath.Dir(inventory))
	testfs.Write(t, inventory, "\"input.txt\"\n")
	err := Stage(root, Manifest{
		Root: "scratch", Files: [][2]string{{"inputs.jsonl", "input.txt"}}, InputList: "scratch/inputs.jsonl",
	})
	if err == nil || !strings.Contains(err.Error(), "overlaps protected input") {
		t.Fatalf("inventory overwrite accepted: %v", err)
	}
	body, err := os.ReadFile(inventory)
	if err != nil || string(body) != "\"input.txt\"\n" {
		t.Fatalf("inventory changed: %q %v", body, err)
	}
}

func TestPackageDependencyCycle(t *testing.T) {
	root := testfs.Root(t)
	err := Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"javascript/app/src/index.js", "javascript/app/src/index.js"}},
		Links: [][2]string{
			{"javascript/app/node_modules/@s/lib", "../../../lib"},
			{"javascript/lib/node_modules/@s/app", "../../../app"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(filepath.Join(root, "scratch/javascript/app/src/index.js"))
	if err != nil {
		t.Fatal(err)
	}
	throughCycle, err := os.Stat(filepath.Join(root, "scratch/javascript/app/node_modules/@s/lib/node_modules/@s/app/src/index.js"))
	if err != nil || !os.SameFile(original, throughCycle) {
		t.Fatalf("package identity changed through cycle: %v", err)
	}
}

func TestCopiesInputSymlink(t *testing.T) {
	root := testfs.Root(t)
	input := "bin/javascript/lib/dist/index.js"
	if err := Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"javascript/lib/dist/index.js", input}},
	}); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, "scratch/javascript/lib/dist/index.js")
	info, err := os.Lstat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("still a symlink")
	}
	body, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "export const y = 2;\n" {
		t.Fatalf("body = %q", body)
	}
	orig, err := os.ReadFile(filepath.Join(root, "real/lib/dist/index.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(orig) != "export const y = 2;\n" {
		t.Fatal("wrote through input")
	}
}

func TestWritableCopy(t *testing.T) {
	root := testfs.Root(t)
	input := filepath.Join(root, "javascript/app/src/index.js")
	if err := os.Chmod(input, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"javascript/app/src/index.js", "javascript/app/src/index.js"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scratch/javascript/app/src/index.js"), []byte("rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "export const x = 1;\n" {
		t.Fatal("mutated input")
	}
}

func TestHardlinkDetach(t *testing.T) {
	root := testfs.Root(t)
	input := filepath.Join(root, "bin/javascript/app/generated.json")
	testfs.Mkdir(t, filepath.Dir(input))
	testfs.Write(t, input, "{}\n")
	if err := os.Chmod(input, 0o444); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(root, "cached.json")
	if err := os.Link(input, cached); err != nil {
		t.Fatal(err)
	}
	if err := Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"javascript/app/generated.json", "bin/javascript/app/generated.json"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scratch/javascript/app/generated.json"), []byte("rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(cached)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "{}\n" {
		t.Fatal("wrote through hard link")
	}
}

func TestDirectoryInputDereferencesInnerSymlinks(t *testing.T) {
	root := testfs.Root(t)
	outside := filepath.Join(root, "outside.js")
	testfs.Write(t, outside, "export const z = 3;\n")
	if err := os.Chmod(outside, 0o444); err != nil {
		t.Fatal(err)
	}
	testfs.Mkdir(t, filepath.Join(root, "javascript/app/assets"))
	if err := os.Symlink(outside, filepath.Join(root, "javascript/app/assets/linked.js")); err != nil {
		t.Fatal(err)
	}
	if err := Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"javascript/app/assets", "javascript/app/assets"}, {"outside.js", "outside.js"}},
	}); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, "scratch/javascript/app/assets/linked.js")
	info, err := os.Lstat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("inner symlink survived")
	}
	if err := os.WriteFile(staged, []byte("rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "export const z = 3;\n" {
		t.Fatal("mutated outside")
	}
}

func TestPackageLinkDereferencesInputContents(t *testing.T) {
	root := testfs.Root(t)
	outside := filepath.Join(root, "outside.js")
	testfs.Write(t, outside, "export const z = 3;\n")
	if err := os.Chmod(outside, 0o444); err != nil {
		t.Fatal(err)
	}
	testfs.Mkdir(t, filepath.Join(root, "node_modules/entities"))
	if err := os.Symlink(outside, filepath.Join(root, "node_modules/entities/index.js")); err != nil {
		t.Fatal(err)
	}
	if err := Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"node_modules/entities", "node_modules/entities"}, {"outside.js", "outside.js"}},
		Links: [][2]string{
			{"javascript/app/node_modules/entities", "../../../node_modules/entities"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, "scratch/javascript/app/node_modules/entities/index.js")
	info, err := os.Lstat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("still a symlink")
	}
}

func TestCopiesDirectorySymlinks(t *testing.T) {
	root := testfs.Root(t)
	if err := Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"javascript/lib/dist", "bin/javascript/lib/dist"}, {"real/lib/dist/index.js", "real/lib/dist/index.js"}},
	}); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, "scratch/javascript/lib/dist/index.js")
	info, err := os.Lstat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("still a symlink")
	}
}

func TestPrepareReplacesExisting(t *testing.T) {
	root := testfs.Root(t)
	if err := Stage(root, Manifest{
		Root:  "bin",
		Files: [][2]string{{"javascript/lib/dist/index.js", "javascript/app/src/index.js"}},
	}); err != nil {
		t.Fatal(err)
	}
	orig, err := os.ReadFile(filepath.Join(root, "real/lib/dist/index.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(orig) != "export const y = 2;\n" {
		t.Fatal("wrote through existing symlink")
	}
	staged, err := os.ReadFile(filepath.Join(root, "bin/javascript/lib/dist/index.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(staged) != "export const x = 1;\n" {
		t.Fatalf("staged = %q", staged)
	}
}

func TestRefuseEscapeAbsoluteAndSymlinkedParent(t *testing.T) {
	root := testfs.Root(t)
	err := Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"../escape.js", "javascript/app/package.json"}},
	})
	if err == nil || !strings.Contains(err.Error(), "leaves the tree") {
		t.Fatalf("err = %v", err)
	}
	err = Stage(root, Manifest{
		Root:  "scratch",
		Links: [][2]string{{"javascript/app/node_modules/x", "/abs"}},
	})
	if err == nil || !strings.Contains(err.Error(), "must be relative") {
		t.Fatalf("err = %v", err)
	}
	testfs.Mkdir(t, filepath.Join(root, "scratch"))
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "scratch/linked")); err != nil {
		t.Fatal(err)
	}
	err = Stage(root, Manifest{
		Root:  "scratch",
		Files: [][2]string{{"linked/lib/x.js", "javascript/app/package.json"}},
	})
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("err = %v", err)
	}
}

func TestRejectsInPlaceStaging(t *testing.T) {
	root := testfs.Root(t)
	input := "bin/javascript/lib/dist/index.js"
	err := Stage(root, Manifest{
		Root:  "bin",
		Files: [][2]string{{"javascript/lib/dist/index.js", input}},
	})
	if err == nil || !strings.Contains(err.Error(), "staging destination") {
		t.Fatalf("in-place input accepted: %v", err)
	}
	info, err := os.Lstat(filepath.Join(root, input))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("input was replaced: %v", err)
	}
}

func TestDirectoryInventoryCannotBroadenDeclaredInputs(t *testing.T) {
	for _, contents := range []string{"not-json\n", "\"../outside\"\n", "\"undeclared\"\n"} {
		t.Run(contents, func(t *testing.T) {
			root := testfs.Root(t)
			testfs.Mkdir(t, filepath.Join(root, "artifact"))
			testfs.Write(t, filepath.Join(root, "undeclared"), "outside")
			testfs.Write(t, filepath.Join(root, "inputs.jsonl"), contents)
			err := Stage(root, Manifest{Root: "scratch", Files: [][2]string{{"artifact", "artifact"}}, InputList: "inputs.jsonl"})
			if err == nil {
				t.Fatal("invalid input inventory accepted")
			}
		})
	}
}

func TestDeclaredDirectoryLeavesWorkThroughSandboxSymlinks(t *testing.T) {
	t.Run("action", func(t *testing.T) {
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

		err := Stage(root, Manifest{Root: "scratch", Files: [][2]string{{"artifact", artifact}}, InputList: "inputs.jsonl"})
		if err != nil {
			t.Fatal(err)
		}

		body, err := os.ReadFile(filepath.Join(root, "scratch/artifact/nested/value.js"))
		if err != nil || string(body) != "value.js" {
			t.Fatalf("staged file = %s, %v", body, err)
		}
	})
}
