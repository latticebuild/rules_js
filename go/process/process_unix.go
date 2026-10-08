//go:build !windows

package process

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

func processSignal(state *os.ProcessState) os.Signal {
	if state != nil {
		if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return status.Signal()
		}
	}
	return nil
}
func exit(code int, sig os.Signal) {
	if sig != nil {
		signal.Reset(sig)
		if err := syscall.Kill(os.Getpid(), sig.(syscall.Signal)); err == nil {
			for {
				time.Sleep(time.Second)
			}
		}
	}
	os.Exit(code)
}
