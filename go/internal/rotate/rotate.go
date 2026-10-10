// Package rotate is scratch-rotate: it prunes scratch artifacts past a
// retention window and trims the kit's JSONL logs.
//
// The project tier deletes files of any extension at any depth, so every
// scratch-registry.txt line is validated before anything under it is
// touched: absolute, under $HOME, a real directory named exactly scratch,
// not a symlink, not a repo. A single bad line would otherwise mean
// unrecoverable mass deletion. Every deleted path goes to the rotate log.
package rotate

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// Run is `kit scratch-rotate [days] [--dry-run]`; it returns the exit status.
func Run(paths config.Paths, n int, dryRun bool, stdout, stderr io.Writer) int {
	r := &rotator{now: time.Now(), dryRun: dryRun, log: filepath.Join(paths.LogDir(), "scratch-rotate.log"),
		verb: "deleted", pruned: "pruned", stdout: stdout}
	if dryRun {
		r.verb, r.pruned = "would-delete", "would prune"
	}
	_ = os.MkdirAll(paths.LogDir(), 0o755)

	scratch := paths.ScratchHome()
	if fsx.IsDir(scratch) {
		// .md artifacts by the retention window; .injected-* session
		// markers after a day: they only dedupe within a session.
		removed := r.prune(scratch, n, false, func(name string) bool { return strings.HasSuffix(name, ".md") })
		markers := r.prune(scratch, 1, true, func(name string) bool { return strings.HasPrefix(name, ".injected-") })
		fmt.Fprintf(stdout, "scratch-rotate: %s %d artifact(s) older than %dd from %s\n", r.pruned, removed, n, scratch)
		fmt.Fprintf(stdout, "scratch-rotate: %s %d session marker(s) older than 1d from %s\n", r.pruned, markers, scratch)
	}
	for _, m := range []struct{ kind, what string }{
		{config.SkillsLoaded, "skill-loaded marker(s)"},
		{config.FileSkills, "file-skills cache(s)"},
		{config.PlanActive, "plan-active marker(s)"},
		{config.Statusline, "statusline cache(s)"},
		{config.ReplyLimit, "reply-limit marker(s)"},
	} {
		if dir := filepath.Join(paths.CacheDir(), m.kind); fsx.IsDir(dir) {
			fmt.Fprintf(stdout, "scratch-rotate: %s %d %s older than 1d from %s\n", r.pruned, r.prune(dir, 1, false, nil), m.what, dir)
		}
	}
	r.registry(paths.ScratchRegistry(), n, stderr)
	return 0
}

// registry prunes every project scratch dir the registry lists by age
// alone (project scratch holds test artifacts and POC files of any
// extension), dropping entries whose directory is gone.
func (r *rotator) registry(registry string, n int, stderr io.Writer) {
	data, err := os.ReadFile(registry)
	if err != nil {
		return
	}
	dropping := "dropping"
	if r.dryRun {
		dropping = "would drop"
	}
	home := os.Getenv("HOME")
	var keep []string
	for dir := range strings.SplitSeq(string(data), "\n") {
		switch {
		case dir == "":
		case !fsx.IsDir(dir):
			fmt.Fprintf(r.stdout, "scratch-rotate: %s stale registry entry %s (directory no longer exists)\n", dropping, dir)
		case !validScratchDir(dir, home):
			fmt.Fprintf(stderr, "scratch-rotate: REFUSING registry entry %s (not a plain scratch/ dir under $HOME)\n", dir)
			keep = append(keep, dir)
		default:
			r.worktrees(dir, stderr)
			fmt.Fprintf(r.stdout, "scratch-rotate: %s %d artifact(s) older than %dd from %s\n", r.pruned, r.prune(dir, n, false, nil), n, dir)
			keep = append(keep, dir)
		}
	}
	if !r.dryRun {
		writeLines(registry, keep)
	}
}

type rotator struct {
	now    time.Time
	dryRun bool
	verb   string // the rotate log's word for a delete
	pruned string // the output's
	log    string
	stdout io.Writer
}

