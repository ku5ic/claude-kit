// Package git runs git. Every git call in the kit goes through it, so how
// git is invoked has one owner.
package git

import (
	"bytes"
	"os/exec"
	"strings"
)

// Command is git with args, run in dir ("" is the working directory).
func Command(dir string, args ...string) *exec.Cmd {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	return exec.Command("git", args...)
}

// Output is git's stdout, as written.
func Output(dir string, args ...string) (string, error) {
	out, err := Command(dir, args...).Output()
	return string(out), err
}

// Line is git's stdout without surrounding whitespace.
func Line(dir string, args ...string) (string, error) {
	out, err := Output(dir, args...)
	return strings.TrimSpace(out), err
}

// Lines is git's stdout split into lines, empty ones dropped.
func Lines(dir string, args ...string) ([]string, error) {
	out, err := Output(dir, args...)
	var lines []string
	for line := range strings.Lines(out) {
		if line = strings.TrimRight(line, "\r\n"); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, err
}

// Files is dir's tracked and untracked-but-not-ignored files matching
// pathspecs (all when none), relative to dir.
func Files(dir string, pathspecs ...string) ([]string, error) {
	out, err := Output(dir, append([]string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, pathspecs...)...)
	var files []string
	for file := range strings.SplitSeq(out, "\x00") {
		if file != "" {
			files = append(files, file)
		}
	}
	return files, err
}

// IsBinary is git's binary heuristic: a NUL byte in the first 8000 bytes.
func IsBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0
}
