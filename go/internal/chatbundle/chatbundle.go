// Package chatbundle builds the plugin zip claude.ai chat installs: the
// skills kit.yml's chat_bundle names, generated from the kit's own files.
package chatbundle

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/fsx"
	"github.com/ku5ic/claude-kit/go/internal/md"
)

const (
	usage = "usage: kit bundle chat [--check]"

	manifestPath = ".claude-plugin/plugin.json"

	// claude.ai's upload limits, in characters.
	maxPluginDescription = 500
	maxSkillDescription  = 1024

	// The skills spec's ceiling for a SKILL.md, in lines.
	maxSkillLines = 500

	rulesHeadPath = "templates/chat/kit-rules.md"
)

// zipTime is every entry's time: a source's mtime would make two builds of
// the same files differ.
var zipTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// xmlTag is what claude.ai refuses in a skill's name or description.
var xmlTag = regexp.MustCompile(`<[a-zA-Z/]`)

// Run is `kit bundle chat`: it writes the zip to root's bin/ and prints its
// path, or with --check only validates. It exits 1 when the bundle has a
// problem, 2 on a usage error.
func Run(root string, cfg *config.Config, args []string, stdout, stderr io.Writer) int {
	if slices.Contains(args, "-h") || slices.Contains(args, "--help") {
		fmt.Fprintln(stdout, usage)
		return 0
	}
	check := slices.Equal(args, []string{"chat", "--check"})
	if !check && !slices.Equal(args, []string{"chat"}) {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	b, err := build(root, cfg)
	if err == nil {
		err = errors.Join(b.transform(cfg.ChatBundle)...)
	}
	if err == nil {
		err = errors.Join(slices.Concat(b.problems(cfg.ChatBundle), b.lint())...)
	}
	if err != nil {
		fmt.Fprintln(stderr, "kit bundle:", err)
		return 1
	}
	if check {
		return 0
	}
	out := filepath.Join(root, "bin", "kit-chat-"+b.version+".zip")
	err = b.addBinary(root)
	if err == nil {
		err = b.write(out)
	}
	if err != nil {
		fmt.Fprintln(stderr, "kit bundle:", err)
		return 1
	}
	fmt.Fprintln(stdout, out)
	return 0
}

// bundle is the plugin in memory: each file's slash path and content, the
// rule files as written with the head of the skill they become, and the
// skills left out.
type bundle struct {
	version   string
	files     map[string][]byte
	rules     map[string]string
	rulesHead string
	others    []string
}

func build(root string, cfg *config.Config) (*bundle, error) {
	c := cfg.ChatBundle
	data, version, err := manifest(root, c.Description)
	if err != nil {
		return nil, err
	}
	b := &bundle{version: version, files: map[string][]byte{manifestPath: data}, rules: map[string]string{}}
	source := os.DirFS(root)
	for _, skill := range c.Skills {
		if err := b.add(source, path.Join("skills", skill)); err != nil {
			return nil, fmt.Errorf("skill %s: %w", skill, err)
		}
	}
	for _, name := range c.Rules {
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return nil, err
		}
		b.rules[name] = string(data)
	}
	head, err := fs.ReadFile(source, rulesHeadPath)
	if err != nil {
		return nil, err
	}
	b.rulesHead = string(head)
	skills, err := fs.ReadDir(source, "skills")
	if err != nil {
		return nil, err
	}
	for _, skill := range skills {
		// The rules skill is bundled, built from chat_bundle.rules.
		if skill.IsDir() && skill.Name() != rulesSkill && !slices.Contains(c.Skills, skill.Name()) {
			b.others = append(b.others, skill.Name())
		}
	}
	if err := b.addScan(source, cfg); err != nil {
		return nil, err
	}
	return b, nil
}

// manifest is the kit's plugin.json with the chat description, and the
// version it carries. The name stays the kit's: a same-named local plugin
// outranks a synced one, so a machine with the full kit never loads this.
func manifest(root, description string) ([]byte, string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifestPath)))
	if err != nil {
		return nil, "", err
	}
	var m struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		Author      struct {
			Name string `json:"name"`
		} `json:"author"`
		Repository string `json:"repository"`
		License    string `json:"license"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, "", fmt.Errorf("%s: %w", manifestPath, err)
	}
	if m.Name == "" || m.Version == "" {
		return nil, "", fmt.Errorf("%s: needs a name and a version", manifestPath)
	}
	m.Description = description
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return append(out, '\n'), m.Version, nil
}

// add copies the Markdown under dir, and only that: a .DS_Store or an
// editor's swap file would change the zip's bytes.
func (b *bundle) add(source fs.FS, dir string) error {
	return fs.WalkDir(source, dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".md" {
			return err
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return err
		}
		b.files[name] = data
		return nil
	})
}

// problems is every claude.ai upload rule the bundle breaks. claude plugin
// validate can't stand in: it checks what Claude Code loads, not this.
func (b *bundle) problems(c config.ChatBundle) []error {
	var errs []error
	if c.Description == "" {
		errs = append(errs, errors.New("chat_bundle.description is empty"))
	}
	if utf8.RuneCountInString(c.Description) > maxPluginDescription {
		errs = append(errs, fmt.Errorf("chat_bundle.description is over %d characters", maxPluginDescription))
	}
	if len(c.Skills) == 0 {
		errs = append(errs, errors.New("chat_bundle.skills is empty"))
	}
	if len(c.Rules) == 0 {
		errs = append(errs, errors.New("chat_bundle.rules is empty"))
	}
	for _, skill := range append(slices.Clone(c.Skills), rulesSkill) {
		errs = append(errs, skillProblems(skill, b.files[skillFile(skill)])...)
	}
	return errs
}

func skillFile(skill string) string { return path.Join("skills", skill, "SKILL.md") }

func skillProblems(skill string, data []byte) []error {
	file := skillFile(skill)
	block := frontmatter(string(data))
	if block == "" {
		return []error{fmt.Errorf("%s: no frontmatter", file)}
	}
	var front struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(block), &front); err != nil {
		return []error{fmt.Errorf("%s: frontmatter: %w", file, err)}
	}
	var errs []error
	if front.Name != skill {
		errs = append(errs, fmt.Errorf("%s: name %q doesn't match its folder", file, front.Name))
	}
	if front.Description == "" {
		errs = append(errs, fmt.Errorf("%s: no description", file))
	}
	if utf8.RuneCountInString(front.Description) > maxSkillDescription {
		errs = append(errs, fmt.Errorf("%s: description is over %d characters", file, maxSkillDescription))
	}
	if xmlTag.MatchString(front.Name) || xmlTag.MatchString(front.Description) {
		errs = append(errs, fmt.Errorf("%s: XML tag in the name or description", file))
	}
	if bytes.Count(data, []byte("\n")) >= maxSkillLines {
		errs = append(errs, fmt.Errorf("%s: %d lines or more", file, maxSkillLines))
	}
	return errs
}

// frontmatter is a file's YAML block, "" when it has none. Its --- lines
// stay: YAML reads them as document markers.
func frontmatter(text string) string {
	var block strings.Builder
	for line := range md.Lines(text) {
		if line.Kind != md.Front {
			break
		}
		block.WriteString(line.Text)
	}
	return block.String()
}

// write puts the zip at out: entries sorted and stamped alike, so the same
// files give the same bytes.
func (b *bundle) write(out string) error {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(maps.Keys(b.files)) {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: zipTime}
		header.SetMode(0o644)
		f, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := f.Write(b.files[name]); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	return fsx.WriteAtomic(out, buf.Bytes(), 0o644)
}
