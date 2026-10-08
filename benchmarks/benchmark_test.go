package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func validSpawn() spawn {
	s := spawn{Target: "//:none", Mnemonic: "BenchmarkNode", Runner: nativeRunner()}
	s.Outputs = []string{"bazel-out/native-fastbuild/bin/none/result.json"}
	s.Metrics.Total, s.Metrics.Execution = "0.024s", "0.019s"
	return s
}

func TestActionEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*spawn)
	}{
		{"cache hit", func(s *spawn) { s.Cached = true }},
		{"failed", func(s *spawn) { s.ExitCode = 1 }},
		{"status failure", func(s *spawn) { s.Status = "TIMEOUT" }},
		{"wrong runner", func(s *spawn) { s.Runner = "remote" }},
		{"missing timing", func(s *spawn) { s.Metrics.Total = "" }},
		{"malformed timing", func(s *spawn) { s.Metrics.Total = "nan" }},
		{"zero timing", func(s *spawn) { s.Metrics.Total = "0s" }},
		{"wrong target", func(s *spawn) { s.Target = "//:many" }},
		{"wrong mnemonic", func(s *spawn) { s.Mnemonic = "CopyFile" }},
		{"wrong output", func(s *spawn) { s.Outputs = []string{"none/other.json"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := validSpawn()
			test.edit(&s)
			if _, err := actionMeasurement([]spawn{s}, "none", nativeRunner()); err == nil {
				t.Fatal("invalid sample qualified")
			}
		})
	}
	s := validSpawn()
	if _, err := actionMeasurement(nil, "none", nativeRunner()); err == nil {
		t.Fatal("missing action qualified")
	}
	if _, err := actionMeasurement([]spawn{s, s}, "none", nativeRunner()); err == nil {
		t.Fatal("duplicate action qualified")
	}
	prerequisite := s
	prerequisite.Mnemonic = "CopyFile"
	m, err := actionMeasurement([]spawn{prerequisite, s}, "none", nativeRunner())
	if err != nil || m.Total != 24_000_000 || m.Execution != 19_000_000 || m.Prerequisites != 24_000_000 {
		t.Fatalf("incorrect action/prerequisite aggregation: %+v, %v", m, err)
	}
}

func TestExecutionLogStream(t *testing.T) {
	s, _ := json.Marshal(validSpawn())
	rows, err := readSpawns(strings.NewReader(string(s) + "\n" + string(s)))
	if err != nil || len(rows) != 2 {
		t.Fatalf("stream: %v %v", rows, err)
	}
	if _, err := readSpawns(strings.NewReader(string(s) + "\n{")); err == nil {
		t.Fatal("truncated log accepted")
	}
}

func TestPairedConfidenceGate(t *testing.T) {
	for _, ratio := range []float64{0.5, 1, 2} {
		pairs := make([]pair, 30)
		for i := range pairs {
			pairs[i].Aspect.Total = int64(100+i) * 1_000_000
			pairs[i].Lattice.Total = int64(float64(pairs[i].Aspect.Total) * ratio)
		}
		a, b := summarize(pairs), summarize(pairs)
		if a != b || a.Ratio != ratio || a.Lower != ratio || a.Upper != ratio || a.Passed != (ratio < 1) {
			t.Fatalf("deterministic paired ratio %g: %+v", ratio, a)
		}
		if summarize(pairs[:1]).Passed {
			t.Fatal("a probe qualified performance")
		}
	}
	pairs := make([]pair, 30)
	for i := range pairs {
		pairs[i].Aspect.Total = 100_000_000
		pairs[i].Lattice.Total = 90_000_000
		if i%2 == 1 {
			pairs[i].Lattice.Total = 110_000_000
		}
	}
	if summarize(pairs).Passed {
		t.Fatal("tie/noisy interval qualified")
	}
}

