package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/bashguard"
	"github.com/ku5ic/claude-kit/go/internal/cache"
	"github.com/ku5ic/claude-kit/go/internal/claims"
	"github.com/ku5ic/claude-kit/go/internal/claudecli"
	"github.com/ku5ic/claude-kit/go/internal/gapfill"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/hook"
	"github.com/ku5ic/claude-kit/go/internal/hooks"
)

// hookDef is one `kit hook` name: the hooks.json events that run it (none
// for a check run only inside a dispatcher, or by hand), and the checks it
// runs in order against one payload read, or its own run for a dispatcher
// with a rule of its own.
type hookDef struct {
	events []string
	steps  []string
	run    func(*hook.Hook) int
}

// hookChecks are the single checks, by the name they report under.
var hookChecks = map[string]hook.Check{
	"plan-mode-context":       hooks.PlanModeContext,
	"guard-plan":              hooks.GuardPlan,
	"rules-gate":              hooks.RulesGate,
	"guard-edit":              hooks.GuardEdit,
	"guard-skills":            hooks.GuardSkills,
	"skills-nudge":            skillsNudge,
	"guard-bash-writes":       onBashWrites(hooks.GuardWrites),
	"bash-baseline":           hooks.BashBaseline,
	"bash-post":               bashPost,
	"bash-reports":            onBashWrites(hooks.CheckBashReports),
	"adr-nudge":               hooks.ADRNudge,
	"guard-slop":              hooks.GuardSlop,
	"guard-weaken":            hooks.GuardWeaken,
	"guard-imports":           hooks.GuardImports,
	"guard-tokens":            hooks.GuardTokens,
	"guard-handback":          hooks.GuardHandback,
	"guard-connector":         hooks.GuardConnector,
	"verify-claims":           hooks.VerifyClaims,
	"check-citations":         hooks.CheckCitations,
	"check-size":              hooks.CheckSize,
	"subagent-slop":           hooks.SubagentSlop,
	"chat-slop":               hooks.ChatSlop,
	"log-skills":              hooks.LogSkills,
	"sanitize-output":         hooks.SanitizeOutput,
	"inject-context":          hooks.InjectContext,
	"inject-subagent-context": hooks.InjectSubagentContext,
	"format-dispatch":         hooks.FormatDispatch,
	"plan-tick":               hooks.PlanTick,
	"plan-changes":            hooks.PlanChanges,
	"stop-checks":             hooks.StopChecks,
	"turn-tree":               hooks.TurnTree,
	"bash-edits":              onBashWrites(hooks.BashEdits),
	"edit-seen":               hooks.MarkEditSeen,
	"review-checks":           hooks.ReviewChecks,
	"reply-length":            hooks.ReplyLength,
	"review-report":           hooks.ReviewReport,
}

