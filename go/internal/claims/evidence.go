package claims

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// evidence is the verifier's input: the session's prompts, tool calls,
// and results, oldest first, then text. Prompts are always whole. Within
// evidenceCap, each call and result naming what text names (namedIn) is
// kept first, newest first, then the rest newest first until one doesn't
// fit, so every item cut is older than every one kept for its age.
// A forked skill's task is its first meta prompt, kept when forked.
func evidence(entries []transcript.Entry, text string, forked bool) string {
	w := evidenceWalk{names: map[string]string{}, inputs: map[string]string{}, namedIDs: map[string]bool{}, tokens: namedIn(text), forked: forked}
	for _, e := range entries {
		w.entry(e)
	}
	items := keepWithin(w.items, w.namedIDs, evidenceCap)
	return render(items, text)
}

// evidenceItem is one prompt, tool call, result, or reply text: id is a
// tool call's, or the call's a result answers, and in is whether it's kept.
type evidenceItem struct {
	text              string
	id                string
	prompt, named, in bool
}

// evidenceWalk gathers the items: names and inputs are each tool call's,
// by its tool_use id, and namedIDs the calls whose input or result names a
// token of the text.
type evidenceWalk struct {
	names, inputs map[string]string
	namedIDs      map[string]bool
	tokens        []string
	forked        bool
	items         []evidenceItem
}

// entry takes in one transcript entry.
func (w *evidenceWalk) entry(e transcript.Entry) {
	switch {
	case e.IsMeta:
		if w.forked && e.Type == "user" {
			w.items = append(w.items, evidenceItem{text: "### Task (forked skill prompt)\n\n" + e.PromptText(), prompt: true})
			w.forked = false
		}
	case e.StartsTurn():
		w.items = append(w.items, evidenceItem{text: "### User prompt\n\n" + e.PromptText(), prompt: true})
	case e.Type == "user":
		for _, b := range e.Blocks {
			if b.Type == "tool_result" {
				w.result(b)
			}
		}
	case e.Type == "assistant":
		w.assistant(e)
	}
}

// result takes in a tool result. A subagent's report is claims of its own,
// not a read.
func (w *evidenceWalk) result(b transcript.Block) {
	name, out := w.names[b.ToolUseID], b.ResultText()
	report := name == "Agent"
	if report {
		name += " (not evidence)"
	}
	named := !report && slices.ContainsFunc(w.tokens, func(t string) bool {
		return strings.Contains(w.inputs[b.ToolUseID], t) || strings.Contains(out, t)
	})
	w.namedIDs[b.ToolUseID] = w.namedIDs[b.ToolUseID] || named
	w.items = append(w.items, evidenceItem{text: fmt.Sprintf("#### Result of %s\n\n%s", name, clip(out, resultCap)), id: b.ToolUseID, named: named})
}

// assistant takes in a reply's text and tool calls.
func (w *evidenceWalk) assistant(e transcript.Entry) {
	for _, b := range e.Blocks {
		switch b.Type {
		case "text":
			w.items = append(w.items, evidenceItem{text: "### Assistant text (not evidence)\n\n" + clip(b.Text, assistCap)})
		case "tool_use":
			w.names[b.ID], w.inputs[b.ID] = b.Name, string(b.RawInput)
			w.items = append(w.items, evidenceItem{text: fmt.Sprintf("#### Tool %s\n\nInput: %s", b.Name, clip(string(b.RawInput), inputCap)), id: b.ID})
		}
	}
}

// keepWithin marks which items fit budget: every prompt, then each named
// call and result newest first, then the rest newest first until one
// doesn't fit.
func keepWithin(items []evidenceItem, namedIDs map[string]bool, budget int) []evidenceItem {
	for i, it := range items {
		items[i].named = it.named || it.id != "" && namedIDs[it.id]
		items[i].in = it.prompt
		if it.prompt {
			budget -= len(it.text) + 2
		}
	}
	fits := func(it evidenceItem) bool {
		if size := len(it.text) + 2; size <= budget {
			budget -= size
			return true
		}
		return false
	}
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].named && !items[i].prompt {
			items[i].in = fits(items[i])
		}
	}
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].prompt || items[i].named {
			continue
		}
		if !fits(items[i]) {
			break
		}
		items[i].in = true
	}
	return items
}

// render is the kept items, oldest first, with a count of the cut ones,
// then text.
func render(items []evidenceItem, text string) string {
	cut := 0
	for _, it := range items {
		if !it.in {
			cut++
		}
	}
	var b bytes.Buffer
	b.WriteString("## Session evidence, oldest first\n\n")
	if cut > 0 {
		fmt.Fprintf(&b, "[%d older items cut]\n\n", cut)
	}
	for _, it := range items {
		if it.in {
			b.WriteString(it.text + "\n\n")
		}
	}
	b.WriteString("## Text to verify\n\n" + clip(text, textCap) + "\n")
	return b.String()
}
