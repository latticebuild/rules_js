package lcov

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/latticebuild/rules_js/go/filetree"
)

type Collection struct {
	Collected, destination, tree, cwd string
	reported                          map[string]string
}

func Prepare(mapFile string) (*Collection, error) {
	destination := os.Getenv("COVERAGE_DIR")
	if destination == "" || mapFile == "" {
		return nil, nil
	}
	contents, err := os.ReadFile(mapFile)
	if err != nil {
		return nil, fmt.Errorf("coverage map %s: %w", mapFile, err)
	}
	var reported map[string]string
	if err := json.Unmarshal(contents, &reported); err != nil {
		return nil, fmt.Errorf("coverage map %s: %w", mapFile, err)
	}
	tree, err := filepath.Abs(filepath.Join(filepath.Dir(mapFile), "tree"))
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	collected := filepath.Join(tree, ".coverage")
	if err := os.MkdirAll(collected, 0o700); err != nil {
		return nil, err
	}
	return &Collection{collected, destination, tree, cwd, reported}, nil
}

func (c *Collection) Publish() error {
	entries, err := os.ReadDir(c.Collected)
	if err != nil {
		return err
	}
	var outputs []struct{ name, text string }
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		text, err := os.ReadFile(filepath.Join(c.Collected, entry.Name()))
		if err != nil {
			return err
		}
		translated, err := translateTracefile(string(text), c.tree, c.cwd, c.reported)
		if err != nil {
			return fmt.Errorf("tracefile %s: %w", entry.Name(), err)
		}
		if translated == "" {
			continue
		}
		name := "js-" + strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) + ".dat"
		outputs = append(outputs, struct{ name, text string }{name, translated})
	}
	// Validate all traces before publishing any; a malformed later file must
	// not leave apparently successful partial coverage in Bazel's directory.
	for _, output := range outputs {
		if err := os.WriteFile(filepath.Join(c.destination, output.name), []byte(output.text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func translateTracefile(text, tree, cwd string, reported map[string]string) (string, error) {
	roots, bases := resolvedPaths(tree), resolvedPaths(cwd)
	var kept, record strings.Builder
	var source string
	inRecord, keep := false, false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "SF:") {
			if inRecord {
				return "", fmt.Errorf("record for %s has no end_of_record", source)
			}
			source, inRecord = strings.TrimPrefix(line, "SF:"), true
			found, ok := reportedPath(source, roots, bases, reported)
			keep = ok
			if ok {
				line = "SF:" + found
			}
		} else if line == "end_of_record" {
			if !inRecord {
				return "", fmt.Errorf("end_of_record outside a record")
			}
			if keep {
				kept.WriteString(record.String())
				kept.WriteString(line + "\n")
			}
			record.Reset()
			source, inRecord = "", false
			continue
		} else if line == "" && !inRecord {
			continue
		}
		record.WriteString(line + "\n")
	}
	if inRecord {
		return "", fmt.Errorf("record for %s has no end_of_record", source)
	}
	return kept.String(), nil
}

func resolvedPaths(directory string) []string {
	paths := []string{filepath.Clean(directory)}
	if real, err := filetree.RealPath(directory); err == nil && real != paths[0] {
		paths = append(paths, real)
	}
	return paths
}

func reportedPath(source string, roots, bases []string, reported map[string]string) (string, bool) {
	var candidates []string
	if filepath.IsAbs(source) {
		candidates = []string{filepath.Clean(source)}
	} else {
		for _, base := range bases {
			candidates = append(candidates, filepath.Join(base, source))
		}
	}
	for _, candidate := range candidates {
		for _, root := range roots {
			relative, err := filepath.Rel(root, candidate)
			if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				continue
			}
			if value, ok := reported[filepath.ToSlash(relative)]; ok {
				return value, true
			}
		}
	}
	return "", false
}
