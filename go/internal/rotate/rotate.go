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
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// Run is `kit scratch-rotate [days] [--dry-run]`; it returns the exit status.
// Registry entries must sit under home.
func Run(paths config.Paths, home string, n int, dryRun bool, stdout, stderr io.Writer) int {
	r := newRotator(paths, dryRun, stdout, stderr)
	r.home(paths.ScratchHome(), n)
	for _, d := range cache.Dirs {
		if dir := filepath.Join(paths.CacheDir(), d.Name); fsx.IsDir(dir) {
			files, _, _ := r.prune(dir, d.Days, nil)
			fmt.Fprintf(stdout, "scratch-rotate: %s %d %s older than %dd from %s\n", r.pruned, len(files), d.What, d.Days, dir)
		}
	}
	r.registry(context.Background(), paths.ScratchRegistry(), home, n)
	return 0
}

func newRotator(paths config.Paths, dryRun bool, stdout, stderr io.Writer) *rotator {
	r := &rotator{now: time.Now(), dryRun: dryRun, log: rotateLog(paths), verb: "deleted", pruned: "pruned", stdout: stdout, stderr: stderr}
	if dryRun {
		r.verb, r.pruned = "would-delete", "would prune"
	}
	return r
}

// home prunes the scratch home's files older than n days, and the folders
// that leaves empty.
func (r *rotator) home(scratch string, n int) {
	if fsx.IsDir(scratch) {
		files, checkouts, folders := r.prune(scratch, n, nil)
		r.emptyFolders(folders, n, slices.Concat(files, checkouts))
		fmt.Fprintf(r.stdout, "scratch-rotate: %s %d artifact(s) older than %dd from %s\n", r.pruned, len(files), n, scratch)
	}
}

func rotateLog(paths config.Paths) string { return filepath.Join(paths.LogDir(), "scratch-rotate.log") }

func dailyStamp(paths config.Paths) string {
	return filepath.Join(paths.CacheDir(), "scratch-rotate.stamp")
}

// DefaultDays is the retention window when none is given.
const DefaultDays = 30

// Daily prunes the scratch home and every registered project's scratch at
// most once a day, past DefaultDays, printing nothing; the rotate log
// lists what went. The cache dirs are cache.Prune's, run every session.
func Daily(paths config.Paths, home string, now time.Time) {
	stamp := dailyStamp(paths)
	if !due(stamp, now) {
		return
	}
	// Stamped first: a walk the hook's timeout kills waits a day, not a session.
	_ = os.MkdirAll(paths.CacheDir(), 0o755)
	_ = os.WriteFile(stamp, []byte(now.UTC().Format(time.RFC3339)+"\n"), 0o644)
	r := newRotator(paths, false, io.Discard, io.Discard)
	r.home(paths.ScratchHome(), DefaultDays)
	r.registry(context.Background(), paths.ScratchRegistry(), home, DefaultDays)
}

// due is whether the run stamp records is more than a day before now.
func due(stamp string, now time.Time) bool {
	data, err := os.ReadFile(stamp)
	if err != nil {
		return true
	}
	last, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	return err != nil || last.After(now) || cache.Older(last, 1, now)
}

// registry prunes every project scratch dir the registry lists by age
// alone (project scratch holds test artifacts and POC files of any
// extension), dropping entries whose directory is gone.
func (r *rotator) registry(ctx context.Context, registry, home string, n int) {
	data, err := os.ReadFile(registry)
	if err != nil {
		return
	}
	dropping := "dropping"
	if r.dryRun {
		dropping = "would drop"
	}
	var keep, stale []string
	for dir := range strings.SplitSeq(string(data), "\n") {
		switch {
		case dir == "":
		case !fsx.IsDir(dir):
			stale = append(stale, dir)
		case !validScratchDir(dir, home):
			fmt.Fprintf(r.stderr, "scratch-rotate: REFUSING registry entry %s (not a plain scratch/ dir under $HOME)\n", dir)
			keep = append(keep, dir)
		default:
			files, checkouts, folders := r.prune(dir, n, nil)
			r.emptyFolders(folders, n, slices.Concat(files, checkouts))
			fmt.Fprintf(r.stdout, "scratch-rotate: %s %d artifact(s) older than %dd from %s\n", r.pruned, len(files), n, dir)
			keep = append(keep, dir)
		}
	}
	dropped := func() {
		for _, dir := range stale {
			fmt.Fprintf(r.stdout, "scratch-rotate: %s stale registry entry %s (directory no longer exists)\n", dropping, dir)
		}
	}
	if r.dryRun {
		dropped()
		return
	}
	unlock, err := lockRegistry(ctx, registry)
	if err != nil {
		fmt.Fprintf(r.stderr, "scratch-rotate: left %s as it was, the next run rewrites it: %v\n", registry, err)
		return
	}
	defer unlock()
	now, err := os.ReadFile(registry)
	if err == nil {
		err = writeLines(registry, withAddedSince(string(data), string(now), keep))
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "scratch-rotate: can't rewrite %s: %v\n", registry, err)
		return
	}
	dropped()
}

