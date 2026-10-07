package checks

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/ku5ic/claude-kit/go/internal/config"
)

// ciStep is one shell step a CI config runs: its file (relative to the
// root), the directory it runs in (relative), its literal env, and its
// script.
type ciStep struct {
	file, dir string
	env       []string
	run       string
}

var nonWord = regexp.MustCompile(`[^A-Za-z0-9]+`)

// ciSteps reads the root's GitHub Actions workflows and GitLab CI config
// for the shell steps a local run could repeat. Whole jobs are dropped
// when they need what a laptop doesn't have or shouldn't use (services, a
// container, OIDC, a deployment environment, secrets in their env, a
// login action, a deny-listed name), and steps when they use an action, a
// non-sh shell, secrets, or a CI expression.
func ciSteps(cfg *config.Config, root string) []ciStep {
	var steps []ciStep
	workflows, _ := filepath.Glob(filepath.Join(root, ".github/workflows/*.y*ml"))
	sort.Strings(workflows)
	for _, file := range workflows {
		steps = append(steps, githubSteps(cfg, root, file)...)
	}
	steps = append(steps, gitlabSteps(cfg, root, filepath.Join(root, ".gitlab-ci.yml"))...)
	return steps
}

// HasCI is true when root has a CI config run-checks reads.
func HasCI(root string) bool {
	workflows, _ := filepath.Glob(filepath.Join(root, ".github/workflows/*.y*ml"))
	_, err := os.Stat(filepath.Join(root, ".gitlab-ci.yml"))
	return len(workflows) > 0 || err == nil
}

func readYAML(file string) map[string]any {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var doc map[string]any
	if yaml.Unmarshal(data, &doc) != nil {
		return nil
	}
	return doc
}

func githubSteps(cfg *config.Config, root, file string) []ciStep {
	doc := readYAML(file)
	jobs, _ := doc["jobs"].(map[string]any)
	// Workflow-level permissions, env, and defaults hold for every job.
	if !permissionsOK(doc["permissions"]) {
		return nil
	}
	workflowEnv, ok := literalEnv(doc["env"])
	if !ok {
		return nil
	}
	workflowDir, workflowShell := runDefaults(doc)
	rel := strings.TrimPrefix(file, root+"/")
	ids := make([]string, 0, len(jobs))
	for id := range jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []ciStep
	for _, id := range ids {
		job, _ := jobs[id].(map[string]any)
		if job == nil || !githubJobOK(cfg, id, job) {
			continue
		}
		jobEnv, ok := literalEnv(job["env"])
		if !ok {
			continue
		}
		jobEnv = append(slices.Clone(workflowEnv), jobEnv...)
		defaultDir, defaultShell := runDefaults(job)
		if defaultDir == "" {
			defaultDir = workflowDir
		}
		if defaultShell == "" {
			defaultShell = workflowShell
		}
		// A Windows runner's default shell is pwsh.
		if defaultShell == "" && strings.Contains(strings.ToLower(str(job["runs-on"])), "windows") {
			defaultShell = "pwsh"
		}
		steps, _ := job["steps"].([]any)
		for _, s := range steps {
			step, _ := s.(map[string]any)
			run, _ := step["run"].(string)
			if step == nil || run == "" || step["uses"] != nil || strings.Contains(run, "${{") || deniedName(cfg, str(step["name"])) {
				continue
			}
			shell := str(step["shell"])
			if shell == "" {
				shell = defaultShell
			}
			if shell != "" && shell != "bash" && shell != "sh" {
				continue
			}
			env, ok := literalEnv(step["env"])
			if !ok {
				continue
			}
			dir := defaultDir
			if d := str(step["working-directory"]); d != "" {
				dir = d
			}
			if strings.Contains(dir, "${{") {
				continue
			}
			out = append(out, ciStep{file: rel, dir: filepath.Clean(dir), env: append(slices.Clone(jobEnv), env...), run: run})
		}
	}
	return out
}

// githubJobOK is false for a job a local run must not, or can't, repeat.
func githubJobOK(cfg *config.Config, id string, job map[string]any) bool {
	for _, key := range []string{"services", "container", "environment", "uses"} {
		if job[key] != nil {
			return false
		}
	}
	if !permissionsOK(job["permissions"]) || deniedName(cfg, id) || deniedName(cfg, str(job["name"])) {
		return false
	}
	steps, _ := job["steps"].([]any)
	for _, s := range steps {
		step, _ := s.(map[string]any)
		uses := str(step["uses"])
		if uses != "" && slices.ContainsFunc(cfg.GateDiscovery.DenyActions, func(a string) bool { return strings.HasPrefix(uses, a) }) {
			return false
		}
	}
	return true
}

// permissionsOK is false for write-all or an OIDC token (id-token: write),
// which a job uses to reach a cloud or a registry.
func permissionsOK(v any) bool {
	switch p := v.(type) {
	case string:
		return p != "write-all"
	case map[string]any:
		return p["id-token"] != "write"
	}
	return true
}

// runDefaults is a workflow's or job's defaults.run working-directory and
// shell.
func runDefaults(m map[string]any) (dir, shell string) {
	defaults, _ := m["defaults"].(map[string]any)
	run, _ := defaults["run"].(map[string]any)
	return str(run["working-directory"]), str(run["shell"])
}

