package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestADRNudge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(root, "docs/adr/0001-hooks-in-go.md"),
		"# 1. Hooks in Go\n\n## Status\n\nAccepted\n\n## Context\n\nShell was slow.\n\n## Decision\n\nEvery hook in `go/internal/hooks/` is Go.\n\n## Consequences\n\nNone.\n")
	testutil.Write(t, filepath.Join(root, "docs/adr/0002-old.md"),
		"# 2. Old\n\n## Status\n\nSuperseded by [3](0003-new.md)\n\n## Decision\n\nHooks in `go/internal/hooks/` are shell.\n")
	testutil.Write(t, filepath.Join(root, "docs/adr/0003-lang.md"),
		"# 3. Language\n\n## Status\n\nAccepted\n\n## Decision\n\nThe `go` toolchain only.\n")
	testutil.Write(t, filepath.Join(root, "go/internal/hooks/edit.go"), "package hooks\n")
	testutil.Write(t, filepath.Join(root, "go/main.go"), "package main\n")
	k := newSandbox(t)
	nudge := func(tool, path, session string) result {
		t.Helper()
		r := k.run("adr-nudge", editPayload(tool, filepath.Join(root, path), session, root))
		r.Allowed(t)
		return r
	}

	first := nudge("Read", "go/internal/hooks/edit.go", "s1")
	first.Has(t, "1. Hooks in Go", "Every hook in `go/internal/hooks/` is Go.", "docs/adr/0001-hooks-in-go.md")
	for _, absent := range []string{"2. Old", "3. Language"} {
		if strings.Contains(first.Output, absent) {
			t.Errorf("nudged %q, a superseded ADR or one citing only a bare word:\n%s", absent, first.Output)
		}
	}
	if again := nudge("Edit", "go/internal/hooks/edit.go", "s1"); strings.Contains(again.Output, "Hooks in Go") {
		t.Errorf("the same ADR nudged twice in a session:\n%s", again.Output)
	}
	nudge("Edit", "go/internal/hooks/edit.go", "s2").Has(t, "1. Hooks in Go")
	if other := nudge("Read", "go/main.go", "s3"); strings.Contains(other.Output, "Hooks in Go") {
		t.Errorf("a file the ADR doesn't cite got its nudge:\n%s", other.Output)
	}

	madr := t.TempDir()
	if err := os.Mkdir(filepath.Join(madr, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(madr, "docs/decisions/0001-api.md"),
		"---\nstatus: \"accepted\"\n---\n\n# REST for the API\n\n## Decision Outcome\n\nChosen option: REST, in `api/handlers.py`.\n")
	testutil.Write(t, filepath.Join(madr, "api/handlers.py"), "")
	k.run("adr-nudge", editPayload("Read", filepath.Join(madr, "api/handlers.py"), "s4", madr)).Has(t, "REST for the API", "Chosen option: REST")

	off := newPlugin(t)
	off.kitYML("disabled_rules: [adr-nudge]\n")
	if r := off.run("adr-nudge", editPayload("Read", filepath.Join(root, "go/internal/hooks/edit.go"), "s1", root)); strings.Contains(r.Output, "Hooks in Go") {
		t.Errorf("disabled_rules: [adr-nudge] still nudged:\n%s", r.Output)
	}
}
