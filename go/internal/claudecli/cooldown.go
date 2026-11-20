package claudecli

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// CooldownPeriod is how long a key a claude run failed on waits before its
// caller runs claude for it again: asking again sooner would likely fail
// again, and each try can take the run's whole timeout.
const CooldownPeriod = time.Hour

// cooldownLockWait bounds how long Record waits for another run's write.
const cooldownLockWait = 2 * time.Second

// Cooldown is one caller's failed claude runs, by a key the caller picks:
// gap-fill's per entry, the claim verifier's one for itself. The zero
// Cooldown holds and records nothing.
type Cooldown struct{ file string }

// Failure is when a run on a key failed, and its error.
type Failure struct {
	At  time.Time `json:"at"`
	Why string    `json:"why"`
}

// Until is when the failure stops holding its key back.
func (f Failure) Until() time.Time { return f.At.Add(CooldownPeriod) }

// holds is true while f holds its key back at now. A stamp ahead of the
// clock, a clock set back, can't say when it ends, so it holds nothing.
func (f Failure) holds(now time.Time) bool {
	return !f.At.After(now) && now.Before(f.Until())
}

// NewCooldown is caller's cooldown under cacheDir.
func NewCooldown(cacheDir, caller string) Cooldown {
	return Cooldown{file: filepath.Join(cacheDir, cache.Cooldown, caller+".json")}
}

// Cooling is key's failure, and true, while it holds the key back at now.
func (c Cooldown) Cooling(key string, now time.Time) (Failure, bool) {
	f, ok := c.failures()[key]
	return f, ok && f.holds(now)
}

// Record stamps each key of failed with its error at now and clears each
// of cleared, dropping the failures that no longer hold, under a lock.
func (c Cooldown) Record(now time.Time, failed map[string]string, cleared []string) error {
	if c.file == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), cooldownLockWait)
	defer cancel()
	unlock, err := cache.Lock(ctx, strings.TrimSuffix(c.file, ".json")+".lock")
	if err != nil {
		return err
	}
	defer unlock()
	all := c.failures()
	before := maps.Clone(all)
	for key, why := range failed {
		all[key] = Failure{At: now, Why: why}
	}
	for _, key := range cleared {
		delete(all, key)
	}
	maps.DeleteFunc(all, func(_ string, f Failure) bool { return !f.holds(now) && !f.At.Equal(now) })
	if maps.Equal(all, before) {
		return nil
	}
	data, err := json.Marshal(all)
	if err != nil {
		return err
	}
	return fsx.WriteAtomic(c.file, data, 0o600)
}

// failures is the caller's file read; none when it's missing or unreadable.
func (c Cooldown) failures() map[string]Failure {
	all := map[string]Failure{}
	if data, err := os.ReadFile(c.file); err == nil {
		_ = json.Unmarshal(data, &all)
	}
	return all
}
