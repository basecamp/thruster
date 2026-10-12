package internal

import (
	"errors"
	"os"
	"os/exec"
	"sync"
)

var ErrProcessNotRunning = errors.New("process not running")

type UpstreamProcess struct {
	started   chan struct{}
	cmd       *exec.Cmd
	startOnce sync.Once
}

func NewUpstreamProcess(name string, arg ...string) *UpstreamProcess {
	return &UpstreamProcess{
		started: make(chan struct{}),
		cmd:     exec.Command(name, arg...),
	}
}

func (p *UpstreamProcess) Started() <-chan struct{} {
	return p.started
}

func (p *UpstreamProcess) Run() (int, error) {
	p.cmd.Stdin = os.Stdin
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr

	err := p.cmd.Start()

	// Broadcast that the start attempt has concluded (unblocks waiters on success OR failure)
	p.startOnce.Do(func() {
		close(p.started)
	})

	if err != nil {
		return 0, err
	}

	err = p.cmd.Wait()

	return p.handleExitCode(err)
}

func (p *UpstreamProcess) Signal(sig os.Signal) error {
	if p.cmd == nil || p.cmd.Process == nil {
		return ErrProcessNotRunning
	}
	return forwardSignal(p.cmd.Process, sig)
}

func (p *UpstreamProcess) handleExitCode(err error) (int, error) {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitCodeFromProcessState(exitErr.ProcessState), nil
	}

	return 0, err
}
