package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// statusKey matches a stable workspace status key named in an environment
// value, such as {STABLE_RELEASE_VERSION}.
var statusKey = regexp.MustCompile(`\{STABLE_[A-Z0-9_]+\}`)

// stampEnv returns env with every {STABLE_KEY} replaced by that key's value in
// Bazel's stable workspace status file at path. A key the file lacks is an
// error, because a stamped build must never publish the placeholder itself.
func stampEnv(path string, env map[string]string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("workspace status: %w", err)
	}
	status := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, _ := strings.Cut(strings.TrimRight(line, "\r"), " ")
		if key != "" {
			status[key] = value
		}
	}
	stamped := make(map[string]string, len(env))
	for name, value := range env {
		missing := ""
		stamped[name] = statusKey.ReplaceAllStringFunc(value, func(placeholder string) string {
			key := strings.Trim(placeholder, "{}")
			if found, ok := status[key]; ok {
				return found
			}
			missing = key
			return placeholder
		})
		if missing != "" {
			return nil, fmt.Errorf("environment %s names %s, which the workspace status lacks; stamp with --config=release", name, missing)
		}
	}
	return stamped, nil
}
