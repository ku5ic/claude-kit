// Package git runs git. Every git call in the kit goes through it, so how
// git is invoked, and how long it may take, has one owner.
package git

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/proc"
)

// Timeout bounds every git call: a lock or a hung remote must not hang a
// hook.
const Timeout = 30 * time.Second

// Command is git with args, run in dir ("" is the working directory).
func Command(dir string, args ...string) *proc.Cmd {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	return proc.Command(Timeout, "git", args...)
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

// Paths is git's NUL-separated stdout (a -z listing), empty ones dropped.
func Paths(dir string, args ...string) ([]string, error) {
	out, err := Output(dir, args...)
	var paths []string
	for path := range strings.SplitSeq(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, err
}

// Files is dir's tracked and untracked-but-not-ignored files matching
// pathspecs (all when none), relative to dir.
func Files(dir string, pathspecs ...string) ([]string, error) {
	return Paths(dir, append([]string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, pathspecs...)...)
}

var toplevels sync.Map // dir -> its work tree's root, "" outside one

// Toplevel is the root of the git work tree holding dir, "" outside one;
// git resolves symlinks, so it's the physical path. Asked once per dir per
// process: a hook asks it from several places.
func Toplevel(dir string) string {
	if top, ok := toplevels.Load(dir); ok {
		return top.(string)
	}
	top, err := Line(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		top = ""
	}
	toplevels.Store(dir, top)
	return top
}

// Branch is the branch checked out in dir's repo, "" on a detached HEAD; it
// errors outside a repo.
func Branch(dir string) (string, error) {
	return Line(dir, "branch", "--show-current")
}

// IsBinary is git's binary heuristic: a NUL byte in the first 8000 bytes.
func IsBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0
}

// Change is one path git status reports: XY its two status letters ("??"
// untracked), Path its path from the repo root, the new one for a rename.
type Change struct{ XY, Path string }

// Status is the working tree's changes in dir's repo, as git status
// --porcelain lists them: an untracked directory is one entry unless
// allUntracked lists its files.
func Status(dir string, allUntracked bool) ([]Change, error) {
	mode := "--untracked-files=normal"
	if allUntracked {
		mode = "--untracked-files=all"
	}
	out, err := Output(dir, "status", "--porcelain", "-z", mode)
	if err != nil {
		return nil, err
	}
	var changes []Change
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		if len(fields[i]) < 4 {
			continue
		}
		c := Change{fields[i][:2], fields[i][3:]}
		if strings.ContainsAny(c.XY, "RC") {
			i++ // the rename's or copy's source follows
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// Snapshot is a tree object holding dir's working tree as it is now,
// tracked and untracked files alike, ignored ones left out: two snapshots
// are equal only when no such file's content changed. It stages into a temp
// index seeded from the real one, so only changed files are rehashed, and
// the real index is never touched.
func Snapshot(dir string) (string, error) {
	gitDir, err := Line(dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp("", "kit-index-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	// No index yet (a repo with nothing staged) leaves none to seed: git
	// reads a missing index as empty, and an empty file as corrupt.
	tmp := filepath.Join(tmpDir, "index")
	switch index, err := os.ReadFile(filepath.Join(gitDir, "index")); {
	case err == nil:
		if err := os.WriteFile(tmp, index, 0o600); err != nil {
			return "", err
		}
	case !errors.Is(err, os.ErrNotExist):
		return "", err
	}
	withIndex := func(args ...string) (string, error) {
		cmd := Command(dir, args...)
		cmd.Env = append(cmd.Environ(), "GIT_INDEX_FILE="+tmp)
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	if _, err := withIndex("add", "-A", "--", ":/"); err != nil {
		return "", err
	}
	return withIndex("write-tree")
}

// TreeDiff is the paths, from the repo root, whose content differs between
// trees from and to.
func TreeDiff(dir, from, to string) ([]string, error) {
	return Paths(dir, "diff-tree", "-r", "-z", "--name-only", "--no-renames", from, to)
}

// Verify reports whether ref names a commit in dir's repo.
func Verify(dir, ref string) bool {
	_, err := Line(dir, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

// Base is the base ref for the repository at dir ("" is the working
// directory), as kit git-base finds it: explicit when it resolves, else the
// upstream unless that's only this branch's own push target, else
// origin/HEAD, else main, master, develop, or trunk. False when nothing
// resolves.
func Base(dir, explicit string) (string, bool) {
	run := func(args ...string) (string, error) { return Line(dir, args...) }
	if explicit != "" && Verify(dir, explicit) {
		return explicit, true
	}
	if upstream, err := run("rev-parse", "--abbrev-ref", "@{upstream}"); err == nil {
		current, _ := run("rev-parse", "--abbrev-ref", "HEAD")
		// ${upstream#*/}: drop the remote name; no slash leaves it whole.
		branch := upstream
		if _, rest, found := strings.Cut(upstream, "/"); found {
			branch = rest
		}
		if branch != current {
			return upstream, true
		}
	}
	if _, err := run("symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		if resolved, err := run("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && Verify(dir, resolved) {
			return resolved, true
		}
	}
	for _, b := range []string{"main", "master", "develop", "trunk"} {
		if Verify(dir, b) {
			return b, true
		}
	}
	return "", false
}
