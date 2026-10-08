package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/latticebuild/rules_js/go/filetree"
	"github.com/latticebuild/rules_js/js/private/tools/internal/testfs"
)

func TestStageCopiesDeclaredJunctionInputIndependently(t *testing.T) {
	root, physical := t.TempDir(), t.TempDir()
	input := filepath.Join(physical, "index.js")
	testfs.Write(t, input, "original\n")
	if err := filetree.DirLink(physical, filepath.Join(root, "input")); err != nil {
		t.Fatal(err)
	}
	if err := Stage(root, Manifest{Root: "scratch", Files: [][2]string{{"package", "input"}}}); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, "scratch/package/index.js")
	info, err := os.Lstat(staged)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("staged file: %v %v", info, err)
	}
	testfs.Write(t, staged, "changed\n")
	body, err := os.ReadFile(input)
	if err != nil || string(body) != "original\n" {
		t.Fatalf("input changed: %q %v", body, err)
	}
}

func TestRunRefusesEscapedJunctionAndRemovesScratch(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	t.Setenv("NODE", mustNode(t))
	testfs.Mkdir(t, filepath.Join(root, "input"))
	testfs.Write(t, filepath.Join(outside, "index.js"), "outside\n")
	testfs.Write(t, filepath.Join(root, "tool.cjs"), "throw new Error('must not run');\n")
	if err := filetree.DirLink(outside, filepath.Join(root, "input/escape")); err != nil {
		t.Fatal(err)
	}
	code, err := runAction(root, Manifest{
		Root: "scratch",
		Files: [][2]string{
			{"package", "input"},
			{"tool.cjs", "tool.cjs"},
		},
		Scripts: []string{"tool.cjs"},
	})
	if code == 0 || err == nil || !strings.Contains(err.Error(), "outside declared") {
		t.Fatalf("escaped junction: %d %v", code, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(outside, "index.js"))
	if err != nil || string(body) != "outside\n" {
		t.Fatalf("outside input changed: %q %v", body, err)
	}
}

func TestStageRefusesJunctionScratchRootBeforeMutation(t *testing.T) {
	root, backing := t.TempDir(), t.TempDir()
	testfs.Write(t, filepath.Join(root, "input.txt"), "input\n")
	testfs.Write(t, filepath.Join(backing, "sentinel"), "untouched\n")
	if err := filetree.DirLink(backing, filepath.Join(root, "scratch")); err != nil {
		t.Fatal(err)
	}
	err := Stage(root, Manifest{Root: "scratch", Files: [][2]string{{"sentinel", "input.txt"}}})
	if err == nil || !strings.Contains(err.Error(), "scratch root is a symlink") {
		t.Fatalf("aliased scratch accepted: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(backing, "sentinel"))
	if err != nil || string(body) != "untouched\n" {
		t.Fatalf("scratch backing changed: %q %v", body, err)
	}
}
