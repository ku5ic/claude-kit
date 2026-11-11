// Package claudecli reads the envelope `claude -p --output-format json`
// prints, which the claim verifier and the gap-fill classifier both run.
package claudecli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Usage is how many runs there were, what they cost, and how long they
// took, as their envelopes say.
type Usage struct {
	Runs     int
	CostUSD  float64
	Duration time.Duration
}

// Pairs is u as log pairs.
func (u Usage) Pairs() []string {
	return []string{"runs", strconv.Itoa(u.Runs), "cost_usd", strconv.FormatFloat(u.CostUSD, 'f', 4, 64),
		"duration_ms", strconv.FormatInt(u.Duration.Milliseconds(), 10)}
}

// Add is u and v summed.
func (u Usage) Add(v Usage) Usage {
	return Usage{u.Runs + v.Runs, u.CostUSD + v.CostUSD, u.Duration + v.Duration}
}

// Parse is out's structured answer and usage. A run that errored or gave no
// answer still reports what it cost.
func Parse(out []byte) (json.RawMessage, Usage, error) {
	var envelope struct {
		IsError    bool            `json:"is_error"`
		Subtype    string          `json:"subtype"`
		Structured json.RawMessage `json:"structured_output"`
		CostUSD    float64         `json:"total_cost_usd"`
		DurationMS int64           `json:"duration_ms"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return nil, Usage{Runs: 1}, fmt.Errorf("malformed output: %w", err)
	}
	u := Usage{1, envelope.CostUSD, time.Duration(envelope.DurationMS) * time.Millisecond}
	if envelope.IsError || len(envelope.Structured) == 0 {
		return nil, u, fmt.Errorf("no answer (%s)", envelope.Subtype)
	}
	return envelope.Structured, u, nil
}
