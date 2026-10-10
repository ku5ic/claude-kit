package enforce

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// Mark is a gate the kit saw change files: its label, the paths it changed
// (from the root), and the restore point taken before that run.
type Mark struct {
	Label string   `json:"label"`
	Paths []string `json:"paths"`
	Ref   string   `json:"ref"`
}

// marksFile holds root's marks, by gate key.
func marksFile(cacheDir, root string) string {
	return filepath.Join(cacheDir, cache.Enforce, cache.RootKey(root)+".mutated.json")
}

// key names g among root's gates: where it runs and what it runs.
func (g Gate) key(root string) string {
	sum := sha256.Sum256([]byte(fsx.Rel(root, g.Dir) + "\x00" + g.Body))
	return hex.EncodeToString(sum[:8])
}

// Marks is root's marks, by gate key.
func Marks(cacheDir, root string) map[string]Mark {
	m := sources.Decode[map[string]Mark](marksFile(cacheDir, root), json.Unmarshal)
	if m == nil {
		m = map[string]Mark{}
	}
	return m
}

// AddMark records that g changed files, so it never runs again until
// ResetMarks.
func AddMark(cacheDir, root string, g Gate, m Mark) error {
	marks := Marks(cacheDir, root)
	marks[g.key(root)] = m
	data, err := json.Marshal(marks)
	if err != nil {
		return err
	}
	return fsx.WriteAtomic(marksFile(cacheDir, root), data, 0o644)
}

// ResetMarks forgets root's marks.
func ResetMarks(cacheDir, root string) error {
	if err := os.Remove(marksFile(cacheDir, root)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// skipMarked skips each gate root's marks name.
func skipMarked(gates []Gate, cacheDir, root string) {
	marks := Marks(cacheDir, root)
	for i := range gates {
		if m, ok := marks[gates[i].key(root)]; ok && gates[i].Skip == "" {
			gates[i].Skip = "changed " + strings.Join(m.Paths, ", ") + " when it last ran; kit gates reset lets it run again"
		}
	}
}
