package bashguard

import (
	"regexp"
	"slices"

	"github.com/ku5ic/claude-kit/go/internal/attribution"
	"github.com/ku5ic/claude-kit/go/internal/classify"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/guard"
	"github.com/ku5ic/claude-kit/go/internal/hook"
)

var (
	// gitMessageFiles are the git subcommands that write a message, each
	// with the flags that name a file it reads the message from.
	gitMessageFiles = map[string]map[string]string{
		"commit":      {"-F": "file", "--file": "file", "-t": "file", "--template": "file"},
		"commit-tree": {"-F": "file"}, "merge": {"-F": "file", "--file": "file"}, "tag": {"-F": "file", "--file": "file"},
		"notes": {"-F": "file", "--file": "file"}, "pull": nil, "revert": nil, "cherry-pick": nil, "rebase": nil, "am": nil,
	}
	// ghReads are the gh and glab commands that only read: a top-level
	// command, or any group's subcommand. Every other call may post text.
	ghReads = []string{"search", "browse", "auth", "status", "help", "version", "completion",
		"view", "list", "diff", "checks", "download", "watch", "checkout"}
	// gitOrGh is a git, gh, or glab word, for a command that didn't parse.
	gitOrGh = regexp.MustCompile(`\b(git|gh|glab)\b`)
)

// CheckAttribution is guard-attribution: it blocks a git command that
// writes a message (a commit, merge, tag, or note) and a gh or glab
// command that may post text (a PR or MR, issue, release, or api call) when the text holds
// an AI tool's attribution (kit.yml attribution). The whole command is
// scanned, since the text can come from anywhere in it, then each file the
// call reads text from. Where part of the command didn't parse, any git,
// gh, or glab in it counts.
func CheckAttribution(h *hook.Hook) error {
	if h.Payload.Err != nil {
		return h.Payload.Err
	}
	cfg := h.Config()
	if cfg == nil {
		return nil
	}
	cmd := h.Payload.String(hook.KeyInputCommand)
	segs, unparsed := parseAll(cmd)
	texts, writes := postedTexts(segs, h.Home, h.Payload.Cwd())
	if !writes && (!unparsed || !gitOrGh.MatchString(cmd)) {
		return nil
	}
	d := attribution.New(cfg.Attribution)
	for _, t := range append([]prText{{where: "the command", text: cmd}}, texts...) {
		if line, ok := d.Find(t.text); ok {
			return h.Block(attribution.Message(t.where, line), "ai-attribution")
		}
	}
	return nil
}

// postedTexts is the text of each file a git, gh, or glab call in segs reads a
// message or post from, and writes whether any call writes one.
func postedTexts(segs []Segment, home, cwd string) (texts []prText, writes bool) {
	for w := range walk(segs, home, cwd) {
		args := values(w.call.Words[w.i+1:])
		src := hostCall{tool: w.name, call: w.call, before: w.seg.src[:min(w.call.start, len(w.seg.src))], home: home, cwd: w.cwd}
		switch w.name {
		case "git":
			sub, rest, dir := classify.GitSubcommand(args)
			flags, ok := gitMessageFiles[sub]
			if !ok {
				continue
			}
			writes = true
			if dir != "" {
				src.cwd = fsx.Abs(w.cwd, guard.ExpandHome(home, dir))
			}
			walkFlags(rest, flags, func(_, file string) { texts = append(texts, src.read("git "+sub+" -F", file)) })
		case "gh", "glab":
			if len(args) == 0 || slices.Contains(ghReads, args[0]) || len(args) > 1 && slices.Contains(ghReads, args[1]) {
				continue
			}
			writes = true
			texts = append(texts, src.texts(w.i+1)...)
			if args[0] == "api" {
				// The whole request body, not only its PR text fields.
				walkFlags(args[1:], map[string]string{"--input": "input"}, func(_, file string) { texts = append(texts, src.read(w.name+" api --input", file)) })
			}
		}
	}
	return texts, writes
}
