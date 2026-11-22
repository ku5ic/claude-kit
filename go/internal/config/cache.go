package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
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

// ConfigCache is where Load keeps its last result, "" when Paths has no ClaudeDir
// (tests building Paths by hand), which would land it in the cwd.
func (p Paths) ConfigCache() string {
	if p.ClaudeDir == "" {
		return ""
	}
	return filepath.Join(p.CacheDir(), "config.gob")
}

// cacheKey names the exact files behind Load's result: path, size, and
// mtime of the base, the overlay, the manifest declaring the /config
// options, every preset file, and the running binary, whose Config struct
// shapes what was stored, then settingsKey. Any edit or rebuild changes it;
// "" when the base or binary can't be stat'ed, so Load skips the cache.
func cacheKey(p Paths) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	presets, _ := filepath.Glob(filepath.Join(p.Root, "presets", "*.yml"))
	key := ""
	for _, path := range append([]string{exe, p.Base, p.Overlay, p.Manifest}, presets...) {
		info, err := os.Stat(path)
		switch {
		case err == nil:
			key += fmt.Sprintf("%s %d %d\n", path, info.Size(), info.ModTime().UnixNano())
		case path == exe || path == p.Base:
			return ""
		}
	}
	return key + settingsKey(p.Settings)
}

// settingsKey is a hash of the two settings.json keys pluginOptions reads,
// since Claude Code's other settings share the file; "" when it's absent,
// and the raw bytes' hash when it isn't JSON, so the warning follows edits.
func settingsKey(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var settings struct {
		EnabledPlugins json.RawMessage `json:"enabledPlugins"`
		PluginConfigs  json.RawMessage `json:"pluginConfigs"`
	}
	if json.Unmarshal(data, &settings) == nil {
		data = append(settings.EnabledPlugins, settings.PluginConfigs...)
	}
	return fmt.Sprintf("%s %x\n", path, sha256.Sum256(data))
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
