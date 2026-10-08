package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type spawn struct {
	Target   string `json:"targetLabel"`
	Mnemonic string `json:"mnemonic"`
	Runner   string `json:"runner"`
	Cached   bool   `json:"cacheHit"`
	ExitCode int    `json:"exitCode"`
	Status   string `json:"status"`
	Inputs   []struct {
		Path   string `json:"path"`
		IsTool bool   `json:"isTool"`
	} `json:"inputs"`
	Outputs []string `json:"listedOutputs"`
	Metrics struct {
		Total     string `json:"totalTime"`
		Execution string `json:"executionWallTime"`
	} `json:"metrics"`
}

type measurement struct {
	Total          int64  `json:"totalNanoseconds"`
	Execution      int64  `json:"executionNanoseconds"`
	Command        int64  `json:"commandNanoseconds"`
	Prerequisites  int64  `json:"prerequisiteNanoseconds"`
	ExpandedInputs int    `json:"expandedInputs"`
	ToolInputs     int    `json:"toolInputs"`
	OutputSHA256   string `json:"outputSha256"`
	Log            string `json:"log"`
	Runner         string `json:"runner"`
}

type pair struct {
	Case    string      `json:"case"`
	Trial   int         `json:"trial"`
	Nonce   string      `json:"nonce"`
	First   string      `json:"first"`
	Lattice measurement `json:"lattice"`
	Aspect  measurement `json:"aspect"`
}

func nativeRunner() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin-sandbox"
	case "linux":
		return "linux-sandbox"
	default:
		return "local"
	}
}

func (b *backend) build(id, target string) ([]spawn, error) {
	log := filepath.Join(b.results, b.name+"-"+id+".spawns.json")
	strategy := nativeRunner()
	args := append(b.startupArgs(), []string{
		"build", "//:" + target,
		"--enable_runfiles", "--jobs=1", "--spawn_strategy=" + strategy,
		"--strategy=BenchmarkNode=" + strategy, "--strategy=BenchmarkInventory=" + strategy,
		"--disk_cache=", "--remote_cache=", "--remote_executor=",
		"--noremote_accept_cached", "--noremote_upload_local_results", "--execution_log_json_file=" + log,
		"--color=no", "--curses=no",
	}...)
	if err := command(b.context, b.directory, filepath.Join(b.results, b.name+"-"+id+".command.log"), b.bazel, args...); err != nil {
		return nil, err
	}
	file, err := os.Open(log)
	if err != nil {
		return nil, err
	}
	rows, readErr := readSpawns(file)
	return rows, errors.Join(readErr, file.Close())
}

func (b *backend) startupArgs() []string {
	args := []string{"--ignore_all_rc_files", "--output_user_root=" + b.outputRoot}
	if b.installBase != "" {
		args = append(args, "--install_base="+b.installBase)
	}
	return args
}

// Bazel's JSON execution log is a stream of complete SpawnExec objects.
func readSpawns(reader io.Reader) ([]spawn, error) {
	decoder := json.NewDecoder(reader)
	var rows []spawn
	for {
		var row spawn
		err := decoder.Decode(&row)
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		if len(rows) >= 100_000 {
			return nil, errors.New("execution log exceeds spawn limit")
		}
		rows = append(rows, row)
	}
}

func actionMeasurement(rows []spawn, target, runner string) (measurement, error) {
	var result measurement
	count := 0
	for _, row := range rows {
		if row.Mnemonic != "BenchmarkNode" || row.Target != "//:"+target {
			if !row.Cached {
				elapsed, err := time.ParseDuration(row.Metrics.Total)
				if err != nil || elapsed < 0 || row.ExitCode != 0 || row.Status != "" {
					return result, errors.New("executed prerequisite lacks valid successful timing evidence")
				}
				result.Prerequisites += elapsed.Nanoseconds()
			}
			continue
		}
		count++
		if row.Cached || row.ExitCode != 0 || row.Status != "" || row.Runner != runner {
			return result, fmt.Errorf("invalid main spawn: cached=%t exit=%d status=%q runner=%q", row.Cached, row.ExitCode, row.Status, row.Runner)
		}
		found := false
		for _, output := range row.Outputs {
			found = found || strings.HasSuffix(filepath.ToSlash(output), "/"+target+"/result.json")
		}
		if !found {
			return result, errors.New("main spawn does not declare the expected output")
		}
		total, err := time.ParseDuration(row.Metrics.Total)
		if err != nil || total <= 0 {
			return result, errors.New("main spawn lacks positive totalTime")
		}
		execution, err := time.ParseDuration(row.Metrics.Execution)
		if err != nil || execution <= 0 {
			return result, errors.New("main spawn lacks positive executionWallTime")
		}
		result.Total, result.Execution, result.Runner = total.Nanoseconds(), execution.Nanoseconds(), row.Runner
		result.ExpandedInputs = len(row.Inputs)
		for _, input := range row.Inputs {
			if input.IsTool {
				result.ToolInputs++
			}
		}
	}
	if count != 1 {
		return result, fmt.Errorf("expected exactly one uncached main spawn, got %d", count)
	}
	return result, nil
}

func (b *backend) measure(id, name, nonce string) (measurement, error) {
	start := time.Now()
	rows, err := b.build(id, name)
	elapsed := time.Since(start)
	if err != nil {
		return measurement{}, err
	}
	m, err := actionMeasurement(rows, name, nativeRunner())
	if err != nil {
		return m, err
	}
	m.Command = elapsed.Nanoseconds()
	m.Log = b.name + "-" + id + ".spawns.json"
	output := filepath.Join(b.directory, "bazel-bin", name, "result.json")
	data, err := os.ReadFile(output)
	if err != nil {
		return m, err
	}
	var value struct {
		Nonce    string     `json:"nonce"`
		Node     string     `json:"node"`
		Packages [][]string `json:"packages"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return m, err
	}
	if value.Nonce != nonce || value.Node != "v26.8.2" || (name == "none" && len(value.Packages) != 0) || (name == "many" && len(value.Packages) < 40) {
		return m, errors.New("output nonce, Node version or application dependency count is incorrect")
	}
	m.OutputSHA256, err = hashFile(output)
	if err != nil {
		return m, err
	}
	if err := os.WriteFile(filepath.Join(b.results, b.name+"-"+id+".output.json"), data, 0o644); err != nil {
		return m, err
	}
	return m, nil
}

func (b *backend) noop(name string) error {
	rows, err := b.build("noop-"+name, name)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Mnemonic == "BenchmarkNode" && !row.Cached {
			return errors.New("unchanged input reran the main action")
		}
	}
	return nil
}
