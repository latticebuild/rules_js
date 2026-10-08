//go:build darwin || linux

package filetree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestRealPathCompatibility(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "target", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "target", "input")
	writeFile(t, file, []byte("input"))
	if err := os.Symlink(filepath.Join(root, "target", "nested"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{root + "/alias/../input", file, filepath.Dir(file)} {
		want, err := filepath.EvalSymlinks(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := RealPath(name)
		if err != nil || got != want {
			t.Fatalf("RealPath(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if err := os.Chmod(file, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(file, 0o600) })
	if _, err := RealPath(file); err != nil {
		t.Fatal("unreadable regular input cannot be resolved:", err)
	}
	t.Chdir(root)
	want, err := filepath.EvalSymlinks("alias/../input")
	if err != nil {
		t.Fatal(err)
	}
	got, err := RealPath("alias/../input")
	if err != nil || got != want {
		t.Fatalf("relative semantics changed: %q %v", got, err)
	}
}

func TestRealPathFIFOIsMetadataOnly(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "fifo")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := RealPath(fifo)
		if err == nil {
			err = NewCopier(nil).Copy(fifo, filepath.Join(root, "copy"))
			if err == nil {
				err = errors.New("FIFO was accepted as a regular input")
			} else {
				err = nil
			}
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO resolution/read blocked")
	}
	_, err := resolveMetadataPath(fifo, func(string) (*os.File, error) {
		t.Fatal("special file reached Darwin metadata opener")
		return nil, errors.New("unexpected open")
	}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMetadataFallbackAndOwnership(t *testing.T) {
	file, _ := copyPaths(t)
	writeFile(t, file, []byte("input"))
	physical, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"native", "unavailable", "denied"} {
		t.Run(mode, func(t *testing.T) {
			var held *os.File
			got, err := resolveMetadataPath(file, func(path string) (*os.File, error) {
				if mode == "denied" {
					return nil, os.ErrPermission
				}
				var err error
				held, err = os.Open(path)
				return held, err
			}, func(*os.File) (string, error) {
				if mode == "unavailable" {
					return "", unix.ENOSYS
				}
				return physical, nil
			}, true)
			if err != nil || got != physical {
				t.Fatalf("%s: %q %v", mode, got, err)
			}
			if held != nil {
				if _, err := held.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatal("metadata handle leaked:", err)
				}
			}
		})
	}
	for range 32 {
		var held *os.File
		_, err := resolveMetadataPath(file, func(path string) (*os.File, error) {
			var err error
			held, err = os.Open(path)
			return held, err
		}, func(*os.File) (string, error) { return "relative", nil }, false)
		if err == nil {
			t.Fatal("relative physical candidate accepted")
		}
		if _, err := held.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatal("failed metadata lookup leaked a handle:", err)
		}
	}
	_, err = resolveMetadataPath(file, os.Open, func(held *os.File) (string, error) {
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
		return physical, nil
	}, false)
	if !errors.Is(err, os.ErrClosed) {
		t.Fatal("Close failure was hidden:", err)
	}
}

func TestMetadataRefusesReplacedAndDeletedInputs(t *testing.T) {
	for _, replace := range []bool{false, true} {
		file, _ := copyPaths(t)
		writeFile(t, file, []byte("original"))
		_, err := resolveMetadataPath(file, os.Open, func(*os.File) (string, error) {
			if err := os.Rename(file, file+".moved"); err != nil {
				t.Fatal(err)
			}
			if replace {
				writeFile(t, file, []byte("replacement"))
			}
			return "", unix.ENOSYS
		}, true)
		if err == nil {
			t.Fatal("deleted/replaced fallback acquired authority")
		}
	}
}
