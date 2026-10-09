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
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

var positive = regexp.MustCompile(`^[1-9][0-9]*$`)

// Run is `kit scratch-rotate [days] [--dry-run]`; it returns the exit status.
func Run(cfg *config.Config, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	dryRun, days := false, "30"
	for _, a := range args {
		if a == "--dry-run" {
			dryRun = true
		} else {
			days = a
		}
	}
	if !positive.MatchString(days) {
		fmt.Fprintf(stderr, "scratch-rotate: retention window must be a positive integer, got '%s'\n", days)
		return 2
	}
	n, _ := strconv.Atoi(days)
	home := os.Getenv("HOME")
	r := &rotator{now: time.Now(), dryRun: dryRun, log: filepath.Join(paths.LogDir(), "scratch-rotate.log")}
	pruned, dropping := "pruned", "dropping"
	r.verb = "deleted"
	if dryRun {
		r.verb, pruned, dropping = "would-delete", "would prune", "would drop"
	}
	_ = os.MkdirAll(paths.LogDir(), 0o755)

	scratch := paths.ScratchHome()
	if isDir(scratch) {
		// .md artifacts by the retention window; .injected-* session
		// markers after a day: they only dedupe within a session.
		removed := r.prune(scratch, n, false, func(name string) bool { return strings.HasSuffix(name, ".md") })
		markers := r.prune(scratch, 1, true, func(name string) bool { return strings.HasPrefix(name, ".injected-") })
		fmt.Fprintf(stdout, "scratch-rotate: %s %d artifact(s) older than %dd from %s\n", pruned, removed, n, scratch)
		fmt.Fprintf(stdout, "scratch-rotate: %s %d session marker(s) older than 1d from %s\n", pruned, markers, scratch)
	}

	loaded := filepath.Join(paths.CacheDir(), "skills-loaded")
	if isDir(loaded) {
		fmt.Fprintf(stdout, "scratch-rotate: %s %d skill-loaded marker(s) older than 1d from %s\n", pruned, r.prune(loaded, 1, false, nil), loaded)
	}
	fileSkills := filepath.Join(paths.CacheDir(), "file-skills")
	if isDir(fileSkills) {
		fmt.Fprintf(stdout, "scratch-rotate: %s %d file-skills cache(s) older than 1d from %s\n", pruned, r.prune(fileSkills, 1, false, nil), fileSkills)
	}

	registry := filepath.Join(paths.LogDir(), "scratch-registry.txt")
	if data, err := os.ReadFile(registry); err == nil {
		var keep []string
		for dir := range strings.SplitSeq(string(data), "\n") {
			switch {
			case dir == "":
			case !isDir(dir):
				fmt.Fprintf(stdout, "scratch-rotate: %s stale registry entry %s (directory no longer exists)\n", dropping, dir)
			case !validScratchDir(dir, home):
				fmt.Fprintf(stderr, "scratch-rotate: REFUSING registry entry %s (not a plain scratch/ dir under $HOME)\n", dir)
				keep = append(keep, dir)
			default:
				// Project scratch holds test artifacts and POC files of any
				// extension, so it prunes by age alone.
				fmt.Fprintf(stdout, "scratch-rotate: %s %d artifact(s) older than %dd from %s\n", pruned, r.prune(dir, n, false, nil), n, dir)
				keep = append(keep, dir)
			}
		}
		if !dryRun {
			writeLines(registry, keep)
		}
	}

	// Every JSONL log the kit writes is capped here; the hooks that append
	// to them never trim.
	logs, _ := filepath.Glob(filepath.Join(paths.LogDir(), "*.jsonl"))
	for _, log := range logs {
		if !isFile(log) {
			continue
		}
		name := filepath.Base(log)
		lines, err := readLines(log)
		if err != nil {
			// Trimming what was read so far would drop the unread rest.
			fmt.Fprintf(stdout, "scratch-rotate: %s not trimmed, unreadable: %v\n", name, err)
			continue
		}
		switch total := len(lines); {
		case total <= cfg.LogMaxLines:
			fmt.Fprintf(stdout, "scratch-rotate: %s has %d lines, no trim needed\n", name, total)
		case dryRun:
			fmt.Fprintf(stdout, "scratch-rotate: would trim %s from %d to %d lines\n", name, total, cfg.LogMaxLines)
		default:
			writeLines(log, lines[total-cfg.LogMaxLines:])
			fmt.Fprintf(stdout, "scratch-rotate: trimmed %s from %d to %d lines\n", name, total, cfg.LogMaxLines)
		}
	}
	return 0
}

type rotator struct {
	now    time.Time
	dryRun bool
	verb   string
	log    string
}

// prune deletes the regular files under dir (only dir itself with
// topOnly) matching keep whose modification is more than days whole days
// old, as find's -mtime +days, and records each in the rotate log.
func (r *rotator) prune(dir string, days int, topOnly bool, keep func(string) bool) int {
	var matched []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if topOnly && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || (keep != nil && !keep(d.Name())) {
			return nil
		}
		info, err := d.Info()
		if err != nil || int(r.now.Sub(info.ModTime())/(24*time.Hour)) <= days {
			return nil
		}
		if !r.dryRun && os.Remove(path) != nil {
			return nil
		}
		matched = append(matched, path)
		return nil
	})
	if len(matched) > 0 {
		if f, err := os.OpenFile(r.log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			prefix := r.now.UTC().Format("2006-01-02T15:04:05Z") + " " + r.verb + " "
			for _, m := range matched {
				fmt.Fprintln(f, prefix+m)
			}
			f.Close()
		}
	}
	return len(matched)
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

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	return lines, s.Err()
}

// writeLines replaces path atomically with lines, one per line.
func writeLines(path string, lines []string) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rotate-*")
	if err != nil {
		return
	}
	w := bufio.NewWriter(tmp)
	for _, l := range lines {
		// bufio errors are sticky: Flush below reports any of them.
		_, _ = w.WriteString(l)
		_ = w.WriteByte('\n')
	}
	// A short write must not replace the log with a truncated copy.
	if err := errors.Join(w.Flush(), tmp.Close()); err != nil {
		os.Remove(tmp.Name())
		return
	}
	if os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
	}
}
