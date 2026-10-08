package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJunctionFailureRemovesIncompleteDirectory(t *testing.T) {
	link := filepath.Join(t.TempDir(), "payload")
	if err := junction(link, make([]byte, 8)); err == nil {
		t.Fatal("accepted an invalid reparse point")
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("incomplete junction remains: %v", err)
	}
}

func TestJunctionRefusesRemoteTargetBeforeMutation(t *testing.T) {
	for _, target := range []string{`\\server\share\browser`, `\\?\UNC\server\share\browser`} {
		link := filepath.Join(t.TempDir(), "payload")
		if err := dirLink(target, link); err == nil || !strings.Contains(err.Error(), "remote share") {
			t.Fatalf("remote target: %v", err)
		}
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			t.Fatalf("remote target created destination: %v", err)
		}
	}
}

func TestJunctionAndRealPathSupportLongLocalTargets(t *testing.T) {
	source := t.TempDir()
	for range 6 {
		source = filepath.Join(source, strings.Repeat("native", 10))
	}
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "browser")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "payload")
	if err := DirLink(source, link); err != nil {
		t.Fatal(err)
	}
	real, err := RealPath(filepath.Join(link, "browser"))
	want, wantErr := RealPath(file)
	if err != nil || wantErr != nil || PathKey(real) != PathKey(want) {
		t.Fatalf("long native identity: %s %s %v %v", real, want, err, wantErr)
	}
}
