package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/latticebuild/rules_js/go/filetree"
	"github.com/latticebuild/rules_js/js/private/tools/internal/testfs"
)

func TestRunCommandEnvironment(t *testing.T) {
	root := testfs.Root(t)
	script := `import fs from "node:fs";
fs.mkdirSync("out", {recursive:true});
fs.writeFileSync(process.argv[2], process.env.GREETING + " " + process.cwd());
if (process.env.NODE_OPTIONS || process.env.NODE_PATH) process.exit(9);
if (!process.env.LD_LIBRARY_PATH?.endsWith('/declared/loader')) process.exit(10);
`
	testfs.Write(t, filepath.Join(root, "tool.mjs"), script)
	t.Setenv("NODE", mustNode(t))
	t.Setenv("NODE_OPTIONS", "--invalid-inherited-option")
	t.Setenv("NODE_PATH", "/undeclared")
	loader := os.Getenv("LD_LIBRARY_PATH")
	if loader != "" {
		loader += string(os.PathListSeparator)
	}
	t.Setenv("LD_LIBRARY_PATH", loader+"/declared/loader")
	code, err := runAction(root, Manifest{
		Root:    "scratch",
		Files:   [][2]string{{"javascript/app/package.json", "javascript/app/package.json"}, {"tool.mjs", "tool.mjs"}},
		Cwd:     "javascript/app",
		Scripts: []string{"tool.mjs"},
		Args:    []string{"out/result.txt"},
		Env:     map[string]string{"GREETING": "hello"},
		Outputs: [][2]string{{"javascript/app/out", "published"}},
	})
	if err != nil || code != 0 {
		t.Fatalf("Run = %d, %v", code, err)
	}
	body, err := os.ReadFile(filepath.Join(root, "published/result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello "+filepath.Join(root, "scratch/javascript/app") {
		t.Fatalf("result = %s", body)
	}
	if _, err := os.Stat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
}

func TestRunFailurePublishesNothingAndCleansScratch(t *testing.T) {
	root := testfs.Root(t)
	testfs.Write(t, filepath.Join(root, "fail.mjs"), "import fs from 'node:fs'; fs.writeFileSync('ran',''); process.exit(3);\n")
	t.Setenv("NODE", mustNode(t))
	code, err := runAction(root, Manifest{
		Root: "scratch", Cwd: "",
		Files:   [][2]string{{"fail.mjs", "fail.mjs"}},
		Scripts: []string{"fail.mjs"},
		Outputs: [][2]string{{"ran", "published"}},
	})
	if err != nil || code != 3 {
		t.Fatalf("Run = %d, %v", code, err)
	}
	for _, rel := range []string{"scratch", "published"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("%s remains: %v", rel, err)
		}
	}
}

func TestRunPreservesPackageLinksAndInputs(t *testing.T) {
	root := testfs.Root(t)
	testfs.Mkdir(t, filepath.Join(root, "pkg/a"))
	testfs.Mkdir(t, filepath.Join(root, "pkg/b"))
	testfs.Write(t, filepath.Join(root, "pkg/a/package.json"), `{"main":"index.cjs"}`)
	testfs.Write(t, filepath.Join(root, "pkg/b/package.json"), `{"main":"index.cjs"}`)
	testfs.Write(t, filepath.Join(root, "pkg/a/index.cjs"), `exports.name="a"; exports.other=require("alias-b").name;`)
	testfs.Write(t, filepath.Join(root, "pkg/b/index.cjs"), `exports.name="b"; exports.other=require("alias-a").name;`)
	testfs.Write(t, filepath.Join(root, "tool.cjs"), `const fs=require("node:fs"); const a=require("alias-a"); const b=require("alias-b"); fs.writeFileSync("result",a.other+b.other); fs.writeFileSync(require.resolve("alias-a"),"mutated");`)
	t.Setenv("NODE", mustNode(t))
	files := [][2]string{{"tool.cjs", "tool.cjs"}}
	for _, rel := range []string{"pkg/a/package.json", "pkg/a/index.cjs", "pkg/b/package.json", "pkg/b/index.cjs"} {
		files = append(files, [2]string{rel, rel})
	}
	code, err := runAction(root, Manifest{
		Root: "scratch", Files: files,
		Links:   [][2]string{{"node_modules/alias-a", "../pkg/a"}, {"node_modules/alias-b", "../pkg/b"}, {"pkg/a/node_modules/alias-b", "../../b"}, {"pkg/b/node_modules/alias-a", "../../a"}},
		Scripts: []string{"tool.cjs"}, Outputs: [][2]string{{"result", "published"}},
	})
	if err != nil || code != 0 {
		t.Fatalf("Run = %d, %v", code, err)
	}
	body, err := os.ReadFile(filepath.Join(root, "published"))
	if err != nil || string(body) != "ba" {
		t.Fatalf("result = %s, %v", body, err)
	}
	body, err = os.ReadFile(filepath.Join(root, "pkg/a/index.cjs"))
	if err != nil || string(body) == "mutated" {
		t.Fatal("input changed")
	}
}

func runnerBinary(t *testing.T) string {
	t.Helper()
	location := os.Getenv("LATTICEBUILD_TEST_BINARY")
	if location == "" {
		t.Fatal("declared action runner missing from Bazel runfiles")
	}
	path, err := runfiles.Rlocation(location)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func mustNode(t *testing.T) string {
	t.Helper()
	node := os.Getenv("NODE")
	if node == "" {
		for _, path := range strings.Fields(os.Getenv("NODE_RUNFILES")) {
			if name := filepath.Base(path); name == "node" || name == "node.exe" {
				node = path
				break
			}
		}
	}
	if node != "" {
		if filepath.IsAbs(node) {
			return node
		}
		path, err := runfiles.Rlocation(node)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("declared Node executable missing: %v", err)
		}
		return path
	}
	t.Fatal("declared Node runtime missing from Bazel runfiles")
	return ""
}

func TestConcurrentActionsPreserveSharedInputs(t *testing.T) {
	root := testfs.Root(t)
	testfs.Write(t, filepath.Join(root, "tool.cjs"), `const fs=require("node:fs"); fs.writeFileSync("input","changed"); fs.writeFileSync("result",process.argv[2]);`)
	testfs.Write(t, filepath.Join(root, "input"), "original")
	t.Setenv("NODE", mustNode(t))
	start := make(chan struct{})
	failures := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Go(func() {
			<-start
			code, err := runAction(root, Manifest{Root: fmt.Sprintf("scratch-%d", i), Files: [][2]string{{"tool.cjs", "tool.cjs"}, {"input", "input"}}, Scripts: []string{"tool.cjs"}, Args: []string{fmt.Sprint(i)}, Outputs: [][2]string{{"result", fmt.Sprintf("result-%d", i)}}})
			if err != nil || code != 0 {
				failures <- fmt.Errorf("action %d: %d, %v", i, code, err)
			}
		})
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "input"))
	if err != nil || string(body) != "original" {
		t.Fatalf("input = %s, %v", body, err)
	}
	for i := range 2 {
		body, err := os.ReadFile(filepath.Join(root, fmt.Sprintf("result-%d", i)))
		if err != nil || string(body) != fmt.Sprint(i) {
			t.Fatalf("result %d = %s, %v", i, body, err)
		}
	}
}

