// Command benchmark compares native Bazel actions using the same Node workload.
package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/latticebuild/graceproc"
)

//go:embed all:fixtures
var fixtures embed.FS

type backend struct {
	name, directory, bazel, outputRoot, installBase, results string
	context                                                  context.Context
}

type report struct {
	Schema        int                `json:"schema"`
	Source        string             `json:"sourceCommit"`
	Dirty         bool               `json:"dirty"`
	Platform      string             `json:"platform"`
	Architecture  string             `json:"architecture"`
	Date          time.Time          `json:"date"`
	Versions      map[string]string  `json:"versions"`
	Environment   map[string]string  `json:"environment"`
	FixtureSHA256 map[string]string  `json:"fixtureSha256"`
	Preparation   map[string]int64   `json:"preparationNanoseconds"`
	Inventory     inventoryParity    `json:"inventory"`
	Samples       []pair             `json:"samples"`
	Summaries     map[string]summary `json:"summaries"`
	Qualified     bool               `json:"qualified"`
	Probe         bool               `json:"probe"`
	Failure       string             `json:"failure,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "benchmark:", err)
		os.Exit(1)
	}
}

func run() (err error) {
	source := flag.String("source", "", "clean rules_js checkout to measure")
	work := flag.String("work-root", "", "existing physical development directory for disposable callers and caches")
	results := flag.String("results", "", "new directory for complete raw logs and report")
	bazel := flag.String("bazel", "bazel", "Bazel 9.2.0 executable")
	installBase := flag.String("install-base", "", "existing Bazel embedded tool installation shared by both backends")
	probe := flag.Bool("probe", false, "one pair per case; never qualifies performance or a release")
	flag.Parse()
	if *source == "" || *work == "" || *results == "" {
		return errors.New("--source, --work-root and --results are required")
	}
	*source, err = physicalDirectory(*source)
	if err != nil {
		return err
	}
	*work, err = physicalDirectory(*work)
	if err != nil {
		return err
	}
	if *installBase != "" {
		*installBase, err = externalInstallBase(*source, *installBase)
		if err != nil {
			return err
		}
	}
	*results, err = resultDirectory(*source, *results)
	if err != nil {
		return err
	}
	inside, err := physicalWithin(*source, *work)
	if err != nil {
		return err
	}
	if inside {
		return errors.New("benchmark work and results must be outside the measured checkout")
	}
	if err := os.Mkdir(*results, 0o755); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), graceproc.Signals()...)
	defer stop()
	commit, err := capture(ctx, *source, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	dirty, err := capture(ctx, *source, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if dirty != "" && !*probe {
		return errors.New("qualification requires a clean checkout; --probe cannot qualify a release")
	}
	owned, err := os.MkdirTemp(*work, "rules-js-benchmark-")
	if err != nil {
		return err
	}
	var finalReport *report
	cleanupSafe := true
	defer func() {
		if cleanupSafe {
			err = errors.Join(err, removeOwned(owned))
		}
		if finalReport != nil {
			if err != nil {
				finalReport.Qualified = false
				finalReport.Failure = err.Error()
			}
			err = errors.Join(err, saveJSON(filepath.Join(*results, "report.json"), finalReport))
		}
	}()
	versionContext, cancelVersion := context.WithTimeout(ctx, time.Minute)
	versionLog := filepath.Join(*results, "bazel-version.log")
	versionArgs := []string{"--batch", "--ignore_all_rc_files", "--output_user_root=" + filepath.Join(owned, "bazel-version")}
	if *installBase != "" {
		versionArgs = append(versionArgs, "--install_base="+*installBase)
	}
	err = command(versionContext, *work, versionLog, *bazel, append(versionArgs, "version", "--gnu_format")...)
	cancelVersion()
	if err != nil {
		cleanupSafe = false
		return fmt.Errorf("Bazel version probe failed; retained %s: %w", owned, err)
	}
	versionOutput, err := os.ReadFile(versionLog)
	if err != nil {
		return err
	}
	bazelVersion := string(versionOutput)
	if !strings.Contains(bazelVersion, "bazel 9.2.0") {
		return fmt.Errorf("expected Bazel 9.2.0, got %q", bazelVersion)
	}
	module, err := os.ReadFile(filepath.Join(*source, "MODULE.bazel"))
	if err != nil {
		return err
	}
	version := regexp.MustCompile(`version\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"`).FindSubmatch(module)
	if version == nil {
		return errors.New("cannot determine subject module version")
	}
	r := report{
		Schema: 2, Source: commit, Dirty: dirty != "", Platform: runtime.GOOS,
		Architecture: runtime.GOARCH, Date: time.Now().UTC(), Probe: *probe,
		Versions:      map[string]string{"bazel": "9.2.0", "node": "26.8.2", "pnpm": "12.4.2", "aspect_rules_js": "3.5.0", "rules_nodejs": "6.7.5", "latticebuild_js": string(version[1]), "go": runtime.Version()},
		FixtureSHA256: map[string]string{}, Preparation: map[string]int64{}, Summaries: map[string]summary{},
	}
	finalReport = &r
	defer func() {
		after, checkErr := capture(context.Background(), *source, "git", "status", "--porcelain")
		if checkErr != nil {
			err = errors.Join(err, checkErr)
		} else if after != dirty {
			err = errors.Join(err, errors.New("measured checkout changed during benchmark"))
		}
		head, checkErr := capture(context.Background(), *source, "git", "rev-parse", "HEAD")
		if checkErr != nil {
			err = errors.Join(err, checkErr)
		} else if head != commit {
			err = errors.Join(err, errors.New("measured HEAD changed during benchmark"))
		}
	}()
	r.Environment, err = machineEnvironment(ctx, *work)
	if err != nil {
		return err
	}
	if *installBase != "" {
		r.Environment["bazelInstallBase"] = *installBase
		if runtime.GOOS == "linux" {
			digest, err := hashFile(filepath.Join(*installBase, "linux-sandbox"))
			if err != nil {
				return err
			}
			r.Environment["linuxSandboxSHA256"] = digest
			if err := command(ctx, *work, filepath.Join(*results, "linux-sandbox-probe.log"), filepath.Join(*installBase, "linux-sandbox"), "--", "/bin/true"); err != nil {
				return fmt.Errorf("native Linux sandbox support probe failed: %w", err)
			}
		}
	}
	if runtime.GOOS == "darwin" && !*probe && !strings.HasPrefix(r.Environment["osVersion"], "27.") {
		return errors.New("macOS performance qualification requires macOS 27")
	}
	var backends []*backend
	for _, name := range []string{"lattice", "aspect"} {
		b := &backend{name: name, directory: filepath.Join(owned, name), bazel: *bazel, outputRoot: filepath.Join(owned, "bazel", name), installBase: *installBase, results: *results, context: ctx}
		backends = append(backends, b)
		if err := prepareFixture(b.directory, name, *source, string(version[1]), r.FixtureSHA256); err != nil {
			return err
		}
	}
	// Shutdown must run before deleting the owned output roots, even on failure.
	defer func() {
		for _, b := range backends {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
			shutdownErr := command(cleanup, b.directory, filepath.Join(*results, b.name+"-shutdown.log"), b.bazel, append(b.startupArgs(), "shutdown")...)
			if shutdownErr != nil {
				cleanupSafe = false
				shutdownErr = fmt.Errorf("owned Bazel shutdown failed; retained %s: %w", owned, shutdownErr)
			}
			err = errors.Join(err, shutdownErr)
			cancel()
		}
	}()
	for _, b := range backends {
		start := time.Now()
		if b.name == "lattice" {
			if err := command(ctx, b.directory, filepath.Join(*results, "pnpm-install.log"), "mise", "exec", "pnpm@12.4.2", "--", "pnpm", "install", "--ignore-scripts", "--frozen-lockfile", "--store-dir", filepath.Join(owned, "pnpm-store")); err != nil {
				return err
			}
			if got, err := hashFile(filepath.Join(b.directory, "pnpm-lock.yaml")); err != nil {
				return err
			} else if got != r.FixtureSHA256["pnpm-lock.yaml"] {
				return errors.New("frozen pnpm lock changed")
			}
		}
		r.Preparation[b.name+"-install"] = time.Since(start).Nanoseconds()
		for _, name := range []string{"none", "many", "inventory"} {
			start = time.Now()
			if _, err := b.build("prepare-"+name, name); err != nil {
				return err
			}
			r.Preparation[b.name+"-first-"+name] = time.Since(start).Nanoseconds()
		}
	}
	r.Inventory, err = compareInventories(backends[0], backends[1])
	if err != nil {
		return err
	}
	warmups, trials := 3, 30
	if *probe {
		warmups, trials = 0, 1
	}
	for _, name := range []string{"none", "many"} {
		var retained []pair
		for trial := -warmups; trial < trials; trial++ {
			nonce := fmt.Sprintf("%s-%s-%d", r.Date.Format("20060102T150405.000000000"), name, trial)
			for _, b := range backends {
				if err := writeInput(b.directory, name, nonce); err != nil {
					return err
				}
			}
			p := pair{Case: name, Trial: trial, Nonce: nonce}
			order := []*backend{backends[0], backends[1]}
			if (trial+warmups)%2 == 1 {
				order[0], order[1] = order[1], order[0]
			}
			p.First = order[0].name
			for _, b := range order {
				fmt.Printf("%s pair %d/%d: %s\n", name, trial+1, trials, b.name)
				m, err := b.measure(fmt.Sprintf("%s-%03d", name, trial+warmups), name, nonce)
				if err != nil {
					return err
				}
				if b.name == "lattice" {
					p.Lattice = m
				} else {
					p.Aspect = m
				}
			}
			if p.Lattice.OutputSHA256 != p.Aspect.OutputSHA256 {
				return errors.New("paired output mismatch")
			}
			r.Samples = append(r.Samples, p)
			if trial >= 0 {
				retained = append(retained, p)
			}
		}
		r.Summaries[name] = summarize(retained)
		for _, b := range backends {
			start := time.Now()
			if err := b.noop(name); err != nil {
				return err
			}
			r.Preparation[b.name+"-noop-"+name] = time.Since(start).Nanoseconds()
		}
	}
	// Qualification certifies complete, comparable measurements. Speed is an
	// observation; a slower or inconclusive result remains in the report.
	r.Qualified = !*probe && r.Summaries["none"].Pairs == 30 && r.Summaries["many"].Pairs == 30
	return nil
}

