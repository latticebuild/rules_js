package main

import (
	"fmt"
	"os"
)

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "filter-runtime:", err)
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
	if len(args) < 2 {
		return 2, fmt.Errorf("usage: filter-runtime <source> <destination> [inventory...]")
	}
	return 0, filterDirectory(root, args[0], args[1], args[2:]...)
}
