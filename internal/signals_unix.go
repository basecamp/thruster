//go:build !windows

package internal

import (
	"os"
	"syscall"
)

var terminationSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
var defaultTerminationSignal os.Signal = syscall.SIGTERM

func exitCodeFromSignal(sig os.Signal) int {
	if sysSig, ok := sig.(syscall.Signal); ok {
		return 128 + int(sysSig)
	}
	return 1
}

func exitCodeFromProcessState(state *os.ProcessState) int {
	if status, ok := state.Sys().(syscall.WaitStatus); ok {
		if status.Signaled() {
			return 128 + int(status.Signal())
		}
	}
	return state.ExitCode()
}

func forwardSignal(p *os.Process, sig os.Signal) error {
	return p.Signal(sig)
}
