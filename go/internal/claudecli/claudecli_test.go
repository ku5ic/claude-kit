package claudecli

import (
	"testing"
	"time"
)

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
