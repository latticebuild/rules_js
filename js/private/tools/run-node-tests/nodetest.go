package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/latticebuild/rules_js/go/arguments"
	"github.com/latticebuild/rules_js/go/junit"
	"github.com/latticebuild/rules_js/go/lcov"
	"github.com/latticebuild/rules_js/go/process"
	"github.com/latticebuild/rules_js/go/testenv"
)

// RunNodeTests delegates discovery and reporters to the declared Node CLI.
func run(args []string) process.Result {
	var node, mapFile string
	files, err := arguments.Parse("run-node-tests", args, map[string]*string{"node": &node, "coverage": &mapFile})
	if err != nil {
		return process.Failed(err)
	}
	if len(files) == 0 {
		return process.Failed(errors.New("js_test requires test files"))
	}
	for _, file := range files {
		if strings.HasPrefix(file, "-") {
			return process.Failed(fmt.Errorf("js_test takes test files, not %s; filter with --test_filter", file))
		}
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
	if coverage != nil {
		env.Coverage = coverage.Collected
		changes["COVERAGE_DIR"] = env.Coverage
	}
	argv := []string{"--test", "--test-reporter=spec", "--test-reporter-destination=stdout"}
	var partial string
	if env.XML != "" {
		partial, err = junit.Temporary(env.XML)
		if err != nil {
			return process.Failed(err)
		}
		defer func() { _ = os.Remove(partial) }()
		argv = append(argv, "--test-reporter=junit", "--test-reporter-destination="+partial)
	}
	if env.Coverage != "" {
		argv = append(argv, "--experimental-test-coverage", "--test-reporter=lcov", "--test-reporter-destination="+filepath.Join(env.Coverage, "node.info"))
	}
	if env.Filter != "" {
		argv = append(argv, "--test-name-pattern="+env.Filter)
	}
	if env.Total != 0 {
		argv = append(argv, fmt.Sprintf("--test-shard=%d/%d", env.Shard, env.Total))
	}
	argv = append(argv, "--")
	argv = append(argv, files...)
	result := process.Supervise(process.Command(node, argv, changes, "NODE_TEST_CONTEXT", "NODE_V8_COVERAGE"))
	if result.Signal == nil && result.Err == nil && partial != "" {
		result.Err = junit.Publish(partial, env.XML)
	}
	if result.Signal == nil && result.Code == 0 && result.Err == nil && coverage != nil {
		result.Err = coverage.Publish()
	}
	return result
}