// registry is every `kit hook` name. A test holds it and hooks/hooks.json
// to each other.
var registry = map[string]hookDef{
	"bash-dispatch":      {events: []string{"PreToolUse"}, steps: bashSteps()},
	"guard-dispatch":     {events: []string{"PreToolUse"}, run: hooks.GuardDispatch},
	"gate-dispatch":      {events: []string{"PreToolUse"}, steps: []string{"rules-gate", "review-report"}},
	"rules-gate":         {},
	"post-edit-dispatch": {events: []string{"PostToolUse"}, steps: []string{"sanitize-output", "format-dispatch", "edit-seen", "plan-tick"}},
	"edit-seen":          {},
	"bash-edits":         {events: []string{"PostToolUse"}, steps: []string{"bash-edits", "bash-reports"}},
	"log-skills":         {events: []string{"PostToolUse", "UserPromptExpansion"}},
	"plan-mode-context":  {events: []string{"PostToolUse"}},
	"prompt-dispatch":    {events: []string{"UserPromptSubmit"}, steps: []string{"turn-tree", "plan-mode-context", "reply-length", "chat-slop"}},
	// stop-checks runs before review-report: the first block ends a run, and
	// stop-checks skips a retried Stop, which review-report still gates.
	"stop-dispatch":           {events: []string{"Stop"}, steps: []string{"reply-length", "chat-slop", "stop-checks", "review-report"}},
	"inject-context":          {events: []string{"SessionStart"}},
	"inject-subagent-context": {events: []string{"SubagentStart"}},
	"review-checks":           {events: []string{"SubagentStop"}},
	"reply-length":            {},
	"review-report":           {},
	"chat-slop":               {},
	"stop-checks":             {},
	"turn-tree":               {},
	"guard-plan":              {},
	// verify-claims runs last, so a cheap guard's block saves its model run.
	"handback-dispatch":  {events: []string{"PreToolUse"}, steps: []string{"guard-handback", "check-size", "check-citations", "verify-claims"}},
	"plan-dispatch":      {events: []string{"PreToolUse"}, steps: []string{"plan-changes", "check-size", "check-citations", "guard-plan", "verify-claims"}},
	"connector-dispatch": {events: []string{"PreToolUse"}, steps: []string{"guard-connector", "check-size", "verify-claims"}},
	"subagent-dispatch":  {events: []string{"SubagentStop"}, steps: []string{"subagent-slop", "check-size", "check-citations", "verify-claims"}},
	"check-citations":    {},
	"check-size":         {},
	"guard-handback":     {},
	"guard-connector":    {},
	"subagent-slop":      {},
	"verify-claims":      {},
	"guard-bash":         {},
	"guard-attribution":  {},
	"guard-commit":       {},
	"guard-edit":         {},
	"guard-pr":           {},
	"guard-skills":       {},
	"skills-nudge":       {},
	"adr-nudge":          {},
	"guard-slop":         {},
	"guard-weaken":       {},
	"guard-imports":      {},
	"guard-tokens":       {},
	"sanitize-output":    {},
	"format-dispatch":    {},
	"plan-tick":          {},
	"plan-changes":       {},
}

// cmdHook runs `kit hook <name>`: stdin is the payload, the exit status is
// Claude Code's (0 allow, 2 block). Anything unexpected fails open.
func cmdHook(e *env, args []string, stdin io.Reader) (status int) {
	start := time.Now()
	if len(args) == 0 {
		fmt.Fprintln(e.stderr, "kit hook: missing hook name")
		return 0
	}
	// The classifier's and the verifier's own claude sessions run no kit
	// hook. Only a value the kit set, with its marker, counts.
	markers := filepath.Join(e.paths.CacheDir(), cache.Guard)
	if claudecli.Honored(markers, gapfill.Guard) || claudecli.Honored(markers, claims.Guard) {
		return 0
	}
	name := args[0]
	def, ok := registry[name]
	if !ok {
		fmt.Fprintf(e.stderr, "kit hook: unknown hook %q\n", name)
		return 0
	}
	defer func() {
		if r := recover(); r != nil {
			hook.FailOpen(e.stderr, name, r)
			status = 0
		}
	}()

	raw, err := io.ReadAll(stdin)
	if err != nil {
		hook.FailOpen(e.stderr, name, err)
		return 0
	}
	h := &hook.Hook{
		Name:    name,
		Args:    args[1:],
		Payload: hook.ParsePayload(raw),
		Paths:   e.paths,
		Stdout:  e.stdout,
		Stderr:  e.stderr,
		Home:    e.home,
	}
	if limit := hookTimeout(e.paths.Root, name, h.Payload.String(hook.KeyHookEventName), h.Payload.String(hook.KeyToolName)); limit > 0 {
		git.SetDeadline(start.Add(limit - hookMargin))
	}
	if def.run != nil {
		return def.run(h)
	}
	steps := def.steps
	if steps == nil {
		steps = []string{name}
	}
	named := make([]hook.NamedCheck, len(steps))
	for i, s := range steps {
		named[i] = hook.NamedCheck{Name: s, Check: hookChecks[s]}
	}
	return hook.Run(h, named...)
}

