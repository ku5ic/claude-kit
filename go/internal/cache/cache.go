// Package cache is the kit's state dirs under <kit home>/cache: what each
// holds and how long a file there lives unmodified. SessionStart prunes
// them silently; scratch-rotate prunes and reports.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

// The state dirs, by name.
const (
	SkillsLoaded = "skills-loaded"
	FileSkills   = "file-skills"
	PlanActive   = "plan-active"
	Statusline   = "statusline"
	ReplyLimit   = "reply-limit"
	Stack        = "stack"
	StopReports  = "stop-reports"
)

// Dir is one state dir: its name, what a file in it is, and the days one
// lives unmodified.
type Dir struct {
	Name, What string
	Days       int
}

// Dirs is every state dir the kit writes; one missing here is never pruned.
// Per-session markers last a day; a stack report is rebuilt cheaply when
// pruned, so it lasts until a project goes unused for a month.
var Dirs = []Dir{
	{SkillsLoaded, "skill-loaded marker(s)", 1},
	{FileSkills, "file-skills cache(s)", 1},
	{PlanActive, "plan-active marker(s)", 1},
	{Statusline, "statusline cache(s)", 1},
	{ReplyLimit, "reply-limit marker(s)", 1},
	{Stack, "stack report(s)", 30},
	{StopReports, "stop-checks report(s)", 1},
}

// StopReport is where stop-checks keeps its last report for the repo at
// root, for kit explain stop: the hook itself is silent unless it blocks.
func StopReport(cacheDir, root string) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(cacheDir, StopReports, hex.EncodeToString(sum[:])[:8])
}

// Expired is every regular file in dir modified more than days whole days
// before now.
func Expired(dir string, days int, now time.Time) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var old []string
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && info.Mode().IsRegular() && now.Sub(info.ModTime()) > time.Duration(days)*24*time.Hour {
			old = append(old, filepath.Join(dir, e.Name()))
		}
	}
	return old
}

// Prune deletes every state dir's expired files under cacheDir.
func Prune(cacheDir string, now time.Time) {
	for _, d := range Dirs {
		for _, path := range Expired(filepath.Join(cacheDir, d.Name), d.Days, now) {
			_ = os.Remove(path)
		}
	}
}