func TestFreshStageSharedParentsAndWritableInputs(t *testing.T) {
	root := t.TempDir()
	files := [][2]string{{"tool.cjs", "tool.cjs"}}
	for i := range 96 {
		name := fmt.Sprintf("shared/group-%d/value-%d", i%3, i)
		testfs.Mkdir(t, filepath.Dir(filepath.Join(root, name)))
		testfs.Write(t, filepath.Join(root, name), fmt.Sprint(i))
		if err := os.Chmod(filepath.Join(root, name), 0o444); err != nil {
			t.Fatal(err)
		}
		files = append(files, [2]string{name, name})
	}
	testfs.Write(t, filepath.Join(root, "tool.cjs"), `const fs=require('node:fs');
let total=0; for(let i=0;i<96;i++) total+=Number(fs.readFileSync('shared/group-'+(i%3)+'/value-'+i));
fs.writeFileSync('shared/group-0/value-0','modified'); fs.writeFileSync('result',String(total));`)
	t.Setenv("NODE", mustNode(t))
	code, err := runAction(root, Manifest{Root: "scratch", Files: files, Scripts: []string{"tool.cjs"}, Outputs: [][2]string{{"result", "published"}}})
	if err != nil || code != 0 {
		t.Fatalf("fresh staging: %d %v", code, err)
	}
	for name, expected := range map[string]string{"published": "4560", "shared/group-0/value-0": "0"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(data) != expected {
			t.Fatalf("%s: %q %v", name, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatal("fresh scratch remains:", err)
	}
}

func TestRejectsEscapesAndOverlapsBeforeStaging(t *testing.T) {
	script := []string{"tool.cjs"}
	for name, c := range map[string]struct {
		manifest Manifest
		err      string
	}{
		"native scratch overlap": {Manifest{Root: "scratch", Executable: "scratch/tool", Scripts: script}, "native executable overlaps scratch"},
		"native output overlap":  {Manifest{Root: "scratch", Executable: "published/tool", Scripts: script, Outputs: [][2]string{{"output", "published"}}}, "native executable overlaps output"},
		"native escape":          {Manifest{Root: "scratch", Executable: "../tool", Scripts: script}, "leaves the tree"},
		"root replacement":       {Manifest{Root: ".", Files: [][2]string{{"x", "input"}}, Scripts: script}, "tree path . leaves the tree"},
		"empty destination":      {Manifest{Root: "scratch", Files: [][2]string{{"", "input"}}, Scripts: script}, "tree path  leaves the tree"},
		"overlap":                {Manifest{Root: "scratch", Files: [][2]string{{"a", "input"}, {"a-b", "input"}, {"a/b", "input"}}, Scripts: script}, "overlapping destinations"},
		"conflicting duplicate":  {Manifest{Root: "scratch", Files: [][2]string{{"a", "input"}, {"a", "other"}}, Scripts: script}, "conflicting destinations at a"},
		"link escape":            {Manifest{Root: "scratch", Links: [][2]string{{"x", "../outside"}}, Scripts: script}, "link x leaves the tree"},
		"source escape":          {Manifest{Root: "scratch", Files: [][2]string{{"a", "../input"}}, Scripts: script}, "tree path ../input leaves the tree"},
		"script escape":          {Manifest{Root: "scratch", Scripts: []string{"../tool.cjs"}}, "tree path ../tool.cjs leaves the tree"},
		"no script":              {Manifest{Root: "scratch", Files: [][2]string{{"a", "input"}}}, "manifest names no script"},
		"output input overlap":   {Manifest{Root: "scratch", Files: [][2]string{{"a", "input"}}, Outputs: [][2]string{{"a", "output"}}, Scripts: script}, "output a overlaps input a"},
		"output storage overlap": {Manifest{Root: "scratch", Files: [][2]string{{"a", "input"}}, Outputs: [][2]string{{"b", "input"}}, Scripts: script}, "output b overlaps input a"},
	} {
		t.Run(name, func(t *testing.T) {
			root := testfs.Root(t)
			testfs.Write(t, filepath.Join(root, "input"), "original")
			if _, err := runAction(root, c.manifest); err == nil || !strings.Contains(err.Error(), c.err) {
				t.Fatalf("error = %v; want %q", err, c.err)
			}
			body, err := os.ReadFile(filepath.Join(root, "input"))
			if err != nil || string(body) != "original" {
				t.Fatal("input damaged")
			}
		})
	}
}

func TestRunRefusesInputAliasesToOutputStorage(t *testing.T) {
	for _, output := range []string{"output/value.txt", "output", "output/new/value.txt"} {
		t.Run(output, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("NODE", mustNode(t))
			testfs.Mkdir(t, filepath.Join(root, "output"))
			testfs.Write(t, filepath.Join(root, "output/value.txt"), "original\n")
			testfs.Write(t, filepath.Join(root, "tool.cjs"), "throw new Error('must not run');\n")
			if err := filetree.DirLink(filepath.Join(root, "output"), filepath.Join(root, "input")); err != nil {
				t.Fatal(err)
			}
			code, err := runAction(root, Manifest{
				Root: "scratch",
				Files: [][2]string{
					{"tool.cjs", "tool.cjs"},
					{"source", "input"},
				},
				Scripts: []string{"tool.cjs"},
				Outputs: [][2]string{{"result", output}},
			})
			if code == 0 || err == nil || !strings.Contains(err.Error(), "overlaps input storage") {
				t.Fatalf("input alias accepted: %d %v", code, err)
			}
			body, err := os.ReadFile(filepath.Join(root, "output/value.txt"))
			if err != nil || string(body) != "original\n" {
				t.Fatalf("input changed: %q %v", body, err)
			}
			for _, name := range []string{"scratch", "output/new"} {
				if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected write to %s: %v", name, err)
				}
			}
		})
	}
}