// The bash guards come from bashguard.Steps, which kit explain reads too.
func init() {
	for _, s := range bashguard.Steps {
		hookChecks[s.Name] = s.Check
	}
}

// bashSteps are bash-dispatch's steps: bashguard.Steps, the write paths,
// the baseline bash-edits diffs from, the nudge, then the post checks,
// last so a cheap guard's block saves their model run.
func bashSteps() []string {
	names := make([]string, 0, len(bashguard.Steps)+4)
	for _, s := range bashguard.Steps {
		names = append(names, s.Name)
	}
	return append(names, "guard-bash-writes", "bash-baseline", "skills-nudge", "bash-post")
}

// bashPost is check-size and verify-claims on what a Bash command posts
// through gh or glab. hooks and bashguard share a layer, so the two meet here.
func bashPost(h *hook.Hook) error {
	if h.Payload.String(hook.KeyToolName) != "Bash" {
		return nil
	}
	text, where := bashguard.PostText(h.Payload.String(hook.KeyInputCommand), h.Home, h.Payload.Cwd())
	return hooks.CheckPost(h, text, where)
}

// onBashWrites is check run on the files a Bash command writes: guard-edit's
// path rules as guard-bash-writes, the scratch report checks as
// bash-reports, and bash-edits' git-move test. hooks and bashguard share a
// layer, so the two meet here.
func onBashWrites(check func(*hook.Hook, []string) error) hook.Check {
	return func(h *hook.Hook) error {
		if h.Payload.String(hook.KeyToolName) != "Bash" {
			return nil
		}
		return check(h, bashguard.WriteTargets(h.Payload.String(hook.KeyInputCommand), h.Home, h.Payload.Cwd()))
	}
}

// skillsNudge is skills-nudge, a Bash command's reads included. hooks and
// bashguard share a layer, so the two meet here.
func skillsNudge(h *hook.Hook) error {
	if h.Payload.String(hook.KeyToolName) != "Bash" {
		return hooks.SkillsNudge(h)
	}
	return hooks.NudgeFiles(h, bashguard.ReadTargets(h.Payload.String(hook.KeyInputCommand), h.Home, h.Payload.Cwd()))
}

// hookMargin is the time a hook keeps past its last git call to answer.
const hookMargin = time.Second

// hookTimeout is name's timeout in root's hooks/hooks.json, on event and,
// where a matcher picks tools, on tool: the shortest when several match,
// and 0 when none does or the file can't be read.
func hookTimeout(root, name, event, tool string) time.Duration {
	raw, err := os.ReadFile(filepath.Join(root, "hooks", "hooks.json"))
	if err != nil {
		return 0
	}
	var file struct {
		Hooks map[string][]struct {
			Matcher string
			Hooks   []struct {
				Command string
				Timeout int
			}
		}
	}
	if json.Unmarshal(raw, &file) != nil {
		return 0
	}
	var shortest time.Duration
	for _, group := range file.Hooks[event] {
		if !matcherPicks(group.Matcher, tool) {
			continue
		}
		for _, c := range group.Hooks {
			_, rest, ok := strings.Cut(c.Command, " hook ")
			if words := strings.Fields(rest); !ok || len(words) == 0 || words[0] != name || c.Timeout <= 0 {
				continue
			}
			if d := time.Duration(c.Timeout) * time.Second; shortest == 0 || d < shortest {
				shortest = d
			}
		}
	}
	return shortest
}

// matcherPicks is true when a hooks.json matcher picks tool: no matcher,
// "*", or a regexp over the whole name. An event with no tool takes any.
func matcherPicks(matcher, tool string) bool {
	if matcher == "" || matcher == "*" || tool == "" {
		return true
	}
	re, err := regexp.Compile("^(?:" + matcher + ")$")
	return err == nil && re.MatchString(tool)
}
