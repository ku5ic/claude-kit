package sources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

func TestLoadConfigWarnsOnAnUnknownTaskProvider(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base, overlay := filepath.Join(dir, "kit.yml"), filepath.Join(dir, "over.yml")
	if err := os.WriteFile(base, []byte("protected_branches: [main]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, []byte("disabled_task_providers: [make, mkae]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, warnings, err := LoadConfig(config.Paths{Base: base, Overlay: overlay})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || warnings[0].File != overlay || warnings[0].Err.Error() != `disabled_task_providers: "mkae" names no task provider` {
		t.Errorf("warnings = %v", warnings)
	}
}
