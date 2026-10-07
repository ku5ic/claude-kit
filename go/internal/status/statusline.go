// Package status renders Claude Code's statusLine and subagentStatusLine.
// Neither may ever break a render: anything unexpected yields a
// best-effort line or no output, never an error.
package status

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	var v any = data
	for part := range strings.SplitSeq(path, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return def
		}
		v = m[part]
	}
	switch x := v.(type) {
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

	modelName := jqString(data, "model.display_name", "unknown")
	cwd := jqString(data, "workspace.current_dir", "")
	sessionID := jqString(data, "session_id", "")
	ctxPct := jqString(data, "context_window.used_percentage", "0")
	cost := jqString(data, "cost.total_cost_usd", "0")
	durationMS := jqString(data, "cost.total_duration_ms", "0")
	effort := jqString(data, "effort.level", "")
	fiveH := jqString(data, "rate_limits.five_hour.used_percentage", "")
	agent := jqString(data, "agent.name", "")
	if agent == "" {
		agent = jqString(data, "agent_type", "")
	}
	transcript := jqString(data, "transcript_path", "")

	actualShort, actualDisplay, sessionShort, declaredShort, declaredDisplay := models(transcript, modelName)

	dirName := filepath.Base(cwd)
	if cwd == "" {
		dirName = "."
	}
	gitSegment := gitStatus(home, cwd, sessionID)

	row1 := modelColor + modelName + reset
	switch {
	case declaredShort != "":
		// Usually the override silently fell back to the session model,
		// already shown first; name the actual model only when it's a
		// third, different one.
		if actualShort == sessionShort {
			row1 += "  " + red + "!" + declaredDisplay + reset
		} else {
			row1 += "  " + red + "!" + declaredDisplay + "->  " + actualDisplay + reset
		}
	case actualShort != "" && actualShort != sessionShort:
		row1 += "  " + yellow + "->  " + actualDisplay + reset
	}
	if agent != "" {
		row1 += " (" + agent + ")"
	}
	row1 += "  " + dirColor + dirName + reset
	if gitSegment != "" {
		parts := strings.SplitN(gitSegment, "\t", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		row1 += "  " + branchColor + parts[0] + reset + " " + addColor + "+" + parts[1] + reset + " " + delColor + "~" + parts[2] + reset
	}

	ctx, _ := strconv.Atoi(strings.SplitN(ctxPct, ".", 2)[0])
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

	costFmt := cost
	if f, err := strconv.ParseFloat(cost, 64); err == nil {
		costFmt = fmt.Sprintf("%.2f", f)
	}
	ms, _ := strconv.ParseFloat(durationMS, 64)
	seconds := int(ms) / 1000
	var duration string
	switch {
	case seconds >= 3600:
		duration = fmt.Sprintf("%dh %dm", seconds/3600, seconds%3600/60)
	case seconds >= 60:
		duration = fmt.Sprintf("%dm", seconds/60)
	default:
		duration = fmt.Sprintf("%ds", seconds)
	}

	row2 := fmt.Sprintf("%s%s%s %d%%  $%s   %s%s%s", color, bar, reset, ctx, costFmt, durationColor, duration, reset)
	tail := ""
	if effort != "" {
		tail += "  effort:" + effort
	}
	if fiveH != "" {
		tail += "  5h:" + strings.SplitN(fiveH, ".", 2)[0] + "%"
	}
	if tail != "" {
		row2 += " " + tail
	}
	fmt.Fprintf(stdout, "%s\n%s\n", row1, row2)
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
func models(transcript, modelName string) (actualShort, actualDisplay, sessionShort, declaredShort, declaredDisplay string) {
	if transcript == "" {
		return
	}
	lines := tailLines(transcript, 60)
	type entry struct {
		Type        string `json:"type"`
		IsMeta      bool   `json:"isMeta"`
		IsSidechain bool   `json:"isSidechain"`
		Timestamp   string `json:"timestamp"`
		Message     struct {
			Model   string          `json:"model"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
		Attachment struct {
			Type  string `json:"type"`
			Model string `json:"model"`
		} `json:"attachment"`
	}
	var entries []entry
	for _, line := range lines {
		var e entry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}
	since, actualID := "", ""
	for _, e := range entries {
		if e.Type == "user" && !e.IsMeta && promptText(e.Message.Content) != "" {
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

// promptText is a user entry's text: the string content, or its text
// blocks joined.
func promptText(content json.RawMessage) string {
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func tailLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	// Transcripts grow large; read only the end.
	const window = 4 << 20
	if info, err := f.Stat(); err == nil && info.Size() > window {
		_, _ = f.Seek(-window, io.SeekEnd) // failing, it reads from the start: slower, same tail
	}
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 32*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		// What was read before the error is not the file's tail.
		return nil
	}
	return lines[max(0, len(lines)-n):]
}

// gitStatus is "branch\tadditions\tdeletions" for cwd's repo, cached per
// session for STATUSLINE_CACHE_TTL seconds (default 1, statusLine's
// refreshInterval), or "" outside a repo. Staged plus unstaged numstat, as
// `git diff --shortstat` counts; not `git diff HEAD`, which fails on an
// unborn branch.
func gitStatus(home, cwd, sessionID string) string {
	if cwd == "" || exec.Command("git", "-C", cwd, "rev-parse", "--is-inside-work-tree").Run() != nil {
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
	dir := filepath.Join(home, "cache", "statusline")
	file := filepath.Join(dir, "git-"+safe)
	if info, err := os.Stat(file); err == nil && info.Size() > 0 && time.Now().Unix()-info.ModTime().Unix() < int64(ttl) {
		data, _ := os.ReadFile(file)
		return strings.TrimSuffix(string(data), "\n")
	}
	_ = os.MkdirAll(dir, 0o755) // without it, the segment just isn't cached
	out, _ := exec.Command("git", "-C", cwd, "branch", "--show-current").Output()
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		branch = "detached"
	}
	add, del := 0, 0
	for _, args := range [][]string{{"diff", "--numstat"}, {"diff", "--cached", "--numstat"}} {
		out, _ := exec.Command("git", append([]string{"-C", cwd}, args...)...).Output()
		for line := range strings.SplitSeq(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				a, _ := strconv.Atoi(f[0]) // "-" for binary files counts 0
				d, _ := strconv.Atoi(f[1])
				add, del = add+a, del+d
			}
		}
	}
	segment := fmt.Sprintf("%s\t%d\t%d", branch, add, del)
	if tmp, err := os.CreateTemp(dir, ".git-*"); err == nil {
		_, werr := tmp.WriteString(segment + "\n")
		// A short write must not be cached for the whole ttl.
		if errors.Join(werr, tmp.Close()) != nil || os.Rename(tmp.Name(), file) != nil {
			os.Remove(tmp.Name())
		}
	}
	return segment
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
