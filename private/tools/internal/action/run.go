package action

import (
	"errors"
	"fmt"
	"github.com/latticebuild/graceproc"
	"github.com/latticebuild/rules_js/go/filetree"
	"os"
	"os/exec"
	"os/signal"

	// Run owns one deterministic scratch directory. It never writes a command's
	// outputs into input storage and removes scratch after success or failure.
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

func Run(execroot string, m Manifest) (int, error) {
	execroot, err := filepath.Abs(execroot)
	if err != nil {
		return 1, err
	}
	execroot, err = filetree.RealPath(execroot)
	if err != nil {
		return 1, err
	}
	scratch, err := filetree.Path(execroot, m.Root)
	if err != nil {
		return 1, err
	}
	if m.StatusFile != "" {
		if m.Env, err = stampEnv(filepath.Join(execroot, m.StatusFile), m.Env); err != nil {
			return 1, err
		}
	}
	return runAt(execroot, scratch, m)
}

// runAt stages m into scratch, runs its command, and publishes declared
// outputs.
func runAt(execroot, scratch string, m Manifest) (code int, err error) {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, graceproc.Signals()...)
	defer signal.Stop(signals)
	if err := validateRun(execroot, scratch, m); err != nil {
		return 1, err
	}
	limit, protected, err := inputCopier(execroot, scratch, m)
	if err != nil {
		return 1, err
	}
	for _, path := range []string{m.Executable, m.StatusFile} {
		if path == "" {
			continue
		}
		input, err := filetree.Path(execroot, path)
		if err != nil {
			return 1, err
		}
		physical, err := filetree.RealPath(input)
		if err != nil {
			return 1, err
		}
		protected = append(protected, physical)
	}
	node := os.Getenv("NODE")
	if node == "" && m.Executable == "" {
		node, err = exec.LookPath("node")
		if err != nil {
			return 1, err
		}
	}
	if node != "" {
		if !filepath.IsAbs(node) {
			node = filepath.Join(execroot, node)
		}
		physical, err := filetree.RealPath(node)
		if err != nil {
			return 1, err
		}
		protected = append(protected, physical)
	}
	if err := limit.CheckDestination(execroot, scratch, protected...); err != nil {
		return 1, err
	}
	for _, pair := range m.Outputs {
		target, _ := filetree.Path(execroot, pair[1])
		if err := limit.CheckDestination(execroot, target, protected...); err != nil {
			return 1, err
		}
	}
	if err := acquireScratch(execroot, scratch); err != nil {
		return 1, err
	}
	defer func() {
		if cleanup := os.RemoveAll(scratch); cleanup != nil {
			err = errors.Join(err, fmt.Errorf("remove scratch: %w", cleanup))
			code = 1
		}
	}()
	if err := stageAt(execroot, scratch, m, limit); err != nil {
		return 1, err
	}
	node, cwd, env, err := commandEnvironment(execroot, scratch, m)
	if err != nil {
		return 1, err
	}
	program := node
	if m.Executable != "" {
		program, err = filetree.Path(execroot, m.Executable)
		if err != nil {
			return 1, err
		}
	}
	if code, err := runCommand(scratch, program, cwd, env, m, signals); err != nil || code != 0 {
		return code, err
	}
	if err := publishOutputs(execroot, scratch, m); err != nil {
		return 1, err
	}
	return 0, nil
}

// acquireScratch creates an action's deterministic scratch directory
// exclusively, so accidental reuse by a concurrent build fails.
func acquireScratch(execroot, scratch string) error {
	if err := filetree.CheckParents(execroot, scratch); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(scratch), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(scratch, 0o755); err != nil {
		return fmt.Errorf("acquire scratch directory: %w", err)
	}
	return nil
}

func commandEnvironment(execroot, root string, m Manifest) (string, string, []string, error) {
	var err error
	cwd := root
	if m.Cwd != "" && m.Cwd != "." {
		cwd, err = filetree.Path(root, m.Cwd)
		if err != nil {
			return "", "", nil, err
		}
	}
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		return "", "", nil, err
	}
	node := os.Getenv("NODE")
	if node == "" {
		node = "node"
	} else if !filepath.IsAbs(node) {
		node = filepath.Join(execroot, filepath.FromSlash(node))
	}
	// Native file walkers stop ancestor ignore discovery at .git. The empty
	// boundary keeps execroot's /bazel-* ignores outside this staged checkout.
	for _, dir := range []string{".tmp", ".home", ".git"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return "", "", nil, err
		}
	}
	defaults := map[string]string{
		"PATH":   filepath.Dir(node),
		"TMPDIR": filepath.Join(root, ".tmp"), "TMP": filepath.Join(root, ".tmp"), "TEMP": filepath.Join(root, ".tmp"),
		"HOME": filepath.Join(root, ".home"), "USERPROFILE": filepath.Join(root, ".home"),
	}
	env := mergeEnv(mergeEnv(actionEnv(os.Environ()), defaults), m.Env)
	// A native adapter must use the declared interpreter, even when the
	// action supplies an environment variable with another spelling on Windows.
	env = mergeEnv(env, map[string]string{"NODE": node})
	return node, cwd, env, nil
}

