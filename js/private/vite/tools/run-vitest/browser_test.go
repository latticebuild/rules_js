package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/latticebuild/rules_js/go/filetree"
)

func TestBrowserPrivateMetadataAndStableNativePayload(t *testing.T) {
	for _, mode := range []string{"directory", "manifest"} {
		t.Run(mode, func(t *testing.T) {
			fixture := browserFixture(t, mode)
			var native string
			for range 2 {
				bundle := t.TempDir()
				layout := fixture.writeLayout(t, bundle)
				t.Setenv("BOUND_ROOT", bundle)
				root, err := prepareBrowser(layout)
				if err != nil {
					t.Fatal(err)
				}
				installation := filepath.Join(root, "chromium-1247")
				if _, err := os.Stat(filepath.Join(installation, "DEPENDENCIES_VALIDATED")); !os.IsNotExist(err) {
					t.Fatalf("private validation marker was not fresh: %v", err)
				}
				if err := os.WriteFile(filepath.Join(installation, "DEPENDENCIES_VALIDATED"), []byte("this run"), 0o600); err != nil {
					t.Fatal(err)
				}
				resolved, err := filetree.RealPath(filepath.Join(installation, "native", "browser"))
				want, wantErr := filetree.RealPath(filepath.Join(fixture.source, "native", "browser"))
				if err != nil || wantErr != nil || filetree.PathKey(resolved) != filetree.PathKey(want) || (native != "" && native != resolved) {
					t.Fatalf("unstable browser identity: %s %s %v %v", resolved, want, err, wantErr)
				}
				native = resolved
				if err := os.RemoveAll(bundle); err != nil {
					t.Fatal(err)
				}
				for file, want := range map[string]string{"native/browser": "native", "native/resource": "companion", "DEPENDENCIES_VALIDATED": "legacy"} {
					contents, err := os.ReadFile(filepath.Join(fixture.source, filepath.FromSlash(file)))
					if err != nil || string(contents) != want {
						t.Fatalf("source %s changed during metadata writes/cleanup: %q %v", file, contents, err)
					}
				}
			}
		})
	}
}