// gitlabReserved are .gitlab-ci.yml's top-level keys that aren't jobs.
var gitlabReserved = []string{"stages", "variables", "include", "default", "workflow", "image", "services", "before_script", "after_script", "cache", "pages"}

func gitlabSteps(cfg *config.Config, root, file string) []ciStep {
	doc := readYAML(file)
	if doc == nil {
		return nil
	}
	// include: local files merge in first, so jobs can extend their templates.
	merged := map[string]any{}
	for _, inc := range gitlabLocalIncludes(doc["include"]) {
		for k, v := range readYAML(filepath.Join(root, strings.TrimPrefix(inc, "/"))) {
			merged[k] = v
		}
	}
	for k, v := range doc {
		merged[k] = v
	}
	// Global variables, and default: services or id_tokens, hold for every
	// job that doesn't set its own.
	globalEnv, ok := literalEnv(merged["variables"])
	if !ok {
		return nil
	}
	def, _ := merged["default"].(map[string]any)
	rel := strings.TrimPrefix(file, root+"/")
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []ciStep
	for _, name := range names {
		job, _ := merged[name].(map[string]any)
		if job == nil || strings.HasPrefix(name, ".") || slices.Contains(gitlabReserved, name) {
			continue
		}
		job = gitlabExtend(merged, job, 0)
		inherited := func(key string) any {
			if v := job[key]; v != nil {
				return v
			}
			if v := def[key]; v != nil {
				return v
			}
			return merged[key]
		}
		if inherited("services") != nil || inherited("id_tokens") != nil || job["environment"] != nil || job["secrets"] != nil || job["trigger"] != nil || deniedName(cfg, name) {
			continue
		}
		env, ok := literalEnv(job["variables"])
		if !ok {
			continue
		}
		env = append(slices.Clone(globalEnv), env...)
		// GitLab runs before_script and script in one shell, so a cd or an
		// export on one line holds for the next: they're one step.
		lines, ok := scriptLines(inherited("before_script"))
		script, ok2 := scriptLines(job["script"])
		if !ok || !ok2 || len(script) == 0 {
			continue
		}
		out = append(out, ciStep{file: rel, dir: ".", env: env, run: strings.Join(append(lines, script...), "\n")})
	}
	return out
}

// scriptLines is a GitLab script list; ok is false when a line uses a CI
// variable or is a !reference (a nested list, as yaml.v3 decodes it),
// which a local run can't reproduce.
func scriptLines(v any) ([]string, bool) {
	items, _ := v.([]any)
	var out []string
	for _, item := range items {
		s, isString := item.(string)
		if !isString || strings.Contains(s, "$CI_") {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func gitlabLocalIncludes(v any) []string {
	var out []string
	add := func(item any) {
		switch item := item.(type) {
		case string:
			if !strings.Contains(item, "://") {
				out = append(out, item)
			}
		case map[string]any:
			if local := str(item["local"]); local != "" {
				out = append(out, local)
			}
		}
	}
	switch v := v.(type) {
	case []any:
		for _, item := range v {
			add(item)
		}
	default:
		add(v)
	}
	return out
}

// gitlabExtend resolves a job's extends: each template's keys apply first,
// the job's own keys last.
func gitlabExtend(doc, job map[string]any, depth int) map[string]any {
	var parents []string
	switch e := job["extends"].(type) {
	case string:
		parents = []string{e}
	case []any:
		for _, p := range e {
			if s, ok := p.(string); ok {
				parents = append(parents, s)
			}
		}
	}
	if len(parents) == 0 || depth >= maxDepth {
		return job
	}
	out := map[string]any{}
	for _, p := range parents {
		if parent, ok := doc[p].(map[string]any); ok {
			for k, v := range gitlabExtend(doc, parent, depth+1) {
				out[k] = v
			}
		}
	}
	for k, v := range job {
		out[k] = v
	}
	delete(out, "extends")
	return out
}

// literalEnv is a CI env or variables map as K=V words; ok is false when any
// value is a secret or a CI expression, which a local run can't have.
func literalEnv(v any) ([]string, bool) {
	m, _ := v.(map[string]any)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		value := str(m[k])
		if strings.Contains(value, "${{") || strings.Contains(value, "secrets.") || strings.Contains(value, "$CI_") {
			return nil, false
		}
		out = append(out, k+"="+value)
	}
	return out, true
}

// deniedName is true when a job or step name holds a deny_names word.
func deniedName(cfg *config.Config, name string) bool {
	for _, w := range nonWord.Split(strings.ToLower(name), -1) {
		if slices.Contains(cfg.GateDiscovery.DenyNames, w) {
			return true
		}
	}
	return false
}

// deniedCommand is true when words hold a deny_commands token: one word
// anywhere, or several in a row.
func deniedCommand(cfg *config.Config, words []string) bool {
	for _, token := range cfg.GateDiscovery.DenyCommands {
		t := strings.Fields(token)
		for i := 0; i+len(t) <= len(words); i++ {
			if slices.Equal(words[i:i+len(t)], t) {
				return true
			}
		}
	}
	return false
}

func str(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		b, _ := yaml.Marshal(v)
		return strings.TrimSpace(string(b))
	}
}
