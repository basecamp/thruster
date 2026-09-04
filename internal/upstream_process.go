package internal

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

type UpstreamProcess struct {
	Started chan struct{}
	cmd     *exec.Cmd
}

func NewUpstreamProcess(name string, arg ...string) *UpstreamProcess {
	return &UpstreamProcess{
		Started: make(chan struct{}, 1),
		cmd:     exec.Command(name, arg...),
	}
}

func (p *UpstreamProcess) Run() (int, error) {
	p.cmd.Stdin = os.Stdin
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr

	err := p.cmd.Start()
	if err != nil {
		return 0, err
	}

	p.Started <- struct{}{}

	err = p.cmd.Wait()

	return p.handleExitCode(err)
}

func (p *UpstreamProcess) Signal(sig os.Signal) error {
	return p.cmd.Process.Signal(sig)
}

func (p *UpstreamProcess) handleExitCode(err error) (int, error) {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				return 128 + int(status.Signal()), nil
			}
		}
		return exitErr.ExitCode(), nil
	}

	return 0, err
}
