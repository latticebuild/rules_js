//go:build !windows

package integration

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/latticebuild/rules_js/js/private/ts/tools/internal/testfs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type Manifest struct {
	Executable string            `json:"executable,omitempty"`
	Root       string            `json:"root"`
	Files      [][2]string       `json:"files"`
	InputList  string            `json:"input_list"`
	Links      [][2]string       `json:"links"`
	Cwd        string            `json:"cwd"`
	Scripts    []string          `json:"scripts"`
	Args       []string          `json:"args"`
	Env        map[string]string `json:"env"`
	// StatusFile is Bazel's stable workspace status file, present when an
	// environment value names a {STABLE_KEY} of a stamped build.
	StatusFile string      `json:"status_file,omitempty"`
	Outputs    [][2]string `json:"outputs"`
}

func TestNativeCompilerProcessOwnership(t *testing.T) {
	checker, err := runfiles.Rlocation(os.Getenv("LATTICEBUILD_TSC_BINARY"))
	if err != nil {
		t.Fatal(err)
	}
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprintf("crash-%t", crash), func(t *testing.T) {
			root := testfs.Root(t)
			ready := filepath.Join(root, "ready")
			binary, err := os.ReadFile(checker)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "checker"), binary, 0o755); err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"input.ts":                 "export const value = 1;",
				"tsconfig.json":            "{}",
				"validation/package.json":  `{"bin":{"tsc":"validator.cjs"}}`,
				"validation/validator.cjs": `console.log(JSON.stringify({compilerOptions:{},files:[require('node:path').resolve('input.ts')]}));`,
				"compiler.cjs": `const fs=require('node:fs'),cp=require('node:child_process');
fs.writeFileSync('output','partial');
const child=cp.spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore'});
child.on('spawn',()=>{
  fs.writeFileSync(process.env.READY+'.tmp',[process.ppid,process.pid,child.pid].join(' '));
  fs.renameSync(process.env.READY+'.tmp',process.env.READY);
});
setInterval(()=>{},1000);`,
			}
			manifest := Manifest{Root: "scratch", Executable: "checker", Scripts: []string{"compiler.cjs"}, Args: []string{"tsconfig.json", "{}", `["input.ts"]`, "[]", `["output"]`, "validation", "stamp"}, Env: map[string]string{"READY": ready, "NODE": "/undeclared/node", "PATH": "/undeclared"}, Outputs: [][2]string{{"output", "published"}}}
			for path, text := range files {
				testfs.Mkdir(t, filepath.Dir(filepath.Join(root, path)))
				testfs.Write(t, filepath.Join(root, path), text)
				manifest.Files = append(manifest.Files, [2]string{path, path})
			}
			data, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			testfs.Write(t, filepath.Join(root, "manifest.json"), string(data))
			command := exec.CommandContext(t.Context(), runnerBinary(t), "manifest.json")
			command.Dir = root
			command.Env = append(os.Environ(), "NODE="+mustNode(t))
			log := new(strings.Builder)
			command.Stdout = log
			command.Stderr = log
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = command.Process.Kill() })
			var pids []int
			deadline := time.Now().Add(15 * time.Second)
			for {
				data, err := os.ReadFile(ready)
				if err == nil {
					for _, word := range strings.Fields(string(data)) {
						pid, err := strconv.Atoi(word)
						if err != nil {
							t.Fatal(err)
						}
						pids = append(pids, pid)
					}
					break
				}
				if time.Now().After(deadline) {
					_ = command.Process.Kill()
					_ = command.Wait()
					t.Fatalf("compiler did not start: %s", log)
				}
				time.Sleep(10 * time.Millisecond)
			}
			if len(pids) != 3 {
				t.Fatalf("pids=%v", pids)
			}
			want := 143
			if crash {
				want = 137
				if err := syscall.Kill(pids[0], syscall.SIGKILL); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := command.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			}
			var exited *exec.ExitError
			if err := command.Wait(); !errors.As(err, &exited) || exited.ExitCode() != want {
				t.Fatalf("exit=%v want=%d\n%s", err, want, log)
			}
			for _, name := range []string{"scratch", "published"} {
				if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
					t.Fatalf("%s remains: %v", name, err)
				}
			}
			for _, pid := range pids {
				deadline := time.Now().Add(3 * time.Second)
				for syscall.Kill(pid, 0) != syscall.ESRCH {
					if time.Now().After(deadline) {
						t.Fatalf("process %d survived", pid)
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		})
	}
}

func runnerBinary(t *testing.T) string {
	t.Helper()
	path, err := runfiles.Rlocation(os.Getenv("LATTICEBUILD_TEST_BINARY"))
	if err != nil {
		t.Fatalf("resolve Bazel runner executable: %v", err)
	}
	return path
}

func mustNode(t *testing.T) string {
	t.Helper()
	node := os.Getenv("NODE")
	if node == "" {
		for _, path := range strings.Fields(os.Getenv("NODE_RUNFILES")) {
			if name := filepath.Base(path); name == "node" || name == "node.exe" {
				node = path
				break
			}
		}
	}
	if node != "" {
		if filepath.IsAbs(node) {
			return node
		}
		path, err := runfiles.Rlocation(node)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("declared Node executable missing: %v", err)
		}
		return path
	}
	if os.Getenv("TEST_SRCDIR") != "" {
		t.Fatal("declared Node runtime missing from Bazel runfiles")
	}
	path, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("required Node runtime not found")
	}
	return path
}
