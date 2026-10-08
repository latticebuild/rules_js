package main

import (
	"fmt"
	"os"

	"github.com/latticebuild/rules_js/private/tools/internal/action"
)

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "run-action:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
func run(args []string) (int, error) {
	root, err := os.Getwd()
	if err != nil {
		return 1, err
	}
	if len(args) != 1 {
		return 2, fmt.Errorf("usage: run-action <manifest.json>")
	}
	m, err := action.LoadManifest(args[0])
	if err != nil {
		return 1, err
	}
	return action.Run(root, m)
}
