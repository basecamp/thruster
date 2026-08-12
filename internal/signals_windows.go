//go:build windows

package internal

import (
	"os"
)

// Windows does not have SIGTERM natively, and os.Interrupt is unreliable via os.Process.Signal.
var terminationSignals = []os.Signal{os.Interrupt}
var defaultTerminationSignal = os.Kill

func exitCodeFromSignal(sig os.Signal) int {
	return 1
}

func exitCodeFromProcessState(state *os.ProcessState) int {
	return state.ExitCode()
}

func forwardSignal(p *os.Process, sig os.Signal) error {
	if sig == os.Interrupt || sig == os.Kill {
		// os.Interrupt and os.Kill are not reliably supported via os.Process.Signal on Windows unless
		// CREATE_NEW_PROCESS_GROUP is used with specific Windows console APIs.
		// Escalating to Kill ensures the child process reliably terminates.
		return p.Kill()
	}
	return p.Signal(sig)
}
