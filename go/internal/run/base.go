package run

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// Base is a gate's failure checked again on the git base: SHA's tree,
// unpacked once under CacheDir, run with the same command.
type Base struct {
	Root, SHA, CacheDir string
}

var (
	// failureLine is an output line that names a failure.
	failureLine = regexp.MustCompile(`(?i)\b(fail(ed|ure)?|error|panic)\b`)
	// timing is a duration a runner prints beside a result, different each run.
	timing = regexp.MustCompile(`\d+(\.\d+)?(ms|s)\b`)
)

// hasAll is whether out, g's output on the branch, names the same failure
// lines as g's output on the base, no more and no fewer: a shared summary
// line (make's "Error 1") alone would hide a new failure printed without
// a failure word. Output naming no failure never matches.
func (b *Base) hasAll(g enforce.Gate, out string, timeout time.Duration) bool {
	want := failureLines(out, b.Root)
	if len(want) == 0 {
		return false
	}
	baseOut, ok := b.output(g, timeout)
	if !ok {
		return false
	}
	return maps.Equal(want, failureLines(baseOut, b.dir()))
}

// failureLines is out's failure lines, with root and timings left out.
func failureLines(out, root string) map[string]bool {
	lines := map[string]bool{}
	for line := range strings.Lines(strings.ReplaceAll(out, root, "")) {
		if failureLine.MatchString(line) {
			lines[strings.TrimSpace(timing.ReplaceAllString(line, ""))] = true
		}
	}
	return lines
}

// dir is where SHA's tree is unpacked, under the cache.
func (b *Base) dir() string { return filepath.Join(b.CacheDir, "base", b.SHA) }

// output is g's output run in the base's tree, cached per SHA and command;
// false when the tree can't be unpacked or the run timed out.
func (b *Base) output(g enforce.Gate, timeout time.Duration) (string, bool) {
	body := strings.ReplaceAll(g.Command(), b.Root, b.dir())
	sum := sha256.Sum256([]byte(g.Dir + "\x00" + body))
	cached := b.dir() + "-" + hex.EncodeToString(sum[:8]) + ".out"
	if data, err := os.ReadFile(cached); err == nil {
		return string(data), true
	}
	if err := b.unpack(); err != nil {
		return "", false
	}
	at := g
	at.Dir = strings.Replace(g.Dir, b.Root, b.dir(), 1)
	o := execute(command(at, body, timeout))
	if o.timedOut {
		return "", false
	}
	_ = fsx.WriteAtomic(cached, []byte(o.out), 0o600)
	return o.out, true
}

// unpack writes SHA's tree to dir once, through git archive, so the repo
// gets no worktree of the kit's. The lock keeps a second run (a subagent's
// Stop beside the main session's) from deleting a tree mid-extract.
func (b *Base) unpack() error {
	done := b.dir() + ".done"
	if fsx.IsFile(done) {
		return nil
	}
	unlock, err := cache.Lock(context.Background(), b.dir()+".lock")
	if err != nil {
		return err
	}
	defer unlock()
	if fsx.IsFile(done) {
		return nil
	}
	if err := os.RemoveAll(b.dir()); err != nil {
		return err
	}
	archive, err := git.Output(b.Root, "archive", "--format=tar", b.SHA)
	if err != nil {
		return err
	}
	if err := untar(tar.NewReader(strings.NewReader(archive)), b.dir()); err != nil {
		return err
	}
	return fsx.WriteAtomic(done, nil, 0o600)
}

// throughLink is whether target, path's link, climbs with .. after a step
// through another of links: the lexical check above reads y/../x as x,
// while the system follows y first.
func throughLink(path, target string, links map[string]string) bool {
	parts := strings.Split(target, "/")
	if !slices.Contains(parts, "..") {
		return false
	}
	cur := filepath.Dir(path)
	for _, part := range parts[:len(parts)-1] {
		cur = filepath.Join(cur, part)
		if _, ok := links[cur]; ok {
			return true
		}
	}
	return false
}

// untar writes r's files and links under dir. Links come last, so no file
// is written through one, and a name or link target leaving dir is an
// error: the base commit may come from anyone.
func untar(r *tar.Reader, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	links := map[string]string{}
	for {
		hdr, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if err := untarEntry(r, hdr, dir, links); err != nil {
			return err
		}
	}
	for path, target := range links {
		if throughLink(path, target, links) {
			return errors.New("run: base archive links outside its tree through another link: " + path)
		}
	}
	for path, target := range links {
		if err := os.Symlink(target, path); err != nil {
			return err
		}
	}
	return nil
}

// untarEntry writes hdr's directory or file under dir, or records its link
// in links for untar to write last.
func untarEntry(r *tar.Reader, hdr *tar.Header, dir string, links map[string]string) error {
	// IsLocal in place of fsx.Within: CodeQL's Zip Slip query sees it as a guard.
	if !filepath.IsLocal(hdr.Name) {
		return errors.New("run: base archive names a path outside its tree: " + hdr.Name)
	}
	path := filepath.Join(dir, hdr.Name)
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(path, 0o755)
	case tar.TypeSymlink:
		if filepath.IsAbs(hdr.Linkname) || !fsx.Within(dir, filepath.Join(filepath.Dir(path), hdr.Linkname)) {
			return errors.New("run: base archive links outside its tree: " + hdr.Name)
		}
		links[path] = hdr.Linkname
	case tar.TypeReg:
		return writeFile(path, r, hdr.FileInfo().Mode().Perm())
	}
	return nil
}

// writeFile is path written from r with perm, its directory made first.
func writeFile(path string, r io.Reader, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	return errors.Join(err, f.Close())
}