func validateRun(execroot, scratch string, m Manifest) error {
	if err := validateLayout(scratch, m); err != nil {
		return err
	}
	if m.Executable != "" {
		program, err := filetree.Path(execroot, m.Executable)
		if err != nil {
			return err
		}
		if filetree.Within(scratch, program) || filetree.Within(program, scratch) {
			return errors.New("native executable overlaps scratch")
		}
		for _, output := range m.Outputs {
			target, err := filetree.Path(execroot, output[1])
			if err != nil {
				return err
			}
			if filetree.Within(target, program) || filetree.Within(program, target) {
				return errors.New("native executable overlaps output")
			}
		}
	}
	if m.Cwd != "" && m.Cwd != "." {
		if _, err := filetree.Path(scratch, m.Cwd); err != nil {
			return err
		}
	}
	for _, pair := range m.Files {
		source, err := filetree.Path(execroot, pair[1])
		if err != nil {
			return err
		}
		if filetree.Within(scratch, source) || filetree.Within(source, scratch) {
			return fmt.Errorf("scratch overlaps input %s", pair[1])
		}
	}
	if len(m.Scripts) == 0 {
		return errors.New("manifest names no script")
	}
	for _, script := range m.Scripts {
		if _, err := filetree.Path(scratch, script); err != nil {
			return err
		}
	}
	outputPaths := map[string]string{}
	for _, pair := range m.Outputs {
		source, err := filetree.Path(scratch, pair[0])
		if err != nil {
			return err
		}
		target, err := filetree.Path(execroot, pair[1])
		if err != nil {
			return err
		}
		if filetree.Within(scratch, target) || filetree.Within(target, scratch) {
			return fmt.Errorf("output overlaps scratch: %s", pair[1])
		}
		for previous := range outputPaths {
			if filetree.Within(previous, target) || filetree.Within(target, previous) {
				return fmt.Errorf("overlapping outputs: %s and %s", previous, target)
			}
		}
		outputPaths[target] = source
		for _, input := range m.Files {
			staged, _ := filetree.Path(scratch, input[0])
			origin, _ := filetree.Path(execroot, input[1])
			if filetree.Within(source, staged) || filetree.Within(staged, source) || filetree.Within(target, origin) || filetree.Within(origin, target) {
				return fmt.Errorf("output %s overlaps input %s", pair[0], input[0])
			}
		}
		for _, link := range m.Links {
			staged, _ := filetree.Path(scratch, link[0])
			if filetree.Within(staged, source) || filetree.Within(source, staged) {
				return fmt.Errorf("output %s overlaps package link %s", pair[0], link[0])
			}
		}
	}
	for key, value := range m.Env {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("invalid environment variable %q", key)
		}
	}
	return nil
}

func runCommand(root, node, cwd string, env []string, m Manifest, signals <-chan os.Signal) (int, error) {
	// A signal during staging, or while the command exits, cancels the action.
	select {
	case sig := <-signals:
		return graceproc.SignalCode(sig), nil
	default:
	}
	command := make([]string, 0, len(m.Scripts)+len(m.Args))
	for _, script := range m.Scripts {
		path, err := filetree.Path(root, script)
		if err != nil {
			return 1, err
		}
		command = append(command, path)
	}
	cmd := exec.Command(node, append(command, m.Args...)...)
	cmd.Dir, cmd.Env = cwd, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if code, err := graceproc.Run(cmd, signals); err != nil || code != 0 {
		return code, err
	}
	select {
	case sig := <-signals:
		return graceproc.SignalCode(sig), nil
	default:
	}
	return 0, nil
}

func publishOutputs(execroot, root string, m Manifest) error {
	limit := filetree.NewCopier(nil)
	for _, pair := range m.Outputs {
		source, _ := filetree.Path(root, pair[0])
		if err := filetree.CheckParents(root, source); err != nil {
			return err
		}
		info, err := os.Lstat(source)
		if err != nil {
			return fmt.Errorf("declared output %s: %w", pair[0], err)
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("declared output %s is a symlink or reparse point", pair[0])
		}
		physical, err := filetree.RealPath(source)
		if err != nil {
			return fmt.Errorf("declared output %s: %w", pair[0], err)
		}
		limit.DeclareRoot(physical)
	}
	for _, pair := range m.Outputs {
		source, _ := filetree.Path(root, pair[0])
		target, _ := filetree.Path(execroot, pair[1])
		if err := filetree.CheckParents(execroot, target); err != nil {
			return err
		}
		if err := filetree.Prepare(execroot, target); err != nil {
			return err
		}
		if err := limit.Copy(source, target); err != nil {
			return err
		}
	}
	return nil
}

// The action receives only declared overrides and the OS facilities Node needs.
// NODE_OPTIONS/NODE_PATH never leak from the user's shell into a build.
func actionEnv(base []string) []string {
	var allowed []string
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "PATH", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "TMP", "TEMP", "TMPDIR", "LD_LIBRARY_PATH":
			allowed = append(allowed, entry)
		}
	}
	return allowed
}

func mergeEnv(base []string, overlay map[string]string) []string {
	keys := map[string]string{}
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		keys[envName(key)] = entry
	}
	var overlayKeys []string
	for key := range overlay {
		overlayKeys = append(overlayKeys, key)
	}
	slices.Sort(overlayKeys)
	for _, key := range overlayKeys {
		keys[envName(key)] = key + "=" + overlay[key]
	}
	var names []string
	for key := range keys {
		names = append(names, key)
	}
	slices.Sort(names)
	out := make([]string, 0, len(names))
	for _, key := range names {
		out = append(out, keys[key])
	}
	return out
}

// envName is the identity of an environment variable name. Windows compares
// names case-insensitively.
func envName(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}
