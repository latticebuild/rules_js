// Package tsc checks the declared project contract using compiler CLIs.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

var outputOptions = []string{"rootDir", "outDir", "declaration", "declarationMap", "sourceMap", "inlineSourceMap", "emitDeclarationOnly", "composite", "incremental", "tsBuildInfoFile", "declarationDir", "outFile", "noEmit", "allowJs", "checkJs", "jsx", "emitBOM", "noCheck", "resolveJsonModule"}

// Run receives the staged compiler script followed by the rule's project payload.
// Its children inherit the action runner's process group/job. This package must
// not establish a second process ownership boundary.
func run(args []string) error {
	if len(args) != 8 {
		return fmt.Errorf("usage: compile-typescript <compiler> <config> <options-json> <sources-json> <directories-json> <outputs-json> <typescript-package> <stamp>")
	}
	node := os.Getenv("NODE")
	if !filepath.IsAbs(node) {
		return errors.New("NODE must name the absolute declared Node toolchain executable")
	}
	var options map[string]any
	var sources, directories, outputs []string
	for _, item := range []struct {
		text  string
		value any
	}{{args[2], &options}, {args[3], &sources}, {args[4], &directories}, {args[5], &outputs}} {
		if err := json.Unmarshal([]byte(item.text), item.value); err != nil {
			return err
		}
	}
	problems, err := CheckOptions(node, args[1], options, args[6])
	if err != nil {
		return err
	}
	if len(problems) == 0 {
		problems, err = compile(node, args[1], args[0], sources, directories, outputs)
		if err != nil {
			return err
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s:\n  %s", args[1], strings.Join(problems, "\n  "))
	}
	return os.WriteFile(args[7], nil, 0o644)
}

type configuration struct {
	Options    map[string]any    `json:"compilerOptions"`
	Files      []string          `json:"files"`
	References []json.RawMessage `json:"references"`
}

// CheckOptions compares only output-affecting and explicitly declared options.
func CheckOptions(node, config string, expected map[string]any, packageDirectory string) (problems []string, resultErr error) {
	directory, err := filepath.Abs(filepath.Dir(config))
	if err != nil {
		return nil, err
	}
	compiler, err := validationCompiler(packageDirectory)
	if err != nil {
		return nil, err
	}
	actual, err := inspect(node, compiler, config)
	if err != nil {
		return []string{err.Error()}, nil
	}
	if len(actual.References) != 0 {
		return []string{"TypeScript project references are unsupported; declare ordinary package dependencies"}, nil
	}
	if len(actual.Files) == 0 {
		return []string{"TypeScript project has no sources"}, nil
	}
	source := actual.Files[0]
	if !filepath.IsAbs(source) {
		source = filepath.Join(directory, source)
	}
	if err := normalizeResetPaths(node, compiler, config, directory, source, actual.Options); err != nil {
		return nil, err
	}
	normalized := make(map[string]any, len(expected))
	for name, value := range expected {
		if value != nil {
			normalized[name] = value
		}
	}
	// The probe stays beside the original, so all relative options have the same
	// base. Explicit files keep --showConfig from rejecting an empty probe.
	probe, err := os.CreateTemp(directory, ".latticebuild-compiler-options-*.json")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, os.Remove(probe.Name())) }()
	encodeErr := json.NewEncoder(probe).Encode(map[string]any{"compilerOptions": normalized, "files": []string{source}})
	if err := errors.Join(encodeErr, probe.Close()); err != nil {
		return nil, err
	}
	wanted, err := inspect(node, compiler, probe.Name())
	if err != nil {
		return []string{"declared compiler_options: " + err.Error()}, nil
	}
	normalizeOptions(actual.Options, directory)
	normalizeOptions(wanted.Options, directory)
	keys := slices.Clone(outputOptions)
	for key := range expected {
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		a, b := actual.Options[key], wanted.Options[key]
		if !reflect.DeepEqual(a, b) {
			problems = append(problems, fmt.Sprintf("compilerOptions.%s is %s, declared %s", key, show(a, directory), show(b, directory)))
		}
	}
	return problems, nil
}

// TypeScript 6 serializes an explicitly reset scalar path as the configuration
// directory. Inspecting an inheriting configuration in another directory tells
// this apart from a real path: only the reset value follows the new directory.
func normalizeResetPaths(node, compiler, config, directory, source string, options map[string]any) (resultErr error) {
	var ambiguous []string
	for _, key := range []string{"rootDir", "outDir", "declarationDir", "outFile", "tsBuildInfoFile", "baseUrl"} {
		if path, ok := options[key].(string); ok && optionPath(path, directory) == directory {
			ambiguous = append(ambiguous, key)
		}
	}
	if len(ambiguous) == 0 {
		return nil
	}
	probe, err := os.MkdirTemp(directory, ".latticebuild-compiler-config-*")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, os.RemoveAll(probe)) }()
	contents, err := json.Marshal(map[string]any{"extends": filepath.Join(directory, filepath.Base(config)), "files": []string{source}})
	if err != nil {
		return err
	}
	path := filepath.Join(probe, "tsconfig.json")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return err
	}
	inherited, err := inspect(node, compiler, path)
	if err != nil {
		return fmt.Errorf("inspect reset compiler paths: %w", err)
	}
	for _, key := range ambiguous {
		if path, ok := inherited.Options[key].(string); ok && optionPath(path, probe) == probe {
			delete(options, key)
		}
	}
	return nil
}

func optionPath(path, directory string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(directory, path)
	}
	return filepath.Clean(path)
}

