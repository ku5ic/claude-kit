package gapfill

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/git"
	"github.com/ku5ic/claude-kit/go/internal/sources"
)

// view is what the model sees of the project besides its entries: its
// file names, shallowest first, the text of its config files, and its
// lockfile directories. Key names the facts answered for it.
type view struct {
	Files        []string          `json:"files"`
	Contents     map[string]string `json:"contents"`
	LockfileDirs []lockDir         `json:"lockfile_dirs"`
	Key          string            `json:"-"`
}

// lockDir is a directory with a lockfile, or a package.json naming its
// packageManager.
type lockDir struct {
	Dir            string   `json:"dir"`
	Lockfiles      []string `json:"lockfiles,omitempty"`
	PackageManager string   `json:"package_manager,omitempty"`
}

const (
	maxFiles     = 300
	maxFileBytes = 8 << 10
	maxContents  = 64 << 10
)

// configExts are the structured formats tools keep their config and
// dependencies in.
var configExts = []string{".json", ".toml", ".yaml", ".yml", ".ini", ".cfg", ".mod"}

// readView reads root for the model. Contents hold the entries' own
// config files, then each config-format file and root dotfile at the root
// or beside a lockfile: what states a tool or a dependency. Source files
// never do, so editing one never asks again: the key covers the entries,
// the contents, and the lockfile directories, not the file list.
func readView(cfg *config.Config, root string, entries []sources.Entry) view {
	files, err := git.Files(root)
	if err != nil || len(files) == 0 {
		files = topLevel(root)
	}
	slices.SortStableFunc(files, func(a, b string) int { return cmp.Compare(strings.Count(a, "/"), strings.Count(b, "/")) })
	p := view{Files: files[:min(len(files), maxFiles)], Contents: map[string]string{}, LockfileDirs: lockDirs(cfg, root, files)}

	var candidates []string
	for _, e := range entries {
		candidates = append(candidates, e.File)
	}
	evidenceDirs := []string{"."}
	for _, d := range p.LockfileDirs {
		evidenceDirs = append(evidenceDirs, d.Dir)
	}
	for _, f := range files {
		dir, name := filepath.Dir(f), filepath.Base(f)
		if slices.Contains(evidenceDirs, dir) && (slices.Contains(configExts, filepath.Ext(name)) || dir == "." && strings.HasPrefix(name, ".")) {
			candidates = append(candidates, f)
		}
	}
	total := 0
	for _, f := range candidates {
		if _, seen := p.Contents[f]; seen || f == "" || lockfile(cfg, f) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil || len(data) > maxFileBytes || total+len(data) > maxContents || git.IsBinary(data) {
			continue
		}
		p.Contents[f] = string(data)
		total += len(data)
	}

	keys := []string{promptVersion}
	for _, e := range entries {
		keys = append(keys, Key(e))
	}
	slices.Sort(keys)
	facts, _ := json.Marshal(struct {
		Keys     []string
		Contents map[string]string
		Dirs     []lockDir
	}{keys, p.Contents, p.LockfileDirs})
	sum := sha256.Sum256(facts)
	p.Key = hex.EncodeToString(sum[:8])
	return p
}

// topLevel is root's own file names, for a project outside git.
func topLevel(root string) []string {
	entries, _ := os.ReadDir(root)
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			out = append(out, e.Name())
		}
	}
	return out
}

// lockfile is true when file's name matches a lockfile_globs pattern.
func lockfile(cfg *config.Config, file string) bool {
	return slices.ContainsFunc(cfg.LockfileGlobs, func(glob string) bool {
		ok, _ := filepath.Match(glob, filepath.Base(file))
		return ok
	})
}

// lockDirs groups files' lockfiles by directory, and adds each directory
// whose package.json names a packageManager, the root first.
func lockDirs(cfg *config.Config, root string, files []string) []lockDir {
	var out []lockDir
	index := map[string]int{}
	add := func(dir string) *lockDir {
		if i, ok := index[dir]; ok {
			return &out[i]
		}
		index[dir] = len(out)
		out = append(out, lockDir{Dir: dir, PackageManager: sources.JSONValue(filepath.Join(root, dir, "package.json"), ".packageManager")})
		return &out[len(out)-1]
	}
	if pm := sources.JSONValue(filepath.Join(root, "package.json"), ".packageManager"); pm != "" {
		add(".")
	}
	for _, f := range files {
		if lockfile(cfg, f) {
			d := add(filepath.Dir(f))
			d.Lockfiles = append(d.Lockfiles, filepath.Base(f))
		}
	}
	return out
}
