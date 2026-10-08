package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCopyRefusesJunctionAncestorCycle(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := DirLink(root, filepath.Join(root, "inner/back")); err != nil {
		t.Fatal(err)
	}
	physical, err := RealPath(root)
	if err != nil {
		t.Fatal(err)
	}
	copier := NewCopier(nil)
	copier.DeclareRoot(physical)
	if err := copier.Copy(root, filepath.Join(t.TempDir(), "copy")); err == nil || !strings.Contains(err.Error(), "symlink cycle") {
		t.Fatalf("junction cycle: %v", err)
	}
}

func TestDestinationRefusesShortNameAliasOfInput(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "directory with a long name")
	if err := os.Mkdir(input, 0o700); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(windowsPath(input))
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	size, err := windows.GetShortPathName(name, &buffer[0], uint32(len(buffer)))
	if err != nil || size >= uint32(len(buffer)) {
		t.Fatalf("short input path: %d %v", size, err)
	}
	short := filepath.Base(windows.UTF16ToString(buffer[:size]))
	if PathKey(short) == PathKey(filepath.Base(input)) {
		t.Skip("test filesystem does not create 8.3 directory aliases")
	}
	physical, err := RealPath(input)
	if err != nil {
		t.Fatal(err)
	}
	copier := NewCopier(nil)
	copier.DeclareRoot(physical)
	file := filepath.Join(input, "index.js")
	writeFile(t, file, []byte("original\n"))
	for _, output := range []string{"index.js", "new/result.js"} {
		target := filepath.Join(root, short, output)
		if err := copier.CheckDestination(root, target); err == nil || !strings.Contains(err.Error(), "overlaps input storage") {
			t.Fatalf("short-name write to %s accepted: %v", output, err)
		}
	}
	assertContents(t, file, []byte("original\n"))
	if _, err := os.Lstat(filepath.Join(input, "new")); !os.IsNotExist(err) {
		t.Fatalf("destination parent created: %v", err)
	}
}
