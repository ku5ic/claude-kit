// Package status renders Claude Code's statusLine and subagentStatusLine.
// Neither may ever break a render: anything unexpected yields a
// best-effort line or no output, never an error.
package status

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/extract"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/transcript"
)

const (
	barWidth = 10 // blocks in the context-usage bar
	yellowAt = 70 // bar turns yellow at >=70% context used
	redAt    = 90 // bar turns red at >=90% context used

	reset         = "\033[0m"
	yellow        = "\033[33m"
	red           = "\033[31m"
	green         = "\033[32m"
	modelColor    = "\033[38;5;111m"
	dirColor      = "\033[38;5;216m"
	branchColor   = "\033[38;5;141m"
	addColor      = "\033[38;5;150m"
	delColor      = "\033[38;5;209m"
	durationColor = "\033[38;5;245m"
)

// jqString is jq -r's rendering of `.a.b // default`: a missing, null, or
// false value gives default; a number prints in jq's shortest form.
func jqString(data map[string]any, path, def string) string {
	switch x := extract.GetPath(data, path).(type) {
	case nil:
		return def
	case bool:
		if !x {
			return def
		}
		return "true"
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// Statusline prints the two-row status: model/agent/dir/git on row 1,
// context/cost/duration/effort/rate limit on row 2.
func Statusline(stdin io.Reader, stdout io.Writer, home string) {
	defer func() { _ = recover() }()
	raw, _ := io.ReadAll(stdin)
	var data map[string]any
	_ = json.Unmarshal(raw, &data) // bad input renders the defaults
	fmt.Fprintf(stdout, "%s\n%s\n", modelRow(data, home), usageRow(data))
}

// modelRow is the model, any model divergence, the agent, the directory,
// and the git segment.
func modelRow(data map[string]any, home string) string {
	modelName := jqString(data, "model.display_name", "unknown")
	cwd := jqString(data, "workspace.current_dir", "")
	agent := cmp.Or(jqString(data, "agent.name", ""), jqString(data, "agent_type", ""))
	actualShort, actualDisplay, sessionShort, declaredShort, declaredDisplay := models(jqString(data, "transcript_path", ""), modelName)

	row := modelColor + modelName + reset
	switch {
	case declaredShort != "":
		// Usually the override silently fell back to the session model,
		// already shown first; name the actual model only when it's a
		// third, different one.
		if actualShort == sessionShort {
			row += "  " + red + "!" + declaredDisplay + reset
		} else {
			row += "  " + red + "!" + declaredDisplay + "->  " + actualDisplay + reset
		}
	case actualShort != "" && actualShort != sessionShort:
		row += "  " + yellow + "->  " + actualDisplay + reset
	}
	if agent != "" {
		row += " (" + agent + ")"
	}
	dirName := filepath.Base(cwd)
	if cwd == "" {
		dirName = "."
	}
	row += "  " + dirColor + dirName + reset
	if seg := gitStatus(home, cwd, jqString(data, "session_id", "")); seg != "" {
		parts := strings.SplitN(seg, "\t", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		row += "  " + branchColor + parts[0] + reset + " " + addColor + "+" + parts[1] + reset + " " + delColor + "~" + parts[2] + reset
	}
	return row
}

// usageRow is the context bar, cost, duration, effort, and 5h rate limit.
func usageRow(data map[string]any) string {
	ctx, _ := strconv.Atoi(strings.SplitN(jqString(data, "context_window.used_percentage", "0"), ".", 2)[0])
	ctx = min(max(ctx, 0), 100)
	filled := ctx / (100 / barWidth)
	color := green
	switch {
	case ctx >= redAt:
		color = red
	case ctx >= yellowAt:
		color = yellow
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	cost := jqString(data, "cost.total_cost_usd", "0")
	if f, err := strconv.ParseFloat(cost, 64); err == nil {
		cost = fmt.Sprintf("%.2f", f)
	}
	ms, _ := strconv.ParseFloat(jqString(data, "cost.total_duration_ms", "0"), 64)
	row := fmt.Sprintf("%s%s%s %d%%  $%s   %s%s%s", color, bar, reset, ctx, cost, durationColor, duration(int(ms)/1000), reset)
	tail := ""
	if effort := jqString(data, "effort.level", ""); effort != "" {
		tail += "  effort:" + effort
	}
	if fiveH := jqString(data, "rate_limits.five_hour.used_percentage", ""); fiveH != "" {
		tail += "  5h:" + strings.SplitN(fiveH, ".", 2)[0] + "%"
	}
	if tail != "" {
		row += " " + tail
	}
	return row
}

// duration is seconds as "1h 2m", "3m", or "45s".
func duration(seconds int) string {
	switch {
	case seconds >= 3600:
		return fmt.Sprintf("%dh %dm", seconds/3600, seconds%3600/60)
	case seconds >= 60:
		return fmt.Sprintf("%dm", seconds/60)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

var (
	releaseDate   = regexp.MustCompile(`-[0-9]{8,}$`)
	versionSuffix = regexp.MustCompile(`-[0-9].*$`)
	displayNumber = regexp.MustCompile(`[[:space:]]+[0-9].*$`)
)

// modelDisplay turns "claude-opus-5" or "claude-haiku-4-5-20251001" into
// the display form model.display_name uses: "Opus 5", "Haiku 4.5".
func modelDisplay(id string) string {
	id = releaseDate.ReplaceAllString(strings.TrimPrefix(id, "claude-"), "")
	family, version, hasVersion := strings.Cut(id, "-")
	if family != "" {
		family = strings.ToUpper(family[:1]) + family[1:]
	}
	if !hasVersion {
		return family
	}
	return family + " " + strings.ReplaceAll(version, "-", ".")
}

func modelShort(id string) string {
	return versionSuffix.ReplaceAllString(strings.TrimPrefix(id, "claude-"), "")
}

// models reads the actually running model from the transcript's last 60
// lines: the payload's model is the session model and never reflects a
// skill's per-turn override. It also finds a model a skill declared this
// turn (a command_permissions attachment newer than the turn's prompt)
// that didn't take.
func models(path, modelName string) (actualShort, actualDisplay, sessionShort, declaredShort, declaredDisplay string) {
	if path == "" {
		return
	}
	entries, _ := transcript.Tail(path, 60)
	since, actualID := "", ""
	for _, e := range entries {
		if e.Type == "user" && !e.IsMeta && e.PromptText() != "" {
			since = e.Timestamp
		}
		if e.Type == "assistant" && !e.IsSidechain && e.Message.Model != "" {
			actualID = e.Message.Model
		}
	}
	declaredID := ""
	if since != "" {
		for _, e := range entries {
			if e.Type == "attachment" && e.Attachment.Type == "command_permissions" && e.Timestamp >= since && e.Attachment.Model != "" {
				declaredID = e.Attachment.Model
			}
		}
	}
	if actualID == "" {
		return
	}
	actualShort, actualDisplay = modelShort(actualID), modelDisplay(actualID)
	// Strip the display name's version the way the id's is stripped, or
	// "sonnet 5" never equals "sonnet" and every render shows a divergence.
	sessionShort = displayNumber.ReplaceAllString(strings.ToLower(modelName), "")
	if declaredID != "" && declaredID != actualID {
		declaredShort, declaredDisplay = modelShort(declaredID), modelDisplay(declaredID)
	}
	return
}

// gitStatus is "branch\tadditions\tdeletions" for cwd's repo, cached per
// session for STATUSLINE_CACHE_TTL seconds (default 1, statusLine's
// refreshInterval), or "" outside a repo. Staged plus unstaged numstat, as
// `git diff --shortstat` counts; not `git diff HEAD`, which fails on an
// unborn branch.
func gitStatus(home, cwd, sessionID string) string {
	if cwd == "" || git.Command(cwd, "rev-parse", "--is-inside-work-tree").Run() != nil {
		return ""
	}
	ttl, err := strconv.Atoi(os.Getenv("STATUSLINE_CACHE_TTL"))
	if err != nil || ttl < 0 {
		ttl = 1
	}
	safe := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(sessionID, "")
	if safe == "" {
		safe = "nosession"
	}
	dir := filepath.Join(config.Paths{Home: home}.CacheDir(), config.Statusline)
	file := filepath.Join(dir, "git-"+safe)
	if info, err := os.Stat(file); err == nil && info.Size() > 0 && time.Now().Unix()-info.ModTime().Unix() < int64(ttl) {
		data, _ := os.ReadFile(file)
		return strings.TrimSuffix(string(data), "\n")
	}
	_ = os.MkdirAll(dir, 0o755) // without it, the segment just isn't cached
	branch, _ := git.Line(cwd, "branch", "--show-current")
	if branch == "" {
		branch = "detached"
	}
	add, del := 0, 0
	for _, args := range [][]string{{"diff", "--numstat"}, {"diff", "--cached", "--numstat"}} {
		out, _ := git.Output(cwd, args...)
		for line := range strings.SplitSeq(out, "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				a, _ := strconv.Atoi(f[0]) // "-" for binary files counts 0
				d, _ := strconv.Atoi(f[1])
				add, del = add+a, del+d
			}
		}
	}
	// Untracked files count as all-new lines, as they will once added. From
	// the top: ls-files --others lists only cwd's subtree, numstat the whole repo.
	// This runs on every refresh, so only regular files are read, within a
	// byte budget: past it, or for one larger file, the count runs low.
	if top := project.Toplevel(cwd); top != "" {
		untracked, _ := git.Output(top, "ls-files", "--others", "--exclude-standard", "-z")
		budget := int64(untrackedReadBudget)
		for name := range strings.SplitSeq(untracked, "\x00") {
			path := filepath.Join(top, name)
			info, err := os.Lstat(path)
			if name == "" || err != nil || !info.Mode().IsRegular() || info.Size() > budget {
				continue
			}
			budget -= info.Size()
			add += newLines(path)
		}
	}
	segment := fmt.Sprintf("%s\t%d\t%d", branch, add, del)
	_ = project.WriteAtomic(file, []byte(segment+"\n"), 0o600)
	return segment
}

// untrackedReadBudget caps the bytes one refresh reads to count untracked lines.
const untrackedReadBudget = 8 << 20

// newLines is a new file's line count as git diff --numstat shows it: a
// last line without a newline counts, and a binary file (a NUL in its first
// 8000 bytes, git's test) counts 0.
func newLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	buf := make([]byte, 64*1024)
	n, last, first := 0, byte('\n'), true
	for {
		k, err := f.Read(buf)
		if first && bytes.IndexByte(buf[:min(k, 8000)], 0) >= 0 {
			return 0
		}
		first = false
		if k > 0 {
			n += bytes.Count(buf[:k], []byte{'\n'})
			last = buf[k-1]
		}
		if err != nil {
			break
		}
	}
	if last != '\n' {
		n++
	}
	return n
}

// SubagentStatusline prints one {"id","content"} JSON object per task:
// "name [status]  <model>  effort:<level>  <ctx%>". Anything unexpected
// prints nothing: lines not matching that shape are discarded and logged
// by Claude Code.
func SubagentStatusline(stdin io.Reader, stdout io.Writer) {
	defer func() { _ = recover() }()
	var payload struct {
		Tasks []map[string]any `json:"tasks"`
	}
	raw, _ := io.ReadAll(stdin)
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	var out strings.Builder
	for _, t := range payload.Tasks {
		id := jqString(t, "id", "")
		if id == "" {
			continue
		}
		name := jqString(t, "name", "")
		if _, ok := t["name"]; !ok || t["name"] == nil || t["name"] == false {
			name = jqString(t, "label", "")
			if _, ok := t["label"]; !ok || t["label"] == nil || t["label"] == false {
				name = jqString(t, "description", "task")
			}
		}
		head := name
		if status := jqString(t, "status", ""); status != "" {
			head = name + " [" + status + "]"
		}
		fields := []string{head}
		if model := jqString(t, "model", ""); model != "" {
			fields = append(fields, model)
		}
		if effort := jqString(t, "effort", ""); effort != "" {
			fields = append(fields, "effort:"+effort)
		}
		size, _ := t["contextWindowSize"].(float64)
		if tokens, ok := t["tokenCount"].(float64); ok && size > 0 {
			fields = append(fields, strconv.Itoa(int(tokens*100/size))+"%")
		}
		// Not json.Marshal: it escapes <, >, and &, which jq -c doesn't.
		enc := json.NewEncoder(&out)
		enc.SetEscapeHTML(false)
		if enc.Encode(struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		}{id, strings.Join(fields, "  ")}) != nil {
			return
		}
	}
	fmt.Fprint(stdout, out.String())
}
