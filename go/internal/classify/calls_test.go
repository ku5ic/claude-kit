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
		calls, ok := Calls(body)
		var got []string
		for _, c := range calls {
			got = append(got, strings.Join(c, " "))
		}
		if !ok || strings.Join(got, " | ") != want {
			t.Errorf("%q: got %q (ok %v), want %q", body, got, ok, want)
		}
	}
	if _, ok := Calls("if then"); ok {
		t.Error("a body that doesn't parse is not ok")
	}
}

func TestStatements(t *testing.T) {
	t.Parallel()
	got, ok := Statements("cd web && poetry run pytest -q\nnpx eslint . | tee out || true\nexport A=1 && make lint")
	want := []string{"cd web", "poetry run pytest -q", "npx eslint . | tee out || true", "export A=1", "make lint"}
	if !ok || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", got, want)
	}
}
