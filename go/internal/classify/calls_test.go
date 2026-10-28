package classify

import (
	"strings"
	"testing"
)

func TestCalls(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]string{
		"out=\"$(go tool deadcode -test ./...)\"\n[ -z \"$out\" ] || { echo \"$out\"; exit 1; }": "go tool deadcode -test ./...",
		"jq -re .version plugin.json | grep -Eq '^[0-9]+$'":                                      "jq -re .version plugin.json | grep -Eq ^[0-9]+$",
		"find scripts -name '*.sh' -exec shellcheck -S warning {} +":                             "find scripts -name *.sh -exec shellcheck -S warning {} + | shellcheck -S warning",
		"env FOO=1 eslint . && (cd web && npm test)":                                             "eslint . | npm test",
		"$RUNNER test": "$",
	} {
		if calls, ok := Calls(body); !ok || flat(calls) != want {
			t.Errorf("%q: got %q (ok %v), want %q", body, flat(calls), ok, want)
		}
	}
	if _, ok := Calls("if then"); ok {
		t.Error("a body that doesn't parse is not ok")
	}
}

// Words reads past a variable argument; Calls stops at it.
func TestWordsKeepAVariableArgument(t *testing.T) {
	t.Parallel()
	body := `git -C "$REPO" worktree add /tmp/r && $RUNNER test`
	if calls, _ := Calls(body); flat(calls) != "git -C | $" {
		t.Errorf("Calls: got %q", flat(calls))
	}
	if calls, _ := Words(body); flat(calls) != "git -C $ worktree add /tmp/r | $" {
		t.Errorf("Words: got %q", flat(calls))
	}
}

// flat is calls as one line: each call's words, calls joined by " | ".
func flat(calls [][]string) string {
	var out []string
	for _, c := range calls {
		out = append(out, strings.Join(c, " "))
	}
	return strings.Join(out, " | ")
}

func TestStatements(t *testing.T) {
	t.Parallel()
	got, ok := Statements("cd web && poetry run pytest -q\nnpx eslint . | tee out || true\nexport A=1 && make lint")
	want := []string{"cd web", "poetry run pytest -q", "npx eslint . | tee out || true", "export A=1", "make lint"}
	if !ok || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", got, want)
	}
}
