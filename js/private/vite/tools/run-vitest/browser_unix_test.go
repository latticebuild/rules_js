//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserRetainsDeclaredFrameworkAliases(t *testing.T) {
	for _, mode := range []string{"directory", "manifest"} {
		t.Run(mode, func(t *testing.T) {
			fixture := browserFixture(t, mode)
			version := filepath.Join(fixture.source, "native", "Versions", "A")
			if err := os.MkdirAll(filepath.Join(version, "Resources"), 0o700); err != nil {
				t.Fatal(err)
			}
			for name, contents := range map[string]string{"framework": "versioned", "Resources/asset": "resource"} {
				if err := os.WriteFile(filepath.Join(version, filepath.FromSlash(name)), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for name, target := range map[string]string{"Versions/Current": "A", "framework": "Versions/Current/framework", "Resources": "Versions/Current/Resources"} {
				if err := os.Symlink(target, filepath.Join(fixture.source, "native", filepath.FromSlash(name))); err != nil {
					t.Fatal(err)
				}
			}
			fixture.addFiles(t,
				"native/Versions/A/framework", "native/Versions/A/Resources/asset",
				"native/Versions/Current/framework", "native/Versions/Current/Resources/asset",
				"native/framework", "native/Resources/asset",
			)
			bundle := t.TempDir()
			t.Setenv("BOUND_ROOT", bundle)
			root, err := prepareBrowser(fixture.writeLayout(t, bundle))
			if err != nil {
				t.Fatal(err)
			}
			for name, want := range map[string]string{"framework": "versioned", "Resources/asset": "resource"} {
				contents, err := os.ReadFile(filepath.Join(root, "chromium-1247", "native", filepath.FromSlash(name)))
				if err != nil || string(contents) != want {
					t.Fatalf("framework alias %s changed: %q %v", name, contents, err)
				}
			}
			if err := os.RemoveAll(bundle); err != nil {
				t.Fatal(err)
			}
			if target, err := os.Readlink(filepath.Join(fixture.source, "native", "Versions", "Current")); err != nil || target != "A" {
				t.Fatalf("private cleanup altered the native framework alias: %q %v", target, err)
			}
		})
	}
}

func TestBrowserRefusesUnownedAndCyclicFrameworkAliases(t *testing.T) {
	for _, scenario := range []string{"escaping file", "escaping directory", "undeclared alias", "undeclared target", "resolution loop", "ancestor directory cycle"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := browserFixture(t, "manifest")
			native := filepath.Join(fixture.source, "native")
			alias := filepath.Join(native, "alias")
			switch scenario {
			case "escaping file", "escaping directory":
				outside := filepath.Join(t.TempDir(), "resource")
				if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
					t.Fatal(err)
				}
				target := outside
				name := "native/alias"
				if scenario == "escaping directory" {
					target = filepath.Dir(outside)
					name += "/resource"
				}
				if err := os.Symlink(target, alias); err != nil {
					t.Fatal(err)
				}
				fixture.addFiles(t, name)
			case "undeclared alias", "undeclared target":
				target := "browser"
				if scenario == "undeclared target" {
					target = "hidden"
					if err := os.WriteFile(filepath.Join(native, target), []byte("hidden"), 0o600); err != nil {
						t.Fatal(err)
					}
					fixture.addFiles(t, "native/alias")
				}
				if err := os.Symlink(target, alias); err != nil {
					t.Fatal(err)
				}
			case "resolution loop":
				for name, target := range map[string]string{"alias": "loop", "loop": "alias"} {
					if err := os.Symlink(target, filepath.Join(native, name)); err != nil {
						t.Fatal(err)
					}
				}
				fixture.addFiles(t, "native/alias")
			case "ancestor directory cycle":
				if err := os.Symlink(".", alias); err != nil {
					t.Fatal(err)
				}
				fixture.addFiles(t, "native/alias/browser")
			}
			bundle := t.TempDir()
			t.Setenv("BOUND_ROOT", bundle)
			if _, err := prepareBrowser(fixture.writeLayout(t, bundle)); err == nil {
				t.Fatal("accepted invalid native framework aliases")
			}
			if _, err := os.Stat(filepath.Join(bundle, "tree", ".playwright", "chromium-1247", "INSTALLATION_COMPLETE")); !os.IsNotExist(err) {
				t.Fatalf("refusal published a completed installation: %v", err)
			}
		})
	}
}