func sampleInventory() inventory {
	r := inventory{Node: "v26.8.2"}
	for i := range 40 {
		r.Instances = append(r.Instances, packageInstance{Name: fmt.Sprintf("package-%d", i), Version: "1.0.0", Bindings: [][]string{}, Files: []payloadFile{{"index.js", strings.Repeat("a", 64), 20}}})
	}
	return r
}

func TestFullPayloadParity(t *testing.T) {
	l, a := sampleInventory(), sampleInventory()
	l.Instances[0].Files = append(l.Instances[0].Files, payloadFile{"README.md", strings.Repeat("b", 64), 10})
	r, err := inventoryComparison(l, a)
	if err != nil || r.Instances != 40 || r.CommonFiles != 40 || r.LatticeFiles != 41 || len(r.Excluded) != 1 {
		t.Fatalf("preset parity: %+v %v", r, err)
	}
	a.Instances[0].Files[0].SHA256 = strings.Repeat("c", 64)
	if _, err := inventoryComparison(l, a); err == nil {
		t.Fatal("changed runtime bytes accepted")
	}
	a = sampleInventory()
	l.Instances[0].Files = append(l.Instances[0].Files, payloadFile{"dist/runtime.js", strings.Repeat("d", 64), 15})
	if _, err := inventoryComparison(l, a); err == nil {
		t.Fatal("undeclared runtime exclusion accepted")
	}
	l, a = sampleInventory(), sampleInventory()
	a.Instances[0].Bindings = [][]string{{"peerDependencies", "vite", "vite@1.0.0"}}
	if _, err := inventoryComparison(l, a); err == nil {
		t.Fatal("peer mismatch accepted")
	}
	l, a = sampleInventory(), sampleInventory()
	l.Instances = append(l.Instances, l.Instances[0])
	if _, err := inventoryComparison(l, a); err == nil {
		t.Fatal("physical peer instance count was collapsed")
	}
	for _, data := range []string{`["../outside","digest",1]`, `["index.js","digest",-1]`, `["index.js"]`} {
		var file payloadFile
		if err := json.Unmarshal([]byte(data), &file); err == nil {
			t.Fatal("invalid payload accepted:", data)
		}
	}
}

func TestFixturesAndOwnedCleanup(t *testing.T) {
	root := t.TempDir()
	left, right := map[string]string{}, map[string]string{}
	for i, name := range []string{"lattice", "aspect"} {
		hashes := left
		if i == 1 {
			hashes = right
		}
		if err := prepareFixture(filepath.Join(root, name), name, root, "0.1.1", hashes); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"action.mjs", "pnpm-lock.yaml", "package.json", "pnpm-workspace.yaml"} {
		if left[name] != right[name] {
			t.Fatal("different paired fixture:", name)
		}
	}
	sentinel := filepath.Join(root, "source", "file")
	if err := os.Mkdir(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("preserve"), 0o444); err != nil {
		t.Fatal(err)
	}
	owned := filepath.Join(root, "owned")
	if err := os.Mkdir(owned, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(sentinel), filepath.Join(owned, "external")); err != nil {
		// Windows installations without symlink privilege still exercise
		// read-only owned-directory cleanup below.
		t.Log("symlink probe unavailable:", err)
	}
	if err := os.Chmod(owned, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := removeOwned(owned); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || !reflect.DeepEqual(data, []byte("preserve")) {
		t.Fatal("cleanup touched source storage")
	}
}

func TestCommandCancellation(t *testing.T) {
	t.Setenv("LATTICEBUILD_BENCHMARK_CHILD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := command(ctx, t.TempDir(), filepath.Join(t.TempDir(), "child.log"), os.Args[0], "-test.run=^TestBenchmarkChild$"); err == nil {
		t.Fatal("cancelled command reported success")
	}
}

func TestBenchmarkChild(t *testing.T) {
	if os.Getenv("LATTICEBUILD_BENCHMARK_CHILD") == "1" {
		time.Sleep(time.Minute)
	}
}