func prepareFixture(directory, backend, source, version string, hashes map[string]string) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"action.mjs", "package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml", "action.bzl", backend + ".MODULE.bazel.template", backend + ".BUILD.bazel.template"} {
		data, err := fixtures.ReadFile("fixtures/" + name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(sum[:])
		target := name
		if strings.HasSuffix(name, ".template") {
			target = strings.TrimSuffix(strings.TrimPrefix(name, backend+"."), ".template")
			// JSON strings and Starlark strings share the required path escapes.
			quoted, _ := json.Marshal(filepath.ToSlash(source))
			data = []byte(strings.ReplaceAll(strings.ReplaceAll(string(data), `"{{SOURCE}}"`, string(quoted)), "{{VERSION}}", version))
		}
		if err := os.WriteFile(filepath.Join(directory, target), data, 0o644); err != nil {
			return err
		}
	}
	for _, name := range []string{"none", "many"} {
		if err := writeInput(directory, name, "prepare"); err != nil {
			return err
		}
	}
	return nil
}

func writeInput(directory, name, nonce string) error {
	dependencies := []string{}
	if name == "many" {
		dependencies = []string{"knip", "prettier", "vite", "vitest"}
	}
	return saveJSON(filepath.Join(directory, "input_"+name+".json"), struct {
		Nonce        string   `json:"nonce"`
		Dependencies []string `json:"dependencies"`
	}{nonce, dependencies})
}