type rotator struct {
	now    time.Time
	dryRun bool
	verb   string // the rotate log's word for a delete
	pruned string // the output's
	log    string
	stdout io.Writer
	stderr io.Writer
}

// removeWorktree deletes a linked worktree in scratch whatever it holds:
// scratch is throwaway. git removes it, a second --force taking a locked
// one, so its repo forgets it; when git can't, the folder goes anyway.
func (r *rotator) removeWorktree(wt string) error {
	if r.dryRun {
		return nil
	}
	if git.Command(wt, "worktree", "remove", "--force", "--force", wt).Run() == nil {
		return nil
	}
	return os.RemoveAll(wt)
}

// folder is a folder under a pruned dir and its mtime before the prune:
// deleting a file inside sets it to now.
type folder struct {
	path string
	mod  time.Time
}

// prune deletes the regular files and symlinks under dir matching keep
// (every one when nil) whose modification is more than days whole days
// old, records each in the rotate log, and returns them with the folders
// under dir, parents before children. It never deletes part of a nested
// git checkout, which would corrupt it: with keep nil, removeCheckout
// takes one whole once its newest entry is past checkoutDays, and it's
// returned in checkouts, not files.
func (r *rotator) prune(dir string, days int, keep func(string) bool) (files, checkouts []string, folders []folder) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return nil
		}
		if d.IsDir() {
			if isCheckout(path) {
				if keep == nil && r.removeCheckout(path, checkoutDays(dir, path, days)) {
					checkouts = append(checkouts, path)
				}
				return filepath.SkipDir
			}
			if info, err := d.Info(); err == nil {
				folders = append(folders, folder{path, info.ModTime()})
			}
			return nil
		}
		if !d.Type().IsRegular() && d.Type()&fs.ModeSymlink == 0 || (keep != nil && !keep(d.Name())) {
			return nil
		}
		info, err := d.Info()
		if err != nil || !cache.Older(info.ModTime(), days, r.now) {
			return nil
		}
		if !r.dryRun && os.Remove(path) != nil {
			return nil
		}
		files = append(files, path)
		return nil
	})
	r.record(files)
	return files, checkouts, folders
}

// reviewDays is the most a review-* checkout directly under a pruned dir
// keeps: a review that ended without cleaning up leaves one behind.
const reviewDays = 1

// checkoutDays is the window for the checkout at path under dir: days, at
// most reviewDays for a review-* one directly under dir.
func checkoutDays(dir, path string, days int) int {
	if filepath.Dir(path) == dir && strings.HasPrefix(filepath.Base(path), "review-") {
		return min(days, reviewDays)
	}
	return days
}

// removeCheckout deletes a nested checkout whose newest entry is more than
// days old, and reports whether it went (would go, on a dry run), with a
// line of its own naming its window, and in the rotate log. A linked
// worktree goes through removeWorktree; a repo of its own goes whole.
func (r *rotator) removeCheckout(path string, days int) bool {
	if changedWithin(path, days, r.now) {
		return false
	}
	if fsx.IsFile(filepath.Join(path, ".git")) {
		if err := r.removeWorktree(path); err != nil {
			fmt.Fprintf(r.stderr, "scratch-rotate: can't remove worktree %s: %v\n", path, err)
			return false
		}
	} else if !r.dryRun {
		if err := os.RemoveAll(path); err != nil {
			fmt.Fprintf(r.stderr, "scratch-rotate: can't remove checkout %s: %v\n", path, err)
			return false
		}
	}
	fmt.Fprintf(r.stdout, "scratch-rotate: %s checkout older than %dd %s\n", r.verb, days, path)
	r.record([]string{path})
	return true
}

// changedWithin is true when anything under dir, dir included, was modified
// within days of now, symlinks read as links. The walk stops at the first.
func changedWithin(dir string, days int, now time.Time) bool {
	changed := false
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, err := d.Info(); err == nil && !cache.Older(info.ModTime(), days, now) {
			changed = true
			return fs.SkipAll
		}
		return nil
	})
	return changed
}