// worktrees removes the review-* git worktrees directly under a project
// scratch dir whose creation (their .git file) is more than a day old: a
// review that ended without cleaning up leaves one behind.
func (r *rotator) worktrees(dir string, stderr io.Writer) {
	gitFiles, _ := filepath.Glob(filepath.Join(dir, "review-*", ".git"))
	var removed []string
	for _, gitFile := range gitFiles {
		info, err := os.Lstat(gitFile)
		if err != nil || !info.Mode().IsRegular() || !r.older(info, 1) {
			continue
		}
		wt := filepath.Dir(gitFile)
		if why := keepWorktree(wt); why != "" {
			fmt.Fprintf(r.stdout, "scratch-rotate: kept %s %s\n", why, wt)
			continue
		}
		if !r.dryRun {
			if out, err := git.Command(wt, "worktree", "remove", "--force", wt).CombinedOutput(); err != nil {
				fmt.Fprintf(stderr, "scratch-rotate: can't remove review worktree %s: %s\n", wt, strings.TrimSpace(string(out)))
				continue
			}
		}
		fmt.Fprintf(r.stdout, "scratch-rotate: %s review worktree older than 1d %s\n", r.verb, wt)
		removed = append(removed, wt)
	}
	r.record(removed)
}

// keepWorktree is why a review worktree must stay, or "": git refuses to
// remove a locked one, and --force would drop uncommitted work.
func keepWorktree(wt string) string {
	gitDir, err := git.Line(wt, "rev-parse", "--absolute-git-dir")
	changes, statusErr := git.Status(wt, false)
	switch {
	case err != nil || statusErr != nil:
		return "unreadable review worktree"
	case fsx.IsFile(filepath.Join(gitDir, "locked")):
		return "locked review worktree"
	case len(changes) > 0:
		return "review worktree with uncommitted changes"
	}
	return ""
}

// older reports whether info's modification is more than days days old.
func (r *rotator) older(info fs.FileInfo, days int) bool {
	return r.now.Sub(info.ModTime()) > time.Duration(days)*24*time.Hour
}

// prune deletes the regular files under dir (only dir itself with
// topOnly) matching keep whose modification is more than days whole days
// old, and records each in the rotate log. It never enters a nested git
// checkout: deleting its old files would corrupt it.
func (r *rotator) prune(dir string, days int, topOnly bool, keep func(string) bool) int {
	var matched []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && (topOnly || isCheckout(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || (keep != nil && !keep(d.Name())) {
			return nil
		}
		info, err := d.Info()
		if err != nil || !r.older(info, days) {
			return nil
		}
		if !r.dryRun && os.Remove(path) != nil {
			return nil
		}
		matched = append(matched, path)
		return nil
	})
	r.record(matched)
	return len(matched)
}

// record appends each deleted path to the rotate log.
func (r *rotator) record(paths []string) {
	if len(paths) == 0 {
		return
	}
	prefix := r.now.UTC().Format(kitlog.TimeLayout) + " " + r.verb + " "
	var lines strings.Builder
	for _, p := range paths {
		lines.WriteString(prefix + p + "\n")
	}
	_ = fsx.Append(r.log, []byte(lines.String()))
}

// isCheckout reports whether dir is a git checkout: a repo or a worktree.
func isCheckout(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil
}

// validScratchDir treats a registry line as untrusted: absolute, no "..",
// under home, named exactly scratch, a real directory and not a symlink,
// and not a repository.
func validScratchDir(dir, home string) bool {
	if !strings.HasPrefix(dir, "/") || strings.Contains(dir, "..") || home == "" || !strings.HasPrefix(dir, home+"/") || filepath.Base(dir) != "scratch" {
		return false
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return false
	}
	_, err = os.Lstat(filepath.Join(dir, ".git"))
	return os.IsNotExist(err)
}

// writeLines replaces path atomically with lines, one per line.
func writeLines(path string, lines []string) {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	_ = fsx.WriteAtomic(path, []byte(b.String()), 0o600)
}
