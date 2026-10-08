package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegularCacheDoesNotAuthorizeInventory(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	writeFile(t, input, []byte("source"))
	physical, err := RealPath(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, tail := range []string{"{bad}\n", "\"undeclared\"\n", "\"input/child\"\n"} {
		c := NewCopier(nil)
		c.DeclareRoot(physical)
		c.DeclareFile(input, physical)
		writeFile(t, filepath.Join(root, "inventory"), []byte("\"input\"\n"+tail))
		if err := c.ReadInputs(root, "inventory", []string{"input"}); err == nil {
			t.Fatal("cached regular entry bypassed inventory validation:", tail)
		}
	}
	c := NewCopier(nil)
	c.DeclareRoot(physical)
	c.DeclareFile(input, physical)
	writeFile(t, filepath.Join(root, "inventory"), []byte("\"input\"\n"))
	if err := c.ReadInputs(root, "inventory", []string{"input"}); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "output")
	if err := c.Copy(input, output); err != nil {
		t.Fatal(err)
	}
	writeFile(t, output, []byte("output edit"))
	assertContents(t, input, []byte("source"))
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
}

func TestCachedBackingPathCannotRebindCopyAuthority(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	outside := filepath.Join(root, "outside")
	writeFile(t, input, []byte("original"))
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outside, "secret"), []byte("undeclared"))
	physical, err := RealPath(input)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCopier(nil)
	c.DeclareRoot(physical)
	c.DeclareFile(input, physical)
	if err := os.Remove(input); err != nil {
		t.Fatal(err)
	}
	if err := DirLink(outside, input); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "output")
	if err := c.Copy(input, output); err == nil || !strings.Contains(err.Error(), "outside declared") {
		t.Fatal("rebound backing path acquired copy authority:", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("copy started before rebind refusal:", err)
	}
}
