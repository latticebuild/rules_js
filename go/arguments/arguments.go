package arguments

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

func Parse(name string, args []string, fields map[string]*string) ([]string, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	for key, dest := range fields {
		flags.StringVar(dest, key, "", key)
	}
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if node := fields["node"]; node == nil || !filepath.IsAbs(*node) {
		return nil, fmt.Errorf("%s: --node must name the absolute declared Node executable", name)
	}
	return flags.Args(), nil
}
