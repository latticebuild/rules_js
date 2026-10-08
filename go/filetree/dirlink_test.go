package filetree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirLinkPreservesNativeFilesAfterCleanup(t *testing.T) {
	source, owned := t.TempDir(), t.TempDir()
	file := filepath.Join(source, "browser")
	if err := os.WriteFile(file, []byte("native input"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(owned, "payload")
	if err := DirLink(source, link); err != nil {
		t.Fatal(err)
	}
	real, err := RealPath(filepath.Join(link, "browser"))
	want, wantErr := RealPath(file)
	if err != nil || wantErr != nil || PathKey(real) != PathKey(want) {
		t.Fatalf("native identity: %s %s %v %v", real, want, err, wantErr)
	}
	if err := os.RemoveAll(owned); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(file)
	if err != nil || string(contents) != "native input" {
		t.Fatalf("cleanup altered native input: %q %v", contents, err)
	}
}

func TestDirLinkRefusesInvalidTargetsAndExistingDestinations(t *testing.T) {
	source, owned := t.TempDir(), t.TempDir()
	file := filepath.Join(source, "file")
	if err := os.WriteFile(file, []byte("retained"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"relative", file, filepath.Join(source, "missing")} {
		link := filepath.Join(owned, "payload")
		if err := DirLink(target, link); err == nil {
			t.Fatalf("accepted target %s", target)
		}
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			t.Fatalf("invalid target created a destination: %v", err)
		}
	}
	if err := DirLink(source, "relative"); err == nil {
		t.Fatal("accepted a relative destination")
	}
	existing := filepath.Join(owned, "existing")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(existing, "retained")
	if err := os.WriteFile(sentinel, []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := DirLink(source, existing); err == nil {
		t.Fatal("replaced an existing destination")
	}
	contents, err := os.ReadFile(sentinel)
	if err != nil || string(contents) != "owned" {
		t.Fatalf("existing destination changed: %q %v", contents, err)
	}
}
