//go:build !windows

package action

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	// The compiler adapter must inherit the action's process group. Cancellation
	// and an adapter crash must both kill Node and the compiler's own descendants.

	"github.com/latticebuild/rules_js/private/tools/internal/testfs"
)

func TestSignalExitAndCleanup(t *testing.T) {
	bin := runnerBinary(t)
	for _, wrapper := range []bool{false, true} {
		t.Run(fmt.Sprint("signal-wrapper-", wrapper), func(t *testing.T) {
			root := testfs.Root(t)
			script := `process.kill(process.pid, "SIGTERM");`
			if wrapper {
				script = `const fs=require("node:fs"), cp=require("node:child_process");
const child=cp.spawn(process.execPath,["-e","setInterval(()=>{},1000)"],{stdio:"ignore"});
process.on("SIGTERM",()=>{child.kill("SIGTERM");child.on("exit",()=>process.exit(0));});
child.on("spawn",()=>fs.writeFileSync("ready",process.pid+" "+child.pid));
setInterval(()=>{},1000);`
			}
			testfs.Write(t, filepath.Join(root, "tool.cjs"), script)
			body, err := json.Marshal(Manifest{Root: "scratch", Files: [][2]string{{"tool.cjs", "tool.cjs"}}, Scripts: []string{"tool.cjs"}})
			if err != nil {
				t.Fatal(err)
			}
			testfs.Write(t, filepath.Join(root, "manifest.json"), string(body))
			cmd := exec.CommandContext(t.Context(), bin, "manifest.json")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "NODE="+mustNode(t))
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			var pids []int
			if wrapper {
				deadline := time.Now().Add(10 * time.Second)
				for {
					ready, err := os.ReadFile(filepath.Join(root, "scratch/ready"))
					if err == nil {
						for _, word := range strings.Fields(string(ready)) {
							pid, err := strconv.Atoi(word)
							if err != nil {
								t.Fatal(err)
							}
							pids = append(pids, pid)
						}
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("child did not become ready")
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			}
			var exit *exec.ExitError
			if err := cmd.Wait(); !errors.As(err, &exit) || exit.ExitCode() != 143 {
				t.Fatalf("signal exit = %v, want 143", err)
			}
			if _, err := os.Stat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
				t.Fatalf("scratch remains after cancellation: %v", err)
			}
			for _, pid := range pids {
				if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
					t.Errorf("child %d remains after cancellation: %v", pid, err)
				}
			}
		})
	}
}
