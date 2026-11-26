package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/enforce"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/kitlog"
	"github.com/ku5ic/claude-kit/go/internal/layout"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/run"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

// StopChecks is the Stop hook: when the turn created or edited files, through
// an edit tool or, by the turn-start snapshot, any other way, it runs the
// project's file-scoped checks (enforce.Stop) on just those files and blocks
// the stop on a failure, so Claude fixes or reports it. A question-only turn costs
// nothing. When the turn ticked a plan's last open step, it also blocks
// until /code-review has run since the last code
// edit; that review's hook runs the full gate. A tree the full gate already
// checked isn't checked again, nor are the same checks on a tree where they
// all passed.
func StopChecks(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	// Already continuing because of this hook: one retry per failing check,
	// so one that can't be fixed never loops. A done plan's block holds.
	retry := h.Payload.Bool(hook.KeyStopHookActive)
	deadline := time.Now().Add(stopBudget)
	cwd := h.Payload.Cwd()
	path := h.Payload.String(hook.KeyTranscriptPath)
	if !fsx.IsFile(path) {
		return nil
	}
	root := git.Toplevel(cwd) // "" outside a work tree
	cfg := h.Config()
	if root == "" || cfg == nil {
		return nil
	}
	turn, walk := readSession(h, path, root, project.PlanDir(h.Paths, cwd), codeIn(h, root), true)
	// Before the clean-tree return: plans are gitignored, so ticking the
	// last step leaves the tree clean. A finished plan waits for
	// /code-review since the last code edit (skills/kit-rules/reference/verify.md).
	// On a retry it holds only until a review has launched: the edits
	// since one may be its fixes, which need no second review. A session
	// that can't launch one stops on the second retry, told so.
	held := h.Paths.SessionFile(cache.ReviewReport, h.Payload.SessionID(), "plan-done")
	if plan, reviewed := walk.done(); plan != "" && !reviewed && (!retry || !walk.launched()) {
		if retry && held != "" && fsx.IsFile(held) {
			_ = os.Remove(held)
			h.Notify(fmt.Sprintf("kit plan-done: let through, plan %s is done with no /code-review since the last code edit", fsx.Rel(root, plan)))
			return nil
		}
		if held != "" && !h.DryRun {
			if retry {
				_ = fsx.WriteAtomic(held, nil, 0o600)
			} else {
				_ = os.Remove(held)
			}
		}
		return h.Block(fmt.Sprintf("plan %s %s\n", fsx.Rel(root, plan), planDoneHeld)+
			layout.KV("do", "run it now, or wait for the one running")+"\n"+
			layout.KV("then", "the runtime pass (/verify) when the change has observable behavior")+"\n"+
			layout.KV("or", "the edits since are only fixes for that review's findings: say so in one line, no second review (skills/kit-rules/reference/verify.md section 5)"), "plan-done")
	}
	if retry {
		return nil
	}
	edited, tree := turnFiles(h, turn, cwd, root)
	if len(edited) == 0 {
		return nil
	}
	if tree == "" {
		tree, _ = git.Snapshot(root)
	}
	reviewSheets(h, cfg, cwd, root, tree)
	if coveredByFullGate(h, root, tree) {
		if !cfg.ReceiptOnFailures() {
			h.Notify("kit stop: skipped, the full gate already checked this tree")
		}
		return nil
	}
	// A check changed mid-session gets its verdict now, waiting on a
	// background classify that's already asking, instead of skipping.
	p := enforce.Stop(cfg, enforce.Options{Root: root, CacheDir: h.Paths.CacheDir(), Home: h.Home, Ask: true, Timeout: gapfill.ClassifyTimeout}, edited)
	if p.Classifier.Runs > 0 {
		h.Log(kitlog.Guards, "gapfill", p.Classifier.Pairs()...)
	}
	if len(p.Gates) == 0 {
		return nil
	}
	return runStop(h, cfg, p, stopRun{root: root, cwd: cwd, tree: tree, deadline: deadline})
}

// stopRun is where and on what runStop runs the planned gates.
type stopRun struct {
	root, cwd, tree string
	deadline        time.Time
}

