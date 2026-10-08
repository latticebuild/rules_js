package main

import (
	"strings"
	"testing"
)

func TestWriteRefusesInvalidInvocation(t *testing.T) {
	workspace, source := t.TempDir(), t.TempDir()
	for _, test := range []struct {
		name      string
		workspace string
		args      []string
		message   string
	}{
		{"outside bazel run", "", []string{".", source}, "BUILD_WORKSPACE_DIRECTORY"},
		{"missing arguments", workspace, nil, "expected a workspace package"},
		{"extra arguments", workspace, []string{".", source, "unexpected"}, "expected a workspace package"},
		{"relative workspace", "relative", []string{".", source}, "absolute workspace"},
		{"escaping package", workspace, []string{"../outside", source}, "local package path"},
		{"absolute package", workspace, []string{workspace, source}, "local package path"},
		{"missing package", workspace, []string{"absent/package", source}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("BUILD_WORKSPACE_DIRECTORY", test.workspace)
			err := run(test.args)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected refusal containing %q, got %v", test.message, err)
			}
		})
	}
}
