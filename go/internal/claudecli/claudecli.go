// Package claudecli runs `claude -p --output-format json` and reads the
// envelope it prints, for the claim verifier and the gap-fill classifier.
package claudecli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/proc"
	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
)

// DepthVar is set in every claude run the kit starts, so a run whose hooks
// didn't stand down can't start another.
const DepthVar = "KIT_CLAUDE_DEPTH"

// ErrNested is Guarded's error inside a claude run the kit started.
var ErrNested = errors.New("already inside a claude run the kit started")

// Guarded is the environment for a claude run the kit starts: guard set to
// a fresh random value, with a marker named by its hash under markerDir
// that tells the run's own hooks to stand down, and DepthVar set. cleanup
// removes the marker. A value set any other way, by a user or an rc file,
// has no marker, so it turns nothing off.
func Guarded(markerDir, guard string) (env []string, cleanup func(), err error) {
	if os.Getenv(DepthVar) != "" {
		return nil, nil, ErrNested
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, nil, err
	}
	value := hex.EncodeToString(token)
	marker := markerPath(markerDir, value)
	if err := os.MkdirAll(markerDir, 0o700); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		return nil, nil, err
	}
	env = append(os.Environ(), guard+"="+value, DepthVar+"=1")
	return env, func() { _ = os.Remove(marker) }, nil
}

// Honored is true when guard's value in this process's environment has its
// marker under markerDir: this is a claude run the kit started.
func Honored(markerDir, guard string) bool {
	value := os.Getenv(guard)
	if value == "" {
		return false
	}
	_, err := os.Stat(markerPath(markerDir, value))
	return err == nil
}

func markerPath(markerDir, value string) string {
	sum := sha256.Sum256([]byte(value))
	return filepath.Join(markerDir, hex.EncodeToString(sum[:]))
}

// Run runs argv from dir with stdin, for at most timeout, guarded as
// Guarded says, and is its structured answer. A run that started counts in
// Usage whether it timed out, failed, or answered.
func Run(timeout time.Duration, argv []string, dir string, stdin io.Reader, markerDir, guard string) (json.RawMessage, Usage, error) {
	env, cleanup, err := Guarded(markerDir, guard)
	if err != nil {
		return nil, Usage{}, err
	}
	defer cleanup()
	// A path is the configured binary itself, relative to dir as
	// exec.Command takes it; only a bare name goes through PATH.
	bin := argv[0]
	if !strings.Contains(bin, "/") {
		if bin, err = kittool.Path(bin); err != nil {
			return nil, Usage{}, err
		}
	}
	cmd := proc.Command(timeout, bin, argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdin = dir, env, stdin
	out, err := cmd.Output()
	if cmd.TimedOut() {
		return nil, Usage{Runs: 1}, errors.New("timed out")
	}
	if err != nil {
		return nil, Usage{Runs: 1}, err
	}
	return Parse(out)
}

// Usage is how many runs there were, what they cost, and how long they
// took, as their envelopes say.
type Usage struct {
	Runs     int
	CostUSD  float64
	Duration time.Duration
}

// Pairs is u as log pairs.
func (u Usage) Pairs() []string {
	return []string{"runs", strconv.Itoa(u.Runs), "cost_usd", strconv.FormatFloat(u.CostUSD, 'f', 4, 64),
		"duration_ms", strconv.FormatInt(u.Duration.Milliseconds(), 10)}
}

// Add is u and v summed.
func (u Usage) Add(v Usage) Usage {
	return Usage{u.Runs + v.Runs, u.CostUSD + v.CostUSD, u.Duration + v.Duration}
}

// Parse is out's structured answer and usage. A run that errored or gave no
// answer still reports what it cost.
func Parse(out []byte) (json.RawMessage, Usage, error) {
	var envelope struct {
		IsError    bool            `json:"is_error"`
		Subtype    string          `json:"subtype"`
		Structured json.RawMessage `json:"structured_output"`
		CostUSD    float64         `json:"total_cost_usd"`
		DurationMS int64           `json:"duration_ms"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return nil, Usage{Runs: 1}, fmt.Errorf("malformed output: %w", err)
	}
	u := Usage{1, envelope.CostUSD, time.Duration(envelope.DurationMS) * time.Millisecond}
	if envelope.IsError || len(envelope.Structured) == 0 {
		return nil, u, fmt.Errorf("no answer (%s)", envelope.Subtype)
	}
	return envelope.Structured, u, nil
}
