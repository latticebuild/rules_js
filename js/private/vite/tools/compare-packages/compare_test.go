package main

import (
	"encoding/json"
	"github.com/latticebuild/rules_js/js/private/vite/tools/internal/testfs"

	// A Vite build shares a package's target copy with its execution copy only
	// after Compare succeeds, so a failed comparison must leave no stamp.
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	contents := strings.Repeat("export const value = 42;\n", 10_000)
	for _, c := range []struct {
		name    string
		change  func(t *testing.T, execution string)
		message string
	}{
		{name: "equal", change: func(*testing.T, string) {}},
		{name: "different bytes", message: "conflicting", change: func(t *testing.T, execution string) {
			testfs.Write(t, execution, contents[:len(contents)-2]+"!\n")
		}},
		{name: "different sizes", message: "conflicting", change: func(t *testing.T, execution string) {
			testfs.Write(t, execution, contents+"\n")
		}},
		{name: "different permissions", message: "conflicting", change: func(t *testing.T, execution string) {
			if runtime.GOOS == "windows" {
				t.Skip("Windows files carry no Unix permission bits")
			}
			if err := os.Chmod(execution, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "directory", message: "conflicting", change: func(t *testing.T, execution string) {
			if err := os.Remove(execution); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(execution, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing", message: "package output package/dist/index.js:", change: func(t *testing.T, execution string) {
			if err := os.Remove(execution); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := testfs.Root(t)
			testfs.Write(t, filepath.Join(root, "target.js"), contents)
			testfs.Write(t, filepath.Join(root, "execution.js"), contents)
			c.change(t, filepath.Join(root, "execution.js"))
			inventory, err := json.Marshal([][3]string{{"package/dist/index.js", "target.js", "execution.js"}})
			if err != nil {
				t.Fatal(err)
			}
			testfs.Write(t, filepath.Join(root, "inventory.json"), string(inventory))
			err = compare(root, "inventory.json", "out/equivalent")
			_, stamped := os.Stat(filepath.Join(root, "out/equivalent"))
			if c.message == "" {
				if err != nil || stamped != nil {
					t.Fatalf("Compare = %v, stamp %v", err, stamped)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.message) || !strings.Contains(err.Error(), "package/dist/index.js") {
				t.Fatalf("Compare = %v, want an error naming the pair (%s)", err, c.message)
			}
			if stamped == nil {
				t.Fatal("a failed comparison wrote the stamp")
			}
		})
	}
}
