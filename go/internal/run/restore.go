package run

import (
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/git"
)

// refPrefix holds the restore points: each a tree of the work tree as a
// full-gate run found it, named by when, in nanoseconds.
const refPrefix = "refs/kit/gates/"

// keepDays is how long a restore point lives.
const keepDays = 30

// keep stores tree as a restore point in root's repo; "" when git can't.
func keep(root, tree string, now time.Time) string {
	ref := refPrefix + strconv.FormatInt(now.UnixNano(), 10)
	if _, err := git.Output(root, "update-ref", ref, tree); err != nil {
		return ""
	}
	return ref
}

// PruneRestorePoints deletes root's restore points older than keepDays.
func PruneRestorePoints(root string, now time.Time) {
	refs, _ := git.Lines(root, "for-each-ref", "--format=%(refname)", refPrefix)
	for _, ref := range refs {
		nanos, err := strconv.ParseInt(strings.TrimPrefix(ref, refPrefix), 10, 64)
		if err == nil && now.Sub(time.Unix(0, nanos)) > keepDays*24*time.Hour {
			_, _ = git.Output(root, "update-ref", "-d", ref)
		}
	}
}

// Restore puts m's paths back in root's work tree as its restore point
// holds them, the index untouched; a path it doesn't hold (a file the gate
// created) is left. It returns the paths it restored.
func Restore(root string, m enforce.Mark) ([]string, error) {
	held, err := git.Paths(root, append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--name-only", m.Ref, "--"}, m.Paths...)...)
	if err != nil || len(held) == 0 {
		return nil, err
	}
	_, err = git.Output(root, append([]string{"--literal-pathspecs", "restore", "--source=" + m.Ref, "--worktree", "--overlay", "--"}, held...)...)
	return held, err
}
