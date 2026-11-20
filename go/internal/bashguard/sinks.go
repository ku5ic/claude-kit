package bashguard

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/guard"
)

// networkSinks are commands that send what they read to another host.
var networkSinks = map[string]bool{"curl": true, "wget": true, "nc": true, "ncat": true, "netcat": true, "socat": true, "ssh": true}

// dirToNetwork asks before a command sends a whole directory to another
// host: rsync or scp to a host:path, or tar or zip piped into a network
// sink. Only a directory that is the working directory or holds it, or
// one with a credential file at its top, counts, so a deploy of dist/
// passes. A local copy (cp -r . /tmp/x) moves nothing off the machine.
func (st *state) dirToNetwork(seg Segment) {
	for ci, call := range seg.Calls {
		i := lead(call.Words)
		if i >= len(call.Words) {
			continue
		}
		name, args := baseName(call.Words[i].Value), values(call.Words[i+1:])
		var dir string
		switch {
		case name == "rsync" || name == "scp":
			ops := nonOptions(args)
			if len(ops) < 2 || !remote(ops[len(ops)-1]) {
				continue
			}
			dir = st.firstWholeDir(ops[:len(ops)-1])
		case name == "tar" && creates(args) || name == "zip":
			if !slices.ContainsFunc(seg.Calls[ci+1:], func(c Call) bool {
				j := lead(c.Words)
				return j < len(c.Words) && networkSinks[baseName(c.Words[j].Value)]
			}) {
				continue
			}
			dir = st.firstWholeDir(nonOptions(args))
		}
		if dir != "" {
			st.ask(fmt.Sprintf("%s sends %s, the project or a directory holding a credential file, to another host; confirm", name, dir))
		}
	}
}

// firstWholeDir is the first of ops naming a directory that is the
// working directory or holds it, or has a credential file at its top; ""
// when none does.
func (st *state) firstWholeDir(ops []string) string {
	for _, op := range ops {
		p := filepath.Clean(st.abs(unquote(op)))
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			continue
		}
		if rel, err := filepath.Rel(p, st.cwd); err == nil && !strings.HasPrefix(rel, "..") {
			return op
		}
		entries, _ := os.ReadDir(p)
		if slices.ContainsFunc(entries, func(e os.DirEntry) bool {
			return guard.IsSensitive(st.cfg, st.home, filepath.Join(p, e.Name()))
		}) {
			return op
		}
	}
	return ""
}

// remote is true for an rsync or scp operand on another host: host:path,
// user@host:path, host::module, or rsync://.
func remote(op string) bool {
	if strings.HasPrefix(op, "rsync://") {
		return true
	}
	host, _, ok := strings.Cut(op, ":")
	return ok && host != "" && !strings.Contains(host, "/")
}

// creates is true for a tar that writes an archive: -c, --create, or a c
// in its first word's letters (tar czf, tar -czf).
func creates(args []string) bool {
	for k, a := range args {
		switch {
		case a == "--create":
			return true
		case strings.HasPrefix(a, "--"):
		case strings.HasPrefix(a, "-") || k == 0:
			if strings.Contains(a, "c") {
				return true
			}
		}
	}
	return false
}
