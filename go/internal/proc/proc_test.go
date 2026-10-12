package proc

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// runner is this test binary run again as a runner of an interruptible
// command, whose worker, a foreground grandchild, sleeps; with ignoreInt, it
// starts with SIGINT ignored, as an & job does. It returns the runner and
// the worker's pid.
func runner(t *testing.T, ignoreInt bool) (*exec.Cmd, int) {
	t.Helper()
	pidFile := filepath.Join(t.TempDir(), "pid")
	args := []string{os.Args[0], "-test.run=^TestInterruptibleRunner$"}
	if ignoreInt {
		args = append([]string{"sh", "-c", `trap "" INT; exec "$@"`, "sh"}, args...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "PROC_TEST_INTERRUPT="+pidFile)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		data, _ := os.ReadFile(pidFile)
		if worker, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			t.Cleanup(func() { _ = syscall.Kill(worker, syscall.SIGKILL) })
			return cmd, worker
		}
	}
	t.Fatal("the runner never started its worker")
	return nil, 0
}

// TestInterruptibleRunner is the runner process; as a plain test it's a no-op.
func TestInterruptibleRunner(t *testing.T) {
	pidFile := os.Getenv("PROC_TEST_INTERRUPT")
	if pidFile == "" {
		return
	}
	cmd := Command(time.Minute, "sh", "-c", "sh -c 'echo $$ > "+pidFile+"; exec sleep 30'; true")
	cmd.Interruptible()
	_ = cmd.Run()
	os.Exit(0) // only if a signal spared the runner
}

// diesOf wants the runner dead of sig and its worker gone.
func diesOf(t *testing.T, runner *exec.Cmd, worker int, sig syscall.Signal) {
	t.Helper()
	var exit *exec.ExitError
	if err := runner.Wait(); !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != sig {
		t.Errorf("the runner ended with %v, want death by %s", err, sig)
	}
	for range 50 {
		if syscall.Kill(worker, 0) != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("the worker %d outlived %s", worker, sig)
}

// A person's Ctrl-C reaches an interruptible command's whole group, then
// the process that ran it, as it would with no group of its own.
func TestCtrlCReachesAnInterruptibleGroupThenItsRunner(t *testing.T) {
	t.Parallel()
	cmd, worker := runner(t, false)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	diesOf(t, cmd, worker, syscall.SIGINT)
}

// A runner started with SIGINT ignored keeps ignoring it; SIGTERM still
// reaches the group.
func TestAnIgnoredCtrlCStaysIgnored(t *testing.T) {
	t.Parallel()
	cmd, worker := runner(t, true)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if syscall.Kill(worker, 0) != nil {
		t.Fatal("an ignored Ctrl-C killed the worker")
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	diesOf(t, cmd, worker, syscall.SIGTERM)
}

func TestDeadlineKillsTheCommand(t *testing.T) {
	t.Parallel()
	start := time.Now()
	cmd := Command(100*time.Millisecond, "sleep", "5")
	if err := cmd.Run(); err == nil {
		t.Fatal("a killed sleep exited cleanly")
	}
	if !cmd.TimedOut() {
		t.Error("TimedOut is false after the deadline killed it")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("took %s, want the deadline to stop it", elapsed)
	}
}

func TestKillGroupKillsSpawnedWorkers(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), "survived")
	// The worker outlives its parent unless the whole group dies.
	cmd := Command(200*time.Millisecond, "sh", "-c", "(sleep 1; touch "+marker+") & wait")
	cmd.KillGroup()
	_ = cmd.Run()
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("a worker survived the group kill")
	}
}

func TestOutputWithinTheDeadline(t *testing.T) {
	t.Parallel()
	cmd := Command(Quick, "echo", "hi")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "hi" || cmd.TimedOut() {
		t.Errorf("out %q, err %v, timed out %v", out, err, cmd.TimedOut())
	}
}
