package internal

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
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
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr

	err := p.cmd.Start()
	if err != nil {
		return 0, err
	}

	p.Started <- struct{}{}

	go p.handleSignals()
	err = p.cmd.Wait()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}

	return 0, err
}

func (p *UpstreamProcess) Signal(sig os.Signal) error {
	return p.cmd.Process.Signal(sig)
}

func (p *UpstreamProcess) handleSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	sig := <-ch
	slog.Info("Relaying signal to upstream process", "signal", sig.String())
	p.Signal(sig)
}
