package proc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