// emptyFolders deletes, deepest first, each of folders past days by its
// mtime before the prune that holds nothing but what this run removed
// (would remove, on a dry run), and records each in the rotate log.
func (r *rotator) emptyFolders(folders []folder, days int, removed []string) {
	gone := map[string]bool{}
	for _, p := range removed {
		gone[p] = true
	}
	var deleted []string
	for _, f := range slices.Backward(folders) {
		if !cache.Older(f.mod, days, r.now) {
			continue
		}
		entries, err := os.ReadDir(f.path)
		if err != nil || slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return !gone[filepath.Join(f.path, e.Name())] }) {
			continue
		}
		// os.Remove refuses a folder that isn't empty: the last guard.
		if !r.dryRun && os.Remove(f.path) != nil {
			continue
		}
		gone[f.path] = true
		deleted = append(deleted, f.path)
	}
	r.record(deleted)
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
// and not a repository. Under home is judged with links resolved, so a
// link partway down can't lead out of it.
func validScratchDir(dir, home string) bool {
	if !strings.HasPrefix(dir, "/") || strings.Contains(dir, "..") || home == "" || !strings.HasPrefix(dir, home+"/") || filepath.Base(dir) != "scratch" {
		return false
	}
	real, err := filepath.EvalSymlinks(dir)
	realHome, homeErr := filepath.EvalSymlinks(home)
	if err != nil || homeErr != nil || !strings.HasPrefix(real, realHome+"/") {
		return false
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return false
	}
	_, err = os.Lstat(filepath.Join(dir, ".git"))
	return os.IsNotExist(err)
}

// Register adds a project scratch dir to the registry, once, so the
// cwd-less run can prune it. This package owns the registry: Register adds
// to it and registry rewrites it.
func Register(ctx context.Context, paths config.Paths, dir string) error {
	registry := paths.ScratchRegistry()
	unlock, err := lockRegistry(ctx, registry)
	if err != nil {
		return err
	}
	defer unlock()
	data, err := os.ReadFile(registry)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if slices.Contains(strings.Split(string(data), "\n"), dir) {
		return nil
	}
	return fsx.Append(registry, []byte(dir+"\n"))
}

// registryWait bounds how long a registry writer waits for another.
const registryWait = 2 * time.Second

// lockRegistry takes the lock every registry writer shares, waiting at
// most registryWait.
func lockRegistry(ctx context.Context, registry string) (unlock func(), err error) {
	ctx, cancel := context.WithTimeout(ctx, registryWait)
	defer cancel()
	return cache.Lock(ctx, registryLock(registry))
}

// registryLock is the lock file beside the registry.
func registryLock(registry string) string { return registry + ".lock" }

// writeLines replaces path atomically with lines, one per line.
func writeLines(path string, lines []string) error {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return fsx.WriteAtomic(path, []byte(b.String()), 0o600)
}

// Clean is `kit clean [--yes]`: it removes the kit's own state, for an
// uninstall, and with no --yes only lists it. Each path is named, never a
// whole folder the kit shares with Claude Code (~/.claude/cache, logs).
// Project scratch folders sit in their repos, so they're listed for the
// user to delete, never removed; --yes drops the registry that names them.
func Clean(paths config.Paths, yes bool, stdout io.Writer) int {
	owned := []string{paths.ScratchRegistry(), registryLock(paths.ScratchRegistry()), rotateLog(paths), paths.ConfigCache(), dailyStamp(paths), paths.ScratchHome()}
	for _, name := range kitlog.Logs {
		owned = append(owned, paths.LogFile(name))
	}
	for _, d := range cache.Dirs {
		owned = append(owned, filepath.Join(paths.CacheDir(), d.Name))
	}
	projects, _ := os.ReadFile(paths.ScratchRegistry())
	code, found := 0, 0
	for _, p := range owned {
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		found++
		if !yes {
			fmt.Fprintln(stdout, "would remove", p)
		} else if err := os.RemoveAll(p); err != nil {
			fmt.Fprintf(stdout, "clean: %v\n", err)
			code = 1
		} else {
			fmt.Fprintln(stdout, "removed", p)
		}
	}
	for line := range strings.Lines(string(projects)) {
		if line = strings.TrimSpace(line); line == "" || !fsx.IsDir(line) {
			continue
		}
		fmt.Fprintf(stdout, "left in its repo, delete it yourself: %s\n", line)
	}
	if found == 0 {
		fmt.Fprintln(stdout, "kit clean: nothing to remove")
	} else if !yes {
		fmt.Fprintln(stdout, "kit clean --yes removes them")
	}
	return code
}

// withAddedSince is keep plus the registry lines in after that before
// lacked: a project registered while the walk ran.
func withAddedSince(before, after string, keep []string) []string {
	old := strings.Fields(before)
	for line := range strings.Lines(after) {
		if line = strings.TrimSpace(line); line != "" && !slices.Contains(old, line) && !slices.Contains(keep, line) {
			keep = append(keep, line)
		}
	}
	return keep
}
