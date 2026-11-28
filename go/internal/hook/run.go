package hook

import (
	"errors"
	"fmt"
	"io"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
)

// FailOpen prints the notice for a hook that allows because cause, an
// error or a panic's value, made it fail.
func FailOpen(stderr io.Writer, name string, cause any) {
	fmt.Fprintf(stderr, "%s: unexpected error, failing open: %v\n", name, cause)
}

// Check is one policy check. It returns *Blocked to block, nil to allow, and
// any other error to fail open.
type Check func(*Hook) error

// RunCheck runs check under name, failing open on a panic or error: the
// notice goes to stderr, which only the debug log sees, so it is also logged
// to guards.jsonl, and the result is "allow". It returns the check's block,
// nil when it allowed.
func RunCheck(h *Hook, name string, check Check) (blocked *Blocked) {
	saved := h.Name
	h.Name = name
	defer func() {
		if r := recover(); r != nil {
			h.failedOpen(name, r)
			blocked = nil
		}
		h.Name = saved
	}()
	err := check(h)
	if err == nil {
		return nil
	}
	if b, ok := errors.AsType[*Blocked](err); ok {
		return b
	}
	h.failedOpen(name, err)
	return nil
}

// failedOpen reports a check that let the call through because cause made
// it fail: to stderr and guards.jsonl each time, and to the user once a
// session, so a guard that stopped working doesn't go unseen. With no
// session to mark, the user hears of each one.
func (h *Hook) failedOpen(name string, cause any) {
	FailOpen(h.Stderr, name, cause)
	h.Log(kitlog.Guards, "fail-open", "error", fmt.Sprint(cause))
	var marker string
	if h.Payload != nil {
		marker = h.Paths.SessionFile(cache.FailOpen, h.Payload.SessionID())
	}
	if marker != "" && fsx.IsFile(marker) {
		return
	}
	notice := fmt.Sprintf("kit %s: FAILED OPEN, the call went through unchecked: %v", name, cause)
	if marker != "" {
		notice += "\nlater ones this session go to " + h.Paths.LogFile(kitlog.Guards) + " only"
		if !h.DryRun {
			_ = fsx.WriteAtomic(marker, nil, 0o600)
		}
	}
	h.Notify(notice)
}

// Run executes checks in order against one payload read, the dispatcher
// contract: each check is isolated (one failing open never skips the next),
// and the first block ends the run with exit 2. A block after a notice is a
// JSON decision instead, since exit 2 shows Claude the reason and drops the
// notice meant for the user.
func Run(h *Hook, checks ...NamedCheck) int {
	for _, c := range checks {
		b := RunCheck(h, c.Name, c.Check)
		if b == nil {
			continue
		}
		if h.notices == nil {
			fmt.Fprintln(h.Stderr, b.Reason)
			return 2
		}
		h.flush(b.Reason)
		return 0
	}
	h.flush("")
	return 0
}

// NamedCheck pairs a check with the hook name it reports under.
type NamedCheck struct {
	Name  string
	Check Check
}