func validationCompiler(directory string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(directory, "package.json"))
	if err != nil {
		return "", err
	}
	var manifest struct {
		Bin json.RawMessage `json:"bin"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return "", err
	}
	var name string
	if err := json.Unmarshal(manifest.Bin, &name); err != nil {
		var bins map[string]string
		if err := json.Unmarshal(manifest.Bin, &bins); err != nil {
			return "", fmt.Errorf("TypeScript package bin: %w", err)
		}
		name = bins["tsc"]
	}
	if name == "" {
		return "", errors.New("TypeScript validation package has no bin.tsc CLI")
	}
	return filepath.Abs(filepath.Join(directory, name))
}

func inspect(node, compiler, config string) (configuration, error) {
	stdout, stderr, code, err := command(node, compiler, "--showConfig", "--project", config)
	if err != nil {
		return configuration{}, err
	}
	if code != 0 {
		return configuration{}, fmt.Errorf("%s", strings.TrimSpace(stdout+stderr))
	}
	var result configuration
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return result, fmt.Errorf("TypeScript --showConfig: %w", err)
	}
	if result.Options == nil {
		result.Options = map[string]any{}
	}
	return result, nil
}

func normalizeOptions(options map[string]any, directory string) {
	// The pinned compiler defaults rootDir to the configuration directory.
	// --showConfig emits "./" for an explicit null reset but omits an absent
	// rootDir, although both have the same effective value.
	if options["rootDir"] == nil {
		options["rootDir"] = directory
	}
	for _, key := range []string{"rootDir", "outDir", "declarationDir", "outFile", "tsBuildInfoFile", "baseUrl"} {
		if value, ok := options[key].(string); ok {
			options[key] = optionPath(value, directory)
		}
	}
	for _, key := range []string{"rootDirs", "typeRoots"} {
		if values, ok := options[key].([]any); ok {
			for i, value := range values {
				if path, ok := value.(string); ok {
					if !filepath.IsAbs(path) {
						path = filepath.Join(directory, path)
					}
					values[i] = filepath.Clean(path)
				}
			}
		}
	}
	for _, key := range []string{"declaration", "declarationMap", "sourceMap", "inlineSourceMap", "emitDeclarationOnly", "composite", "incremental", "noEmit", "allowJs", "checkJs", "emitBOM", "noCheck", "resolveJsonModule"} {
		if options[key] != nil {
			continue
		}
		value := false
		switch key {
		case "declaration", "incremental":
			value, _ = options["composite"].(bool)
		case "allowJs":
			value, _ = options["checkJs"].(bool)
		case "resolveJsonModule":
			value = true // The pinned Bazel output profile.
		}
		options[key] = value
	}
}

func compile(node, config, compiler string, sources, directories, outputs []string) ([]string, error) {
	stdout, stderr, code, err := command(node, compiler, "--pretty", "false", "--preserveSymlinks", "--listFiles", "--listEmittedFiles", "--project", config)
	if err != nil {
		return nil, fmt.Errorf("selected TypeScript compiler failed: %w", err)
	}
	var inputs, emitted []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "TSFILE: ") {
			emitted = append(emitted, canonical(strings.TrimPrefix(line, "TSFILE: ")))
		} else if filepath.IsAbs(line) && exists(line) {
			inputs = append(inputs, line)
		} else if line != "" {
			if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
				return nil, err
			}
		}
	}
	if _, err := io.WriteString(os.Stderr, stderr); err != nil {
		return nil, err
	}
	if code != 0 {
		return []string{fmt.Sprintf("selected TypeScript compiler exited with %d", code)}, nil
	}
	declared, all, actual := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, path := range sources {
		declared[canonical(path)] = true
	}
	var implementation []string
	for _, input := range inputs {
		path := canonical(input)
		all[path] = true
		generated := false
		for _, directory := range directories {
			if strings.HasPrefix(path, canonical(directory)+string(filepath.Separator)) {
				generated = true
				break
			}
		}
		if !declaration(input) && !strings.Contains(filepath.ToSlash(input), "/node_modules/") && !generated {
			actual[path] = true
			implementation = append(implementation, input)
		}
	}
	var problems []string
	for _, path := range sources {
		file := canonical(path)
		found := actual[file]
		if declaration(file) {
			found = all[file]
		}
		if !found {
			problems = append(problems, fmt.Sprintf("declared source %s is not a compiler input", file))
		}
	}
	for _, file := range implementation {
		if !declared[canonical(file)] {
			problems = append(problems, fmt.Sprintf("compiler input %s is not a declared emitting source", file))
		}
	}
	wanted := make([]string, 0, len(outputs))
	for _, file := range outputs {
		wanted = append(wanted, canonical(file))
	}
	slices.Sort(wanted)
	slices.Sort(emitted)
	if !slices.Equal(wanted, emitted) {
		a, _ := json.Marshal(emitted)
		b, _ := json.Marshal(wanted)
		problems = append(problems, fmt.Sprintf("selected compiler emitted %s, but the target declares %s", a, b))
	}
	return problems, nil
}

func command(node string, args ...string) (string, string, int, error) {
	var stdout, stderr boundedBuffer
	cmd := exec.Command(node, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return stdout.String(), stderr.String(), exit.ExitCode(), nil
	}
	return stdout.String(), stderr.String(), 0, err
}

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 64*1024*1024 {
		return 0, errors.New("compiler output exceeds 64 MiB")
	}
	return b.Buffer.Write(p)
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }
func canonical(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if real, err := filepath.EvalSymlinks(absolute); err == nil {
		return real
	}
	return absolute
}
func declaration(path string) bool {
	return strings.HasSuffix(path, ".d.ts") || strings.HasSuffix(path, ".d.mts") || strings.HasSuffix(path, ".d.cts")
}
func show(value any, directory string) string {
	if path, ok := value.(string); ok && filepath.IsAbs(path) {
		if relative, err := filepath.Rel(directory, path); err == nil {
			value = relative
		}
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