// runStop runs p's gates, unless they all passed on this tree before,
// shows the user every verdict, and blocks on a failure.
func runStop(h *hook.Hook, cfg *config.Config, p enforce.Plan, r stopRun) error {
	pass := passKey(r.tree, p.Gates)
	passFile := cache.StopPassTree(h.Paths.CacheDir(), r.root)
	if prev, err := os.ReadFile(passFile); err == nil && pass != "" && string(prev) == pass {
		if !cfg.ReceiptOnFailures() {
			h.Notify("kit stop: skipped, these checks already passed on this tree")
		}
		return nil
	}
	// A failure the git base has too isn't the branch's to fix. The base
	// is looked up only once a gate fails.
	base := sync.OnceValue(func() *run.Base {
		if sha, ok := git.ForkPoint(r.root); ok {
			return &run.Base{Root: r.root, SHA: sha, CacheDir: h.Paths.CacheDir()}
		}
		return nil
	})
	out := run.Files(p, base, h.Paths.CacheDir(), time.Duration(cfg.GateTimeout)*time.Second, r.deadline)
	if !h.DryRun {
		_ = fsx.WriteAtomic(cache.StopReport(h.Paths.CacheDir(), r.root), []byte(out.Report()), 0o600)
	}
	// The user gets the counts, the passes, and a line naming each failure
	// with its detail file; what to fix goes to Claude in the block's reason.
	file := ""
	if out.Failed() {
		file = stopDetail(h, r.cwd, out.Detail)
	}
	notifyReceipt(h, cfg, out.Receipt, file)
	if !out.Failed() {
		// A gate skipped unchecked checked nothing, so only a whole pass is reused.
		if out.Result.Unchecked == 0 && pass != "" && !h.DryRun {
			_ = fsx.WriteAtomic(passFile, []byte(pass), 0o600)
		}
		return nil
	}
	msg := fmt.Sprintf("%d of %d checks failed\n%s", out.Result.Fail, len(p.Gates), out.Failures)
	if file != "" {
		msg += layout.KV("more", file) + "\n"
	}
	msg += layout.KV("do", "fix them, or reply in one line with the cause")
	return h.Block(msg, "checks-failed")
}

// notifyReceipt shows the user rc, then unchecked and the overrides in
// force. kit.yml's receipt: failures hides a clean pass with nothing
// unchecked; the gates ran either way. A failure means the caller blocks,
// and the block names each failure and skip with its detail, so the
// receipt keeps the counts and passes, plus one line naming the failures
// and detail, the file holding their output.
func notifyReceipt(h *hook.Hook, cfg *config.Config, rc run.Receipt, detail string) {
	if cfg.ReceiptOnFailures() && rc.Clean() {
		return
	}
	var extra []string
	if !rc.Passed() {
		if failed := rc.Failures(); len(failed) > 0 {
			line := strings.Join(failed, ", ")
			if detail != "" {
				line += "  -> " + detail
			}
			extra = append(extra, layout.KV("FAIL", line))
		}
		rc = rc.PassesOnly()
	}
	h.Notify(rc.String(append(extra, overrides(cfg)...)...))
}

// overrides are the kit.yml switches in force that turn checks or rules off.
func overrides(cfg *config.Config) []string {
	var lines []string
	if len(cfg.DisabledChecks) > 0 {
		lines = append(lines, layout.KV("override", "disabled_checks: "+strings.Join(cfg.DisabledChecks, ", ")))
	}
	if len(cfg.DisabledRules) > 0 {
		lines = append(lines, layout.KV("override", "disabled_rules: "+strings.Join(cfg.DisabledRules, ", ")))
	}
	return lines
}

// stopDetail writes a failing Stop's full detail (each gate's command,
// directory, binaries, and output) to scratch, and returns the path; ""
// when it can't.
func stopDetail(h *hook.Hook, cwd, detail string) string {
	dir, err := project.ScratchDir(h.Paths, cwd, !h.DryRun)
	if err != nil || h.DryRun {
		return ""
	}
	file := filepath.Join(dir, "kit-stop-failures.txt")
	if fsx.WriteAtomic(file, []byte(detail), 0o600) != nil {
		return ""
	}
	return file
}

// coveredByFullGate is true when review-checks' last full gate checked
// tree, the work tree as it is now ("" when it can't be read). Once the
// tree moves on, the record goes.
func coveredByFullGate(h *hook.Hook, root, tree string) bool {
	file := cache.FullGateTree(h.Paths.CacheDir(), root)
	checked, err := os.ReadFile(file)
	if err != nil || tree == "" {
		return false
	}
	if tree != string(checked) && !h.DryRun {
		_ = os.Remove(file)
	}
	return tree == string(checked)
}

// passKey names a Stop run by the tree it checked and its gates, which
// carry the files and commands: a later Stop with the same key would run
// the same checks on the same content. "" when the tree is unknown.
func passKey(tree string, gates []enforce.Gate) string {
	data, err := json.Marshal(gates)
	if tree == "" || err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return tree + " " + hex.EncodeToString(sum[:])
}

