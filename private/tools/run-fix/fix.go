package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/latticebuild/rules_js/go/arguments"
	"github.com/latticebuild/rules_js/go/process"
)

// RunFix enters only the workspace Bazel explicitly supplies for source edits.
func run(args []string) process.Result {
	var node, workspacePackage string
	tool, err := arguments.Parse("run-fix", args, map[string]*string{"node": &node, "workspace": &workspacePackage})
	if err != nil {
		return process.Failed(err)
	}
	if len(tool) == 0 {
		return process.Failed(errors.New("run-fix: expected a tool script"))
	}
	workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspace == "" {
		return process.Failed(errors.New("run this target with bazel run; BUILD_WORKSPACE_DIRECTORY is not set"))
	}
	cmd := process.Command(node, tool, nil)
	cmd.Dir = filepath.Join(workspace, workspacePackage)
	if info, err := os.Stat(cmd.Dir); err != nil {
		return process.Failed(fmt.Errorf("cannot enter %s: %w", cmd.Dir, err))
	} else if !info.IsDir() {
		return process.Failed(fmt.Errorf("cannot enter %s: not a directory", cmd.Dir))
	}
	return process.Supervise(cmd)
}
