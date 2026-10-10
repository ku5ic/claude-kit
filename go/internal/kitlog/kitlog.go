// Package kitlog is the kit's JSONL logs: the line schema, appending under
// a size cap, and reading every line back. Appends take a shared lock and a
// trim an exclusive one, so concurrent hooks never lose a line to a trim.
package kitlog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// The kit's logs, the skills log's events that mark a skill as only
// surfaced, and a line's ts layout.
const (
	Skills              = "skills"
	Guards              = "guards"
	EventRequiredSkill  = "required-skill"
	EventSuggestedSkill = "suggested-skill"
	TimeLayout          = "2006-01-02T15:04:05Z"
)

// Entry is one log line as the readers use it; a pointer keeps a null
// distinct from "".
type Entry struct {
	TS            *string `json:"ts"`
	Event         string  `json:"event"`
	SessionID     *string `json:"session_id"`
	ExpansionType string  `json:"expansion_type"`
	CommandName   *string `json:"command_name"`
	SkillFile     *string `json:"skill_file"`
	ToolName      string  `json:"tool_name"`
	Rule          *string `json:"rule"`
}

// Surfaced is true for a skills log line that says a skill was shown, not
// loaded.
func (e Entry) Surfaced() bool {
	return e.Event == EventRequiredSkill || e.Event == EventSuggestedSkill
}

// Line is one log line: a JSON object of pairs' keys and values in order,
// an empty value as null, ending in a newline.
func Line(pairs ...string) []byte {
	var line bytes.Buffer
	line.WriteByte('{')
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			line.WriteByte(',')
		}
		writeString(&line, pairs[i])
		line.WriteByte(':')
		if pairs[i+1] == "" {
			line.WriteString("null")
		} else {
			writeString(&line, pairs[i+1])
		}
	}
	line.WriteString("}\n")
	return line.Bytes()
}

func writeString(buf *bytes.Buffer, s string) {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	buf.Write(bytes.TrimSuffix(tmp.Bytes(), []byte("\n")))
}

// minLineBytes is less than any line Line writes for a hook: ts, hook,
// event, and session_id alone take more. A log no bigger than maxLines of
// these can't be over its cap, so most appends never read the file.
const minLineBytes = 64

// Append adds line to path, creating it and its directory. Past maxLines
// lines plus a tenth, it trims the log to its newest maxLines, so a full log
// is rewritten once per tenth instead of on every append; maxLines 0 never
// trims.
func Append(path string, line []byte, maxLines int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	err = withLock(f, syscall.LOCK_SH, func() error {
		_, err := f.Write(line)
		return err
	})
	var size int64
	if info, serr := f.Stat(); serr == nil {
		size = info.Size()
	}
	if err = errors.Join(err, f.Close()); err != nil || maxLines <= 0 || size <= int64(maxLines)*minLineBytes {
		return err
	}
	return trim(path, maxLines)
}

// trim rewrites path in place to its newest maxLines lines once it holds
// more than maxLines plus a tenth, under an exclusive lock: appenders wait,
// and their O_APPEND writes land after the rewrite.
func trim(path string, maxLines int) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	return withLock(f, syscall.LOCK_EX, func() error {
		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		lines := bytes.SplitAfter(data, []byte("\n"))
		if n := len(lines); n > 0 && len(lines[n-1]) == 0 {
			lines = lines[:n-1]
		}
		if len(lines) <= maxLines+max(1, maxLines/10) {
			return nil
		}
		kept := bytes.Join(lines[len(lines)-maxLines:], nil)
		if _, err := f.WriteAt(kept, 0); err != nil {
			return err
		}
		return f.Truncate(int64(len(kept)))
	})
}

// Each calls fn with every line of path, oldest first, under a shared lock
// so a trim can't rewrite it mid-read. A missing log has no lines; an
// oversized line is passed whole.
func Each(path string, fn func(line []byte)) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	return withLock(f, syscall.LOCK_SH, func() error {
		r := bufio.NewReader(f)
		for {
			line, err := r.ReadBytes('\n')
			if len(line) > 0 {
				fn(line)
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
		}
	})
}

func withLock(f *os.File, how int, fn func() error) error {
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }() // closing the file unlocks it too
	return fn()
}
