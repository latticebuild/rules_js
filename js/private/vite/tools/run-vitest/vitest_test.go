package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/latticebuild/rules_js/go/testenv"
)

func TestVitestDefaultsAndSeparator(t *testing.T) {
	env := testenv.Environment{XML: "result.xml", Outputs: "outputs", Filter: "from bazel", Shard: 2, Total: 4}
	got, err := vitestArguments([]string{"run", "--output-file=custom.xml", "-t", "mine", "--", "--shard=3/4"}, env)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--reporter=junit", "--shard=2/4", "--coverage.reportsDirectory=" + filepath.Join("outputs", "coverage"), "--browser.screenshotDirectory=" + filepath.Join("outputs", "screenshots")} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	for _, arg := range got {
		if strings.HasPrefix(arg, "--outputFile.junit=") || strings.HasPrefix(arg, "--testNamePattern=") {
			t.Errorf("overrode caller: %s", arg)
		}
	}
	if i := slices.Index(got, "--"); i != len(got)-2 {
		t.Fatalf("defaults after separator: %v", got)
	}
	for _, args := range [][]string{{"--shard", "1/4"}, {"--shard=1/4"}} {
		if _, err := vitestArguments(args, env); err == nil {
			t.Fatal("accepted duplicate shard")
		}
	}
	env.Coverage = "private"
	if _, err := vitestArguments([]string{"--coverage.reportsDirectory=elsewhere"}, env); err == nil {
		t.Fatal("accepted foreign coverage directory")
	}
	got, err = vitestArguments([]string{"--coverage=false"}, env)
	if err != nil || slices.Contains(got, "--coverage.enabled=true") {
		t.Fatalf("coverage override: %v %v", got, err)
	}
}

func TestVitestWorkerBudgetHonorsCallerAndSeparator(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"default", []string{"run"}, []string{"run", "--maxWorkers=1"}},
		{"camel split", []string{"run", "--maxWorkers", "3"}, []string{"run", "--maxWorkers", "3"}},
		{"camel equals", []string{"run", "--maxWorkers=3"}, []string{"run", "--maxWorkers=3"}},
		{"kebab split", []string{"run", "--max-workers", "3"}, []string{"run", "--max-workers", "3"}},
		{"kebab equals", []string{"run", "--max-workers=3"}, []string{"run", "--max-workers=3"}},
		{"filter after separator", []string{"run", "--", "--maxWorkers=3"}, []string{"run", "--maxWorkers=1", "--", "--maxWorkers=3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := vitestArguments(tc.args, testenv.Environment{})
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("Vitest arguments = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}
