package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "write-svelte-kit:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 {
		return errors.New("expected a workspace package and generated source directory")
	}
	workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspace == "" {
		return errors.New("run this target with bazel run; BUILD_WORKSPACE_DIRECTORY is not set")
	}
	return write(workspace, args[0], args[1])
}
