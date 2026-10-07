package hooks

import (
	"bufio"
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	// doneStep matches a ticked checklist item.
	doneStep    = regexp.MustCompile(`(?m)^\s*[-*] \[[xX]\]`)
	typedReview = regexp.MustCompile(`^\s*/code-review(\s|$)|<command-name>/code-review</command-name>`)
)

// planDone reads the Stop hook's end-of-plan moment (rules/verify.md) from
// the transcript: plan is the plan in plansDir whose last open step this
// turn ticked, else ""; reviewed is whether /code-review ran after the last
// code edit, anywhere in the transcript. isCode is false for paths whose
// edits aren't code (scratch, outside the repo).
func planDone(transcript, plansDir string, isCode func(string) bool) (plan string, reviewed bool) {
	f, err := os.Open(transcript)
	if err != nil {
		return "", false
	}
	defer f.Close()

	type entry struct {
		Type    string `json:"type"`
		IsMeta  bool   `json:"isMeta"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	type block struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Input struct {
			FilePath     string `json:"file_path"`
			NotebookPath string `json:"notebook_path"`
			Skill        string `json:"skill"`
		} `json:"input"`
	}

	// Transcript line numbers; -1 is never.
	lastEdit, lastReview := -1, -1
	var ticked []string // plans this turn edited
	n := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		n++
		var e entry
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			continue
		}
		var blocks []block
		isArray := json.Unmarshal(e.Message.Content, &blocks) == nil
		switch e.Type {
		case "user":
			// A typed /code-review arrives as the user's own message:
			// "/code-review high", or a <command-name> block.
			var text string
			if json.Unmarshal(e.Message.Content, &text) == nil && typedReview.MatchString(text) {
				lastReview = n
			}
			if e.IsMeta || isArray && slices.ContainsFunc(blocks, func(b block) bool { return b.Type == "tool_result" }) {
				continue
			}
			ticked = ticked[:0] // a new turn starts
		case "assistant":
			for _, b := range blocks {
				switch {
				case b.Type != "tool_use":
				case b.Name == "Skill" && b.Input.Skill == "code-review":
					lastReview = n
				case slices.Contains([]string{"Edit", "Write", "MultiEdit", "NotebookEdit"}, b.Name):
					path := cmp.Or(b.Input.FilePath, b.Input.NotebookPath)
					if strings.HasPrefix(path, plansDir+string(filepath.Separator)) {
						ticked = append(ticked, path)
					} else if isCode(path) {
						lastEdit = n
					}
				}
			}
		}
	}

	for _, path := range ticked {
		if data, err := os.ReadFile(path); err == nil && !openStep.Match(data) && doneStep.Match(data) {
			plan = path
		}
	}
	return plan, lastReview > lastEdit
}