// stopBudget bounds a Stop run, planning and every gate, inside
// stop-dispatch's 300s in hooks.json. The 60s left covers chat-claims ahead
// of it (claims' settleBudget), the snapshot and diffs after the gates (git
// calls of up to git.Timeout each), and the report.
const stopBudget = 240 * time.Second

// stopReadVersion is stopRead's shape. Bump it when stopRead or planWalk
// gains or changes a field: a saved read of another version is dropped and
// the transcript read whole, so no field needs a zero value that suits an
// old walk.
const stopReadVersion = 4

// stopRead is where the last Stop run left the session's transcript: the
// offset and line past what it read, where the current turn starts, and
// the plan walk so far.
type stopRead struct {
	Version    int       `json:"version"`
	Path       string    `json:"path"`
	Offset     int64     `json:"offset"`
	Line       int       `json:"line"`
	TurnOffset int64     `json:"turn_offset"`
	TurnLine   int       `json:"turn_line"`
	Walk       *planWalk `json:"walk"`
}

// turnFiles is every file the turn changed, absolute, and the turn-end
// tree when the turn-start snapshot gave one; none when nothing changed.
// The snapshot also catches a Bash or subagent edit, and a file committed
// mid-turn. A turn whose every write is named leaves the rest to another
// session or the user, so the snapshot adds nothing. Without one, a clean
// tree means the edits were committed.
func turnFiles(h *hook.Hook, turn []transcript.Entry, cwd, root string) (edited []string, tree string) {
	edited = transcript.Edits(turn)
	for i, path := range edited {
		edited[i] = fsx.Abs(cwd, path)
	}
	changed, tree, ok := turnChanges(h, root)
	if ok {
		if !transcript.NamesEveryWrite(turn) {
			// A file a git move brought in is someone else's, until a later
			// command changes it from HEAD.
			moved := fromHead(root, movedFiles(h, root))
			changed = slices.DeleteFunc(changed, func(path string) bool { return slices.Contains(moved, path) })
			edited = append(edited, changed...) // enforce.Stop drops repeats
		}
	} else if changes, err := git.Status(cwd, false); err != nil || len(changes) == 0 {
		return nil, ""
	}
	return edited, tree
}

// codeIn is whether a path is code under root: inside it, outside scratch.
func codeIn(h *hook.Hook, root string) func(string) bool {
	return func(path string) bool {
		return fsx.Within(root, fsx.PhysicalPath(path)) && !project.IsScratch(h.Paths, path)
	}
}

// readSession is the transcript's plan walk, and with wantTurn its current
// turn, reading only what the last run hadn't: the walk from where it
// stopped, and the turn, when wanted, from its start. A caller that drops
// the turn reads from the walk's offset, so an edit mid-turn doesn't decode
// the whole turn again. A different, shorter, or unrecorded transcript is
// read whole. A read error returns what was read and saves nothing: the
// hook fails open.
func readSession(h *hook.Hook, path, root, plansDir string, isCode func(string) bool, wantTurn bool) ([]transcript.Entry, *planWalk) {
	file := h.Paths.SessionFile(cache.StopRead, h.Payload.SessionID(), cache.RootKey(root))
	var st stopRead
	info, statErr := os.Stat(path)
	if data, err := os.ReadFile(file); err != nil || statErr != nil || json.Unmarshal(data, &st) != nil || st.Version != stopReadVersion || st.Path != path || st.Walk == nil || info.Size() < st.Offset {
		st = stopRead{Version: stopReadVersion, Path: path, Walk: newPlanWalk()}
	}
	st.Walk.bashed = bashEdits(h)
	var turn []transcript.Entry
	seen := st.Offset
	from, fromLine := st.Offset, st.Line
	if wantTurn {
		from, fromLine = st.TurnOffset, st.TurnLine
	}
	var err error
	st.Offset, st.Line, err = transcript.EachFrom(path, from, fromLine, func(e transcript.Entry, end int64) {
		if end > seen {
			st.Walk.step(h, e, plansDir, isCode)
		}
		if e.StartsTurn() {
			turn, st.TurnOffset, st.TurnLine = nil, end, e.Line
			return
		}
		turn = append(turn, e)
	})
	// A failed read keeps the saved state: its offset fell back to the
	// turn's start, and the walk didn't see what lies past it.
	if file != "" && !h.DryRun && err == nil {
		if data, err := json.Marshal(st); err == nil {
			_ = fsx.WriteAtomic(file, data, 0o600)
		}
	}
	return turn, st.Walk
}
