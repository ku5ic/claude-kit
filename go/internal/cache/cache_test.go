package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneDropsOnlyExpiredFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	files := map[string]time.Duration{
		"skills-loaded/s1-old": 2 * 24 * time.Hour,
		"skills-loaded/s1-new": time.Hour,
		"stack/p-old":          31 * 24 * time.Hour,
		"stack/p-new":          2 * 24 * time.Hour,
		"unlisted/x":           400 * 24 * time.Hour,
	}
	for name, age := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	Prune(dir, now)
	for name, gone := range map[string]bool{
		"skills-loaded/s1-old": true, "skills-loaded/s1-new": false,
		"stack/p-old": true, "stack/p-new": false, "unlisted/x": false,
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); (err != nil) != gone {
			t.Errorf("%s: gone = %v, want %v", name, err != nil, gone)
		}
	}
}
