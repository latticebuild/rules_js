package testfs

import (
	"os"
	"path/filepath"
	"testing"
)

func Root(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "js-runner-")
	if err != nil {
		t.Fatal(err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	Mkdir(t, filepath.Join(dir, "javascript/app/src"))
	Write(t, filepath.Join(dir, "javascript/app/src/index.js"), "export const x = 1;\n")
	Write(t, filepath.Join(dir, "javascript/app/package.json"), "{\"type\":\"module\"}\n")
	Mkdir(t, filepath.Join(dir, "real/lib/dist"))
	Write(t, filepath.Join(dir, "real/lib/dist/index.js"), "export const y = 2;\n")
	Mkdir(t, filepath.Join(dir, "bin/javascript/lib/dist"))
	if err := os.Symlink(
		filepath.Join(dir, "real/lib/dist/index.js"),
		filepath.Join(dir, "bin/javascript/lib/dist/index.js"),
	); err != nil {
		t.Fatal(err)
	}
	return dir
}

func Mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func Write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
