package process

import "os"

func processSignal(_ *os.ProcessState) os.Signal { return nil }
func exit(code int, _ os.Signal)                 { os.Exit(code) }