func TestBrowserRefusesInvalidOrUndeclaredInputsBeforeStaging(t *testing.T) {
	for _, scenario := range []string{"absolute marker", "escaping file", "duplicate file", "validation payload", "missing file", "missing marker", "extra file", "empty directory", "escaped mapping", "existing installation", "linked browser root", "linked application tree", "linked payload root"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := browserFixture(t, "manifest")
			bundle := t.TempDir()
			var outsideDestination string
			t.Setenv("BOUND_ROOT", bundle)
			switch scenario {
			case "absolute marker":
				fixture.marker = filepath.Join(fixture.source, "INSTALLATION_COMPLETE")
			case "escaping file":
				fixture.files = []string{"../outside"}
			case "duplicate file":
				fixture.files = append(fixture.files, fixture.files[0])
			case "validation payload":
				fixture.files = append(fixture.files, "DEPENDENCIES_VALIDATED")
			case "missing file":
				if err := os.Remove(filepath.Join(fixture.source, "native", "browser")); err != nil {
					t.Fatal(err)
				}
			case "missing marker":
				if err := os.Remove(filepath.Join(fixture.source, "INSTALLATION_COMPLETE")); err != nil {
					t.Fatal(err)
				}
			case "extra file":
				if err := os.WriteFile(filepath.Join(fixture.source, "native", "undeclared"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "empty directory":
				if err := os.Mkdir(filepath.Join(fixture.source, "native", "undeclared"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "escaped mapping":
				outside := filepath.Join(t.TempDir(), "browser")
				if err := os.WriteFile(outside, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				contents := fixture.manifest + fixture.prefix + "native/browser " + outside + "\n"
				if err := os.WriteFile(os.Getenv("RUNFILES_MANIFEST_FILE"), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			case "linked payload root":
				native := filepath.Join(fixture.source, "native")
				target := filepath.Join(fixture.source, "payload")
				if err := os.Rename(native, target); err != nil {
					t.Fatal(err)
				}
				if err := filetree.DirLink(target, native); err != nil {
					t.Fatal(err)
				}
				fixture.addFiles(t, "payload/browser", "payload/resource")
			case "existing installation":
				if err := os.MkdirAll(filepath.Join(bundle, "tree", ".playwright", "chromium-1247"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "linked browser root", "linked application tree":
				outsideDestination = t.TempDir()
				if err := os.WriteFile(filepath.Join(outsideDestination, "retained"), []byte("outside"), 0o600); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(bundle, "tree")
				if scenario == "linked browser root" {
					if err := os.Mkdir(link, 0o700); err != nil {
						t.Fatal(err)
					}
					link = filepath.Join(link, ".playwright")
				}
				if err := filetree.DirLink(outsideDestination, link); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := prepareBrowser(fixture.writeLayout(t, bundle)); err == nil {
				t.Fatal("accepted invalid browser inputs")
			}
			if _, err := os.Stat(filepath.Join(bundle, "tree", ".playwright", "chromium-1247", "INSTALLATION_COMPLETE")); !os.IsNotExist(err) {
				t.Fatalf("failed preparation published a completed installation: %v", err)
			}
			if outsideDestination != "" {
				entries, err := os.ReadDir(outsideDestination)
				contents, readErr := os.ReadFile(filepath.Join(outsideDestination, "retained"))
				if err != nil || len(entries) != 1 || readErr != nil || string(contents) != "outside" {
					t.Fatalf("refusal altered unowned destination: %v %v %q %v", entries, err, contents, readErr)
				}
			}
		})
	}
}

type browserInputFixture struct {
	source, marker, prefix, manifest string
	files                            []string
}

func browserFixture(t *testing.T, mode string) browserInputFixture {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "source repository")
	source := filepath.Join(repository, ".playwright", "chromium-1247")
	if err := os.MkdirAll(filepath.Join(source, "native"), 0o700); err != nil {
		t.Fatal(err)
	}
	for file, contents := range map[string]string{"INSTALLATION_COMPLETE": "", "native/browser": "native", "native/resource": "companion", "DEPENDENCIES_VALIDATED": "legacy"} {
		if err := os.WriteFile(filepath.Join(source, filepath.FromSlash(file)), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prefix := "browser_repo/.playwright/chromium-1247/"
	fixture := browserInputFixture{source: source, marker: prefix + "INSTALLATION_COMPLETE", prefix: prefix, files: []string{"native/browser", "native/resource"}}
	var manifest string
	for _, file := range append([]string{"INSTALLATION_COMPLETE"}, fixture.files...) {
		manifest += prefix + file + " " + filepath.Join(source, filepath.FromSlash(file)) + "\n"
	}
	fixture.manifest = manifest
	runfiles := t.TempDir()
	if mode == "directory" {
		if err := filetree.DirLink(repository, filepath.Join(runfiles, "browser_repo")); err != nil {
			t.Fatal(err)
		}
		t.Setenv("RUNFILES_MANIFEST_FILE", "")
		t.Setenv("RUNFILES_DIR", runfiles)
	} else {
		file := filepath.Join(runfiles, "MANIFEST")
		if err := os.WriteFile(file, []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("RUNFILES_DIR", "")
		t.Setenv("RUNFILES_MANIFEST_FILE", file)
	}
	return fixture
}

func (f browserInputFixture) writeLayout(t *testing.T, bundle string) string {
	t.Helper()
	contents, err := json.Marshal(map[string]any{"marker": f.marker, "files": f.files})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(bundle, "browser.json")
	if err := os.WriteFile(file, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func (f *browserInputFixture) addFiles(t *testing.T, names ...string) {
	t.Helper()
	f.files = append(f.files, names...)
	for _, name := range names {
		f.manifest += f.prefix + name + " " + filepath.Join(f.source, filepath.FromSlash(name)) + "\n"
	}
	if manifest := os.Getenv("RUNFILES_MANIFEST_FILE"); manifest != "" {
		if err := os.WriteFile(manifest, []byte(f.manifest), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
