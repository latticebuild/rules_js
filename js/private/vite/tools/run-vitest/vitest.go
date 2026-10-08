package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/latticebuild/rules_js/go/arguments"
	"github.com/latticebuild/rules_js/go/junit"
	"github.com/latticebuild/rules_js/go/lcov"
	"github.com/latticebuild/rules_js/go/process"
	"github.com/latticebuild/rules_js/go/testenv"
)

func run(args []string) process.Result {
	var node, mapFile, browserLayout string
	tool, err := arguments.Parse("run-vitest", args, map[string]*string{"node": &node, "coverage": &mapFile, "browser-layout": &browserLayout})
	if err != nil {
		return process.Failed(err)
	}
	if len(tool) == 0 {
		return process.Failed(errors.New("run-vitest: expected the Vitest script"))
	}
	coverage, err := lcov.Prepare(mapFile)
	if err != nil {
		return process.Failed(err)
	}
	env, err := testenv.Read(os.LookupEnv)
	if err != nil {
		return process.Failed(err)
	}
	changes := map[string]string{}
	if browserLayout != "" {
		root, err := prepareBrowser(browserLayout)
		if err != nil {
			return process.Failed(err)
		}
		changes["PLAYWRIGHT_BROWSERS_PATH"] = root
	}
	if runtime.GOOS == "windows" {
		// Node worker threads preserve case-sensitive copies of the environment.
		// Execa and cross-spawn look up the Windows shell using this spelling.
		if shell := os.Getenv("COMSPEC"); shell != "" {
			changes["comspec"] = shell
		}
	}
	if coverage != nil {
		env.Coverage = coverage.Collected
		changes["COVERAGE_DIR"] = env.Coverage
	}
	var partial string
	destination := env.XML
	if env.XML != "" && !supplied(tool[1:], "--outputFile", "--output-file", "--outputFile.junit", "--output-file.junit") {
		partial, err = junit.Temporary(env.XML)
		if err != nil {
			return process.Failed(err)
		}
		defer func() { _ = os.Remove(partial) }()
		env.XML = partial
	}
	argv, err := vitestArguments(tool[1:], env)
	if err != nil {
		return process.Failed(err)
	}
	result := process.Supervise(process.Command(node, append([]string{tool[0]}, argv...), changes, "NODE_V8_COVERAGE"))
	if result.Signal == nil && result.Err == nil && partial != "" {
		result.Err = junit.Publish(partial, destination)
	}
	if result.Signal == nil && result.Code == 0 && result.Err == nil && coverage != nil {
		result.Err = coverage.Publish()
	}
	return result
}

func supplied(args []string, names ...string) bool {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		for _, name := range names {
			if arg == name || strings.HasPrefix(arg, name+"=") {
				return true
			}
		}
	}
	return false
}

func vitestArguments(args []string, env testenv.Environment) ([]string, error) {
	var defaults []string
	// Bazel schedules concurrent test actions on the same worker. Let each
	// action consume one Vitest worker unless its caller budgets more.
	if !supplied(args, "--maxWorkers", "--max-workers") {
		defaults = append(defaults, "--maxWorkers=1")
	}
	if env.XML != "" {
		defaults = append(defaults, "--reporter=default", "--reporter=junit")
		if !supplied(args, "--outputFile", "--output-file", "--outputFile.junit", "--output-file.junit") {
			defaults = append(defaults, "--outputFile.junit="+env.XML)
		}
	}
	if env.Coverage != "" {
		if supplied(args, "--coverage.reportsDirectory") {
			return nil, errors.New("run-vitest: Bazel coverage chooses --coverage.reportsDirectory; omit it under bazel coverage")
		}
		if !supplied(args, "--coverage", "--coverage.enabled") {
			defaults = append(defaults, "--coverage.enabled=true")
		}
		defaults = append(defaults, "--coverage.reporter=lcovonly", "--coverage.reportsDirectory="+env.Coverage)
	}
	if env.Outputs != "" {
		if env.Coverage == "" && !supplied(args, "--coverage.reportsDirectory") {
			defaults = append(defaults, "--coverage.reportsDirectory="+filepath.Join(env.Outputs, "coverage"))
		}
		if !supplied(args, "--browser.screenshotDirectory") {
			defaults = append(defaults, "--browser.screenshotDirectory="+filepath.Join(env.Outputs, "screenshots"))
		}
	}
	if env.Filter != "" && !supplied(args, "--testNamePattern", "--test-name-pattern", "-t") {
		defaults = append(defaults, "--testNamePattern="+env.Filter)
	}
	if env.Total != 0 {
		if supplied(args, "--shard") {
			return nil, errors.New("use Bazel shard_count instead of --shard when Bazel sharding is enabled")
		}
		defaults = append(defaults, fmt.Sprintf("--shard=%d/%d", env.Shard, env.Total))
	}
	separator := len(args)
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	result := append([]string{}, args[:separator]...)
	result = append(result, defaults...)
	return append(result, args[separator:]...), nil
}
