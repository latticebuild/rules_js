package process

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/latticebuild/graceproc"
)

type Result struct {
	Code   int
	Signal os.Signal
	Err    error
}

// Main returns the child's status only after command-specific reports and cleanup.
func Main(run func([]string) Result) {
	result := run(os.Args[1:])
	if result.Err != nil {
		fmt.Fprintln(os.Stderr, result.Err)
		if result.Code == 0 {
			result.Code = 1
		}
	}
	exit(result.Code, result.Signal)
}

func Supervise(cmd *exec.Cmd) Result {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, graceproc.Signals()...)
	defer signal.Stop(signals)
	forwarded := make(chan os.Signal, 2)
	finished := make(chan Result, 1)
	go func() { code, err := graceproc.Run(cmd, forwarded); finished <- Result{Code: code, Err: err} }()
	var interrupted os.Signal
	for {
		select {
		case sig := <-signals:
			if interrupted == nil {
				interrupted = sig
			}
			select {
			case forwarded <- sig:
			default:
			}
		case result := <-finished:
			signal.Stop(signals)
			select {
			case sig := <-signals:
				if interrupted == nil {
					interrupted = sig
				}
			default:
			}
			if interrupted == nil {
				interrupted = processSignal(cmd.ProcessState)
			}
			result.Signal = interrupted
			if interrupted != nil {
				result.Code = graceproc.SignalCode(interrupted)
			}
			return result
		}
	}
}

func Command(node string, args []string, changes map[string]string, unset ...string) *exec.Cmd {
	cmd := exec.Command(node, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = environment(os.Environ(), changes, unset...)
	return cmd
}

func environment(base []string, changes map[string]string, unset ...string) []string {
	var result []string
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		replace := false
		for name := range changes {
			if strings.EqualFold(key, name) {
				replace = true
			}
		}
		for _, name := range unset {
			if strings.EqualFold(key, name) {
				replace = true
			}
		}
		if !replace {
			result = append(result, entry)
		}
	}
	for key, value := range changes {
		result = append(result, key+"="+value)
	}
	return result
}

func Failed(err error) Result { return Result{Code: 1, Err: err} }