func physicalDirectory(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("not an existing directory: %s", path)
	}
	return path, nil
}

func externalInstallBase(source, requested string) (string, error) {
	installation, err := physicalDirectory(requested)
	if err != nil {
		return "", err
	}
	inside, err := physicalWithin(source, installation)
	if err != nil {
		return "", err
	}
	if inside {
		return "", errors.New("Bazel install base must be physically outside the measured checkout")
	}
	return installation, nil
}

func resultDirectory(source, requested string) (string, error) {
	requested, err := filepath.Abs(requested)
	if err != nil {
		return "", err
	}
	parent, err := physicalDirectory(filepath.Dir(requested))
	if err != nil {
		return "", err
	}
	result := filepath.Join(parent, filepath.Base(requested))
	inside, err := physicalWithin(source, result)
	if err != nil {
		return "", err
	}
	if inside {
		return "", errors.New("results must be physically outside the measured checkout")
	}
	return result, nil
}

// File identity also handles case aliases on macOS and Windows volumes.
func physicalWithin(root, candidate string) (bool, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	for current := candidate; ; current = filepath.Dir(current) {
		info, err := os.Stat(current)
		if err == nil && os.SameFile(rootInfo, info) {
			return true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		if current == filepath.Dir(current) {
			return false, nil
		}
	}
}

func capture(ctx context.Context, directory, program string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = directory
	data, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", program, err, data)
	}
	return strings.TrimSpace(string(data)), nil
}