func TestRunProtectsNonstagedReadInputs(t *testing.T) {
	for _, kind := range []string{"node", "native executable", "inventory", "status"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("NODE", mustNode(t))
			testfs.Write(t, filepath.Join(root, "tool.cjs"), "throw new Error('must not run');\n")
			body := "protected\n"
			m := Manifest{
				Root: "scratch", Files: [][2]string{{"tool.cjs", "tool.cjs"}},
				Scripts: []string{"tool.cjs"}, Outputs: [][2]string{{"result", "backing/read.txt"}},
			}
			switch kind {
			case "node":
				t.Setenv("NODE", filepath.Join(root, "alias/read.txt"))
			case "native executable":
				m.Executable = "alias/read.txt"
			case "inventory":
				body = "\"tool.cjs\"\n"
				m.InputList = "alias/read.txt"
			case "status":
				body = "STABLE_VERSION value\n"
				m.StatusFile = "alias/read.txt"
			}
			testfs.Mkdir(t, filepath.Join(root, "backing"))
			testfs.Write(t, filepath.Join(root, "backing/read.txt"), body)
			if err := filetree.DirLink(filepath.Join(root, "backing"), filepath.Join(root, "alias")); err != nil {
				t.Fatal(err)
			}
			code, err := runAction(root, m)
			if code == 0 || err == nil || !strings.Contains(err.Error(), "overlaps protected input") {
				t.Fatalf("protected read accepted: %d %v", code, err)
			}
			actual, err := os.ReadFile(filepath.Join(root, "backing/read.txt"))
			if err != nil || string(actual) != body {
				t.Fatalf("protected input changed: %q %v", actual, err)
			}
			if _, err := os.Lstat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
				t.Fatalf("scratch created: %v", err)
			}
		})
	}
}

