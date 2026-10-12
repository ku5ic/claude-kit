package config

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ku5ic/claude-kit/go/internal/fsx"
)

// cached is Load's result as stored on disk; a Warning's error is kept as
// its message.
type cached struct {
	Key      string
	Cfg      Config
	Warnings [][2]string
}

// cacheFile is where Load keeps its last result, "" when Paths has no Home
// (tests building Paths by hand), which would land it in the cwd.
func (p Paths) cacheFile() string {
	if p.Home == "" {
		return ""
	}
	return filepath.Join(p.CacheDir(), "config.gob")
}

// cacheKey names the exact files behind Load's result: path, size, and
// mtime of the base, the overlay, and the running binary, whose Config
// struct shapes what was stored. Any edit or rebuild changes it; "" when the
// base or binary can't be stat'ed, so Load skips the cache.
func cacheKey(p Paths) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	key := ""
	for _, path := range []string{exe, p.Base, p.Overlay} {
		info, err := os.Stat(path)
		switch {
		case err == nil:
			key += fmt.Sprintf("%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
		case path != p.Overlay:
			return ""
		}
	}
	return key
}

// loadCached is the stored result for key, ok false on any miss.
func loadCached(file, key string) (*Config, []Warning, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, false
	}
	var c cached
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&c) != nil || c.Key != key {
		return nil, nil, false
	}
	warnings := make([]Warning, len(c.Warnings))
	for i, w := range c.Warnings {
		warnings[i] = Warning{w[0], errors.New(w[1])}
	}
	return &c.Cfg, warnings, true
}

// storeCached writes the result for key; a failed write only costs the
// next hook a fresh parse.
func storeCached(file, key string, cfg *Config, warnings []Warning) {
	c := cached{Key: key, Cfg: *cfg}
	for _, w := range warnings {
		c.Warnings = append(c.Warnings, [2]string{w.File, w.Err.Error()})
	}
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(c) == nil {
		_ = fsx.WriteAtomic(file, buf.Bytes(), 0o600)
	}
}