func machineEnvironment(ctx context.Context, directory string) (map[string]string, error) {
	environment := map[string]string{"logicalCPUs": fmt.Sprint(runtime.NumCPU()), "runnerOS": os.Getenv("RUNNER_OS"), "runnerArch": os.Getenv("RUNNER_ARCH")}
	var err error
	switch runtime.GOOS {
	case "darwin":
		environment["osVersion"], err = capture(ctx, directory, "sw_vers", "-productVersion")
		if err != nil {
			return nil, err
		}
		environment["cpu"], err = capture(ctx, directory, "sysctl", "-n", "machdep.cpu.brand_string")
	case "linux":
		environment["osVersion"], err = capture(ctx, directory, "uname", "-sr")
		if err != nil {
			return nil, err
		}
		environment["cpu"], err = capture(ctx, directory, "uname", "-m")
	case "windows":
		environment["osVersion"], err = capture(ctx, directory, "pwsh", "-NoProfile", "-Command", "[Environment]::OSVersion.VersionString")
		if err != nil {
			return nil, err
		}
		environment["cpu"], err = capture(ctx, directory, "pwsh", "-NoProfile", "-Command", "(Get-CimInstance Win32_Processor).Name")
	default:
		return nil, errors.New("supported benchmark platforms are Linux, macOS and Windows")
	}
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		environment["storage"], err = capture(ctx, directory, "df", "-P", directory)
	}
	return environment, err
}

func command(ctx context.Context, directory, log, program string, args ...string) (err error) {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	file, err := os.OpenFile(log, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			signals <- os.Interrupt
		case <-done:
		}
	}()
	cmd := exec.Command(program, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = directory, file, file
	code, err := graceproc.Run(cmd, signals)
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), err)
	}
	if err != nil {
		return fmt.Errorf("%s exited %d (see %s): %w", program, code, log, err)
	}
	if code != 0 {
		return fmt.Errorf("%s exited %d (see %s)", program, code, log)
	}
	return nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Bazel marks some generated directories read-only. Only directories in our
// newly owned tree are made removable; WalkDir never follows symlink entries.
func removeOwned(root string) error {
	if err := filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.Chmod(name, info.Mode().Perm()|0o700)
		}
		return nil
	}); err != nil {
		return err
	}
	return os.RemoveAll(root)
}
