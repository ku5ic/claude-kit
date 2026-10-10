// Package proc runs subprocesses under a deadline, so a hung tool can't
// outlast the hook or command that started it. It imports nothing from the
// kit.
package proc

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// Quick bounds a probe: a version, a path, a config lookup.
const Quick = 10 * time.Second

// Cmd is an exec.Cmd killed once its deadline passes.
type Cmd struct {
	*exec.Cmd
	ctx    context.Context
	cancel context.CancelFunc
}

// Command is name with args, killed once timeout passes.
func Command(timeout time.Duration, name string, args ...string) *Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, name, args...)
	// A killed process's orphans could hold its output pipes open.
	cmd.WaitDelay = 2 * time.Second
	return &Cmd{cmd, ctx, cancel}
}

// KillGroup runs c in its own process group, so the deadline kills the
// workers a runner spawned too. A group misses the terminal's Ctrl-C: use
// it only where the deadline is the one way the command stops.
func (c *Cmd) KillGroup() {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
}

// TimedOut reports whether the deadline killed c.
func (c *Cmd) TimedOut() bool { return errors.Is(c.ctx.Err(), context.DeadlineExceeded) }

// Run is exec.Cmd's Run, releasing the deadline's timer after.
func (c *Cmd) Run() error {
	defer c.cancel()
	return c.Cmd.Run()
}

// Output is exec.Cmd's Output, releasing the deadline's timer after.
func (c *Cmd) Output() ([]byte, error) {
	defer c.cancel()
	return c.Cmd.Output()
}

// CombinedOutput is exec.Cmd's CombinedOutput, releasing the deadline's
// timer after.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	defer c.cancel()
	return c.Cmd.CombinedOutput()
}
