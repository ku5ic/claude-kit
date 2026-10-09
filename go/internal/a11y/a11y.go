// Package a11y is a11y-check: it runs axe (@axe-core/cli) against a running
// page and prints a digest of its violations. It never installs anything
// and never starts a server.
package a11y

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/project"
	"github.com/ku5ic/claude-kit/go/internal/tools"
)

var nonSlug = regexp.MustCompile(`[^A-Za-z0-9]+`)

// answers is true when url responds within 5 s; file:// URLs, which axe
// takes too, answer when the file exists.
func answers(url string) bool {
	if path, ok := strings.CutPrefix(url, "file://"); ok {
		_, err := os.Stat(path)
		return err == nil
	}
	if !strings.Contains(url, "://") {
		url = "http://" + url // as curl and axe read localhost:6006
	}
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// Run is `kit a11y-check <url>`. Exit codes: 0 ran (violations or not), 1 axe
// failed, 2 usage, 3 axe not installed, 4 the URL does not answer. The raw
// JSON (an array of axe-core results) is saved beside the scratch report
// path, as .json; the digest has one line per violated rule:
//
//	<rule id>  <impact>  <wcag tags>  nodes=<n>  <first selector>
func Run(cfg *config.Config, paths config.Paths, cwd string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: kit a11y-check <url>")
		return 2
	}
	url := args[0]
	physical, _ := filepath.EvalSymlinks(cwd)
	root := cmp.Or(project.Toplevel(cwd), physical)
	axe := tools.Resolve(cfg, physical, root, "axe", tools.AnyPath)
	if axe.Words == nil {
		fmt.Fprintln(stdout, "a11y-check: axe not found. Install it: npm install -D @axe-core/cli (or -g)")
		return 3
	}
	if !answers(url) {
		fmt.Fprintf(stdout, "a11y-check: %s does not answer.\n", url)
		startHint(cfg, root, stdout)
		return 4
	}
	raw, err := rawPath(cfg, paths, cwd, url)
	if err != nil {
		fmt.Fprintln(stderr, "a11y-check:", err)
		return 1
	}
	out, err := exec.Command(axe.Words[0], append(axe.Words[1:], url, "--stdout")...).Output()
	if err != nil || os.WriteFile(raw, out, 0o644) != nil || digest(out, url, raw, stdout) != nil {
		fmt.Fprintf(stderr, "a11y-check: axe failed on %s\n", url)
		return 1
	}
	return 0
}

// startHint names the package script that likely serves the page.
func startHint(cfg *config.Config, root string, stdout io.Writer) {
	tasks := project.Tasks(cfg, root)
	for _, candidate := range []string{"storybook", "dev"} {
		if i := slices.IndexFunc(tasks, func(t project.Task) bool { return t.Stack == "js" && t.Name == candidate }); i >= 0 {
			fmt.Fprintf(stdout, "Start it with: %s\n", tasks[i].Cmd)
			return
		}
	}
}

// rawPath is where the raw axe JSON goes: beside the scratch report path,
// as .json; a second run in the same minute keeps the first's results.
func rawPath(cfg *config.Config, paths config.Paths, cwd, url string) (string, error) {
	_, rest, found := strings.Cut(url, "://")
	if !found {
		rest = url
	}
	slug := nonSlug.ReplaceAllString(rest, "-")
	slug = strings.TrimSuffix(strings.TrimPrefix(slug[:min(len(slug), 40)], "-"), "-")
	dir, err := project.Dir(cfg, paths, cwd, "scratch", true)
	if err != nil {
		return "", err
	}
	stem := strings.TrimSuffix(project.ReportPath(dir, "a11y-runtime", slug, time.Now().Format("20060102-1504")), ".md")
	raw := stem + ".json"
	for n := 2; project.IsFile(raw); n++ {
		raw = fmt.Sprintf("%s-%d.json", stem, n)
	}
	return raw, nil
}

type violation struct {
	ID     string   `json:"id"`
	Impact *string  `json:"impact"`
	Tags   []string `json:"tags"`
	Nodes  []struct {
		Target []any `json:"target"`
	} `json:"nodes"`
}

// digest prints axe's JSON results as one line per violated rule.
func digest(out []byte, url, raw string, stdout io.Writer) error {
	var results []struct {
		Violations []violation `json:"violations"`
	}
	if err := json.Unmarshal(out, &results); err != nil {
		return err
	}
	count := 0
	for _, r := range results {
		count += len(r.Violations)
	}
	fmt.Fprintf(stdout, "a11y-check: %d violated rule(s) on %s (raw: %s)\n", count, url, raw)
	for _, r := range results {
		for _, v := range r.Violations {
			impact := "-"
			if v.Impact != nil {
				impact = *v.Impact
			}
			var wcag []string
			for _, t := range v.Tags {
				if strings.HasPrefix(t, "wcag") {
					wcag = append(wcag, t)
				}
			}
			tags := cmp.Or(strings.Join(wcag, ","), "-")
			fmt.Fprintf(stdout, "%s  %s  %s  nodes=%d  %s\n", v.ID, impact, tags, len(v.Nodes), v.firstSelector())
		}
	}
	return nil
}

// firstSelector is the first node's target as axe prints it; a nested
// (shadow DOM) target joins with ">>".
func (v violation) firstSelector() string {
	if len(v.Nodes) == 0 {
		return ""
	}
	var parts []string
	for _, t := range v.Nodes[0].Target {
		switch x := t.(type) {
		case string:
			parts = append(parts, x)
		case []any:
			var inner []string
			for _, s := range x {
				inner = append(inner, fmt.Sprint(s))
			}
			parts = append(parts, strings.Join(inner, " "))
		}
	}
	return strings.Join(parts, " >> ")
}
