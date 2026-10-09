// Package transcript reads Claude Code session transcripts: one JSON entry
// per line, user and assistant messages with their content blocks.
package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
)

// EditTools are the tools that write a file.
var EditTools = []string{"Edit", "Write", "MultiEdit", "NotebookEdit"}

// Entry is one transcript line.
type Entry struct {
	Line        int    `json:"-"` // 1-based
	Type        string `json:"type"`
	IsMeta      bool   `json:"isMeta"`
	IsSidechain bool   `json:"isSidechain"`
	Timestamp   string `json:"timestamp"`
	Message     struct {
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Attachment struct {
		Type  string `json:"type"`
		Model string `json:"model"`
	} `json:"attachment"`
	// Text is the content when it's a plain string; Blocks when it's an array.
	Text   string  `json:"-"`
	Blocks []Block `json:"-"`
}

// Block is one content block.
type Block struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Text  string `json:"text"`
	Input struct {
		FilePath     *string `json:"file_path"`
		NotebookPath *string `json:"notebook_path"`
		Skill        string  `json:"skill"`
		Command      string  `json:"command"`
	} `json:"input"`
}

// Path is the file a write tool targets: file_path, else notebook_path, else "".
func (b Block) Path() string {
	switch {
	case b.Input.FilePath != nil:
		return *b.Input.FilePath
	case b.Input.NotebookPath != nil:
		return *b.Input.NotebookPath
	}
	return ""
}

// StartsTurn reports whether e is a real user prompt: a user entry that is
// neither meta nor a tool result.
func (e Entry) StartsTurn() bool {
	return e.Type == "user" && !e.IsMeta && !slices.ContainsFunc(e.Blocks, func(b Block) bool { return b.Type == "tool_result" })
}

// PromptText is the entry's text: its string content, or its text blocks joined.
func (e Entry) PromptText() string {
	if e.Text != "" {
		return e.Text
	}
	var texts []string
	for _, b := range e.Blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// ToolUses is the entry's tool_use blocks.
func (e Entry) ToolUses() []Block {
	var uses []Block
	for _, b := range e.Blocks {
		if b.Type == "tool_use" {
			uses = append(uses, b)
		}
	}
	return uses
}

// Each calls fn for every entry in the transcript at path, in order;
// unparseable lines are skipped.
func Each(path string, fn func(Entry)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for n := 1; scanner.Scan(); n++ {
		if e, ok := parse(scanner.Bytes()); ok {
			e.Line = n
			fn(e)
		}
	}
	return scanner.Err()
}

// Tail is the parsed entries among the last n lines of the transcript,
// reading only its final 4MB. Line stays 0: the start isn't read.
func Tail(path string, n int) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const window = 4 << 20
	if info, err := f.Stat(); err == nil && info.Size() > window {
		_, _ = f.Seek(-window, io.SeekEnd) // failing, it reads from the start: slower, same tail
	}
	var lines [][]byte
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		lines = append(lines, slices.Clone(scanner.Bytes()))
	}
	if err := scanner.Err(); err != nil {
		// What was read before the error is not the file's tail.
		return nil, err
	}
	var entries []Entry
	for _, line := range lines[max(0, len(lines)-n):] {
		if e, ok := parse(line); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func parse(line []byte) (Entry, bool) {
	var e Entry
	if json.Unmarshal(line, &e) != nil {
		return e, false
	}
	if json.Unmarshal(e.Message.Content, &e.Text) != nil {
		_ = json.Unmarshal(e.Message.Content, &e.Blocks)
	}
	return e, true
}
