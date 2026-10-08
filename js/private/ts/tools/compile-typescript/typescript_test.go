package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizedDefaultsAndPaths(t *testing.T) {
	directory := t.TempDir()
	a := map[string]any{"composite": true, "checkJs": true, "outDir": "./build/../dist", "typeRoots": []any{"./types"}}
	normalizeOptions(a, directory)
	for _, key := range []string{"declaration", "incremental", "allowJs", "resolveJsonModule"} {
		if a[key] != true {
			t.Errorf("%s=%v", key, a[key])
		}
	}
	if a["outDir"] != filepath.Join(directory, "dist") || !reflect.DeepEqual(a["typeRoots"], []any{filepath.Join(directory, "types")}) {
		t.Fatal(a)
	}
	if a["rootDir"] != directory {
		t.Fatal("missing rootDir did not use the config directory")
	}
	b := map[string]any{"composite": true, "declaration": false, "checkJs": true, "allowJs": false, "resolveJsonModule": false}
	normalizeOptions(b, directory)
	for _, key := range []string{"declaration", "allowJs", "resolveJsonModule"} {
		if b[key] != false {
			t.Errorf("explicit %s=%v", key, b[key])
		}
	}
}
func TestMissingNodeRefusesBeforeCompiler(t *testing.T) {
	t.Setenv("NODE", "node")
	if err := run([]string{"compiler", "config", "{}", "[]", "[]", "[]", "package", "stamp"}); err == nil {
		t.Fatal("accepted undeclared Node")
	}
}
