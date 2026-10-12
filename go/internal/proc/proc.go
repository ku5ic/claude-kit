// Package proc runs subprocesses under a deadline, so a hung tool can't
// outlast the hook or command that started it. It imports nothing from the
// kit.
package proc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// Quick bounds a probe: a version, a path, a config lookup.
const Quick = 10 * time.Second

// Cmd is an exec.Cmd killed once its deadline passes.
type Cmd struct {
	*exec.Cmd
	ctx           context.Context
	cancel        context.CancelFunc
	interruptible bool
}

// Command is name with args, killed once timeout passes.
func Command(timeout time.Duration, name string, args ...string) *Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, name, args...)
	// A killed process's orphans could hold its output pipes open.
	cmd.WaitDelay = 2 * time.Second
	return &Cmd{Cmd: cmd, ctx: ctx, cancel: cancel}
}

// KillGroup runs c in its own process group, so the deadline kills the
// workers a runner spawned too. A group misses the terminal's Ctrl-C: use
// it only where the deadline is the one way the command stops, else
// Interruptible.
func (c *Cmd) KillGroup() {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
}

// Interruptible is KillGroup for a command a person may stop: while c
// runs, Ctrl-C and SIGTERM go to its group, then end this process too, as
// they would have with no group of its own.
func (c *Cmd) Interruptible() {
	c.KillGroup()
	c.interruptible = true
}

// TimedOut reports whether the deadline killed c.
func (c *Cmd) TimedOut() bool { return errors.Is(c.ctx.Err(), context.DeadlineExceeded) }

// Run is exec.Cmd's Run, releasing the deadline's timer after.
func (c *Cmd) Run() error {
	defer c.cancel()
	if !c.interruptible {
		return c.Cmd.Run()
	}
	sigs := make(chan os.Signal, 1)
	for _, s := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		// Notify would end an ignore this process inherited (an & job's
		// SIGINT), and it must still ignore it.
		if !signal.Ignored(s) {
			signal.Notify(sigs, s)
		}
	}
	defer signal.Stop(sigs)
	if err := c.Start(); err != nil {
		return err
	}
	waited := make(chan error, 1)
	go func() { waited <- c.Wait() }()
	select {
	case err := <-waited:
		return err
	case sig := <-sigs:
		s := sig.(syscall.Signal)
		_ = syscall.Kill(-c.Process.Pid, s)
		<-waited
		signal.Reset(s)
		_ = syscall.Kill(os.Getpid(), s)
		// The kill lands asynchronously; returning would let the caller
		// run on, or exit 0, first.
		time.Sleep(time.Second)
		os.Exit(128 + int(s))
		return nil
	}
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
