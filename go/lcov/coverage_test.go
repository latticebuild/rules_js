package lcov

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLCOVRefusals(t *testing.T) {
	root := t.TempDir()
	for _, trace := range []string{"SF:a.js\nDA:1,1\n", "end_of_record\n", "SF:a.js\nSF:b.js\nend_of_record\n"} {
		if _, err := translateTracefile(trace, root, root, map[string]string{"a.js": "a.js"}); err == nil {
			t.Fatalf("accepted %q", trace)
		}
	}
}

func TestCoverageDoesNotPublishBeforeAllTracesValidate(t *testing.T) {
	root := t.TempDir()
	collected, destination := t.TempDir(), t.TempDir()
	for name, text := range map[string]string{"a.info": "SF:a.js\nDA:1,1\nend_of_record\n", "b.info": "SF:a.js\nDA:1,1\n"} {
		if err := os.WriteFile(filepath.Join(collected, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c := Collection{Collected: collected, destination: destination, tree: root, cwd: root, reported: map[string]string{"a.js": "src/a.js"}}
	if err := c.Publish(); err == nil {
		t.Fatal("accepted malformed coverage")
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial coverage published: %v %v", entries, err)
	}
}