func TestInputSymlinkCycleIsBounded(t *testing.T) {
	root := testfs.Root(t)
	t.Setenv("NODE", mustNode(t))
	testfs.Mkdir(t, filepath.Join(root, "cycle"))
	if err := os.Symlink(".", filepath.Join(root, "cycle/again")); err != nil {
		t.Fatal(err)
	}
	_, err := runAction(root, Manifest{Root: "scratch", Files: [][2]string{{"cycle", "cycle"}}, Scripts: []string{"tool.cjs"}})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
}

func TestDirectoryInputCannotImportUndeclaredSymlinkContents(t *testing.T) {
	root := testfs.Root(t)
	t.Setenv("NODE", mustNode(t))
	testfs.Mkdir(t, filepath.Join(root, "artifact"))
	testfs.Write(t, filepath.Join(root, "undeclared"), "outside artifact")
	if err := os.Symlink("../undeclared", filepath.Join(root, "artifact/escape")); err != nil {
		t.Fatal(err)
	}
	_, err := runAction(root, Manifest{Root: "scratch", Files: [][2]string{{"artifact", "artifact"}}, Scripts: []string{"tool.cjs"}})
	if err == nil || !strings.Contains(err.Error(), "outside declared") {
		t.Fatalf("escape = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("scratch remains: %v", err)
	}
}
