package claudecli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Not parallel: these set the process environment.
func TestGuardedMarker(t *testing.T) {
	const guard = "KIT_TEST_GUARD"
	markers := filepath.Join(t.TempDir(), "guard")
	t.Run("a value the user set has no marker and turns nothing off", func(t *testing.T) {
		t.Setenv(guard, "1")
		if Honored(markers, guard) {
			t.Error("a hand-set value was honored")
		}
	})
	t.Run("the value Guarded sets is honored until cleanup", func(t *testing.T) {
		env, cleanup, err := Guarded(markers, guard)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(env, func(kv string) bool { return strings.HasPrefix(kv, guard+"=") })
		if i < 0 || !slices.Contains(env, DepthVar+"=1") {
			t.Fatalf("env lacks %s or %s: %q", guard, DepthVar, env)
		}
		t.Setenv(guard, strings.TrimPrefix(env[i], guard+"="))
		if !Honored(markers, guard) {
			t.Error("the kit's own value wasn't honored")
		}
		cleanup()
		if Honored(markers, guard) {
			t.Error("the value outlived its run")
		}
	})
	t.Run("inside a run the kit started, another can't start", func(t *testing.T) {
		t.Setenv(DepthVar, "1")
		if _, _, err := Guarded(markers, guard); !errors.Is(err, ErrNested) {
			t.Errorf("err = %v, want ErrNested", err)
		}
		if left, _ := os.ReadDir(markers); len(left) != 0 {
			t.Errorf("a refused run left a marker: %v", left)
		}
	})
}

func TestParse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, out string
		want      Usage
		answer    string
		fails     bool
	}{
		{"an answer with its cost", `{"is_error":false,"subtype":"success","structured_output":{"ok":true},"total_cost_usd":0.62,"duration_ms":9009}`,
			Usage{1, 0.62, 9009 * time.Millisecond}, `{"ok":true}`, false},
		{"an answer without a cost", `{"is_error":false,"structured_output":{"ok":true}}`,
			Usage{Runs: 1}, `{"ok":true}`, false},
		{"an error still reports its cost", `{"is_error":true,"subtype":"error_max_budget_usd","total_cost_usd":0.5,"duration_ms":10}`,
			Usage{1, 0.5, 10 * time.Millisecond}, "", true},
		{"output that isn't JSON", `oops`, Usage{Runs: 1}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			answer, u, err := Parse([]byte(tc.out))
			if (err != nil) != tc.fails || u != tc.want || string(answer) != tc.answer {
				t.Errorf("Parse = %s, %+v, %v; want %s, %+v, fails %v", answer, u, err, tc.answer, tc.want, tc.fails)
			}
		})
	}
}

func TestUsagePairs(t *testing.T) {
	t.Parallel()
	u := Usage{1, 0.25, 1500 * time.Millisecond}.Add(Usage{1, 0.5, 500 * time.Millisecond})
	got := u.Pairs()
	want := []string{"runs", "2", "cost_usd", "0.7500", "duration_ms", "2000"}
	if len(got) != len(want) {
		t.Fatalf("Pairs = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Pairs = %q, want %q", got, want)
		}
	}
}
