package action

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/latticebuild/rules_js/go/filetree"
	"github.com/latticebuild/rules_js/private/tools/internal/testfs"
)

func TestRunUsesPhysicalExecrootCasing(t *testing.T) {
	root := filepath.Join(t.TempDir(), "MixedCaseRoot")
	testfs.Mkdir(t, root)
	t.Setenv("NODE", mustNode(t))
	testfs.Write(t, filepath.Join(root, "input.txt"), "original\n")
	testfs.Write(t, filepath.Join(root, "tool.cjs"), `
const fs = require('node:fs');
fs.writeFileSync('result.json', JSON.stringify({
  cwd: process.cwd(), physical: fs.realpathSync.native('.'), input: fs.readFileSync('input.txt', 'utf8')
}));
`)
	code, err := Run(strings.ToLower(root), Manifest{
		Root:    "scratch",
		Files:   [][2]string{{"tool.cjs", "tool.cjs"}, {"input.txt", "input.txt"}},
		Scripts: []string{"tool.cjs"},
		Outputs: [][2]string{{"result.json", "result.json"}},
	})
	if code != 0 || err != nil {
		t.Fatalf("lowercase execroot: %d %v", code, err)
	}
	body, err := os.ReadFile(filepath.Join(root, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ Cwd, Physical, Input string }
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.Cwd != result.Physical || result.Input != "original\n" {
		t.Fatalf("inconsistent command paths or input: %+v", result)
	}
	body, err = os.ReadFile(filepath.Join(root, "input.txt"))
	if err != nil || string(body) != "original\n" {
		t.Fatalf("input changed: %q %v", body, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
}

func TestFastWindowsParentCannotLeaveDescendants(t *testing.T) {
	root := testfs.Root(t)
	t.Setenv("NODE", mustNode(t))
	testfs.Write(t, filepath.Join(root, "tool.cjs"), `
const child = require('node:child_process').spawn(process.execPath, ['-e', 'setInterval(()=>{},1000)'], {stdio:'ignore'});
require('node:fs').writeFileSync('pid', String(child.pid));
child.unref();
`)
	for i := range 16 {
		code, err := Run(root, Manifest{Root: "scratch", Files: [][2]string{{"tool.cjs", "tool.cjs"}}, Scripts: []string{"tool.cjs"}, Outputs: [][2]string{{"pid", "pid-" + strconv.Itoa(i)}}})
		if err != nil || code != 0 {
			t.Fatalf("iteration %d: %d, %v", i, code, err)
		}
		body, err := os.ReadFile(filepath.Join(root, "pid-"+strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.ParseUint(string(body), 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
		if err == syscall.Errno(87) {
			continue // The kernel has already removed the completed child.
		}
		if err != nil {
			t.Fatal(err)
		}
		status, waitErr := syscall.WaitForSingleObject(handle, 5000)
		_ = syscall.CloseHandle(handle)
		if waitErr != nil || status != syscall.WAIT_OBJECT_0 {
			t.Fatalf("child %d survived job cleanup: %d, %v", pid, status, waitErr)
		}
	}
}

func TestRunRefusesOutputParentJunctionToInput(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NODE", mustNode(t))
	testfs.Mkdir(t, filepath.Join(root, "source"))
	testfs.Write(t, filepath.Join(root, "source/input.txt"), "original\n")
	testfs.Write(t, filepath.Join(root, "tool.cjs"), `require('node:fs').writeFileSync('result.txt','changed');`)
	if err := filetree.DirLink(filepath.Join(root, "source"), filepath.Join(root, "output")); err != nil {
		t.Fatal(err)
	}
	code, err := Run(root, Manifest{
		Root: "scratch",
		Files: [][2]string{
			{"tool.cjs", "tool.cjs"},
			{"source/input.txt", "source/input.txt"},
		},
		Scripts: []string{"tool.cjs"},
		Outputs: [][2]string{{"result.txt", "output/input.txt"}},
	})
	if code == 0 || err == nil || !strings.Contains(err.Error(), "symlink or reparse point") {
		t.Fatalf("aliased output parent accepted: %d %v", code, err)
	}
	body, err := os.ReadFile(filepath.Join(root, "source/input.txt"))
	if err != nil || string(body) != "original\n" {
		t.Fatalf("declared input changed: %q %v", body, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
}

func TestRunRefusesJunctionOutputRootToAnotherDeclaredTree(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NODE", mustNode(t))
	testfs.Write(t, filepath.Join(root, "tool.cjs"), `
const fs = require('node:fs');
fs.mkdirSync('real');
fs.writeFileSync('real/result.txt', 'owned');
fs.symlinkSync('real', 'alias', 'junction');
`)
	code, err := Run(root, Manifest{
		Root:    "scratch",
		Files:   [][2]string{{"tool.cjs", "tool.cjs"}},
		Scripts: []string{"tool.cjs"},
		Outputs: [][2]string{
			{"real", "published-real"},
			{"alias", "published-alias"},
		},
	})
	if code == 0 || err == nil || !strings.Contains(err.Error(), "declared output alias is a symlink") {
		t.Fatalf("aliased output root accepted: %d %v", code, err)
	}
	for _, name := range []string{"scratch", "published-alias", "published-real"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("path %s remains: %v", name, err)
		}
	}
}

func TestRunPublishesThroughTrustedExecrootJunction(t *testing.T) {
	physical := t.TempDir()
	root := filepath.Join(t.TempDir(), "execroot")
	if err := filetree.DirLink(physical, root); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NODE", mustNode(t))
	testfs.Write(t, filepath.Join(physical, "input.txt"), "original\n")
	testfs.Write(t, filepath.Join(physical, "tool.cjs"), `
const fs = require('node:fs');
fs.writeFileSync('result.txt', fs.readFileSync('input.txt'));
`)
	code, err := Run(root, Manifest{
		Root:    "scratch",
		Files:   [][2]string{{"tool.cjs", "tool.cjs"}, {"input.txt", "input.txt"}},
		Scripts: []string{"tool.cjs"},
		Outputs: [][2]string{{"result.txt", "published.txt"}},
	})
	if code != 0 || err != nil {
		t.Fatalf("trusted execroot alias: %d %v", code, err)
	}
	body, err := os.ReadFile(filepath.Join(physical, "published.txt"))
	if err != nil || string(body) != "original\n" {
		t.Fatalf("published output: %q %v", body, err)
	}
	testfs.Write(t, filepath.Join(physical, "published.txt"), "changed\n")
	body, err = os.ReadFile(filepath.Join(physical, "input.txt"))
	if err != nil || string(body) != "original\n" {
		t.Fatalf("input aliases output: %q %v", body, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
}
