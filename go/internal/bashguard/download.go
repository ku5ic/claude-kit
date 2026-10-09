package bashguard

import (
	"regexp"
	"slices"
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/project"
)

// Downloads land only in scratch (rules/tooling.md): every output file and
// directory, and every > redirect, must be a scratch path.

func (c *command) blockDownload(tool, target, flag string) error {
	return c.block(tool+" would write '"+target+"' outside scratch; downloads go only to scratch: "+tool+" "+flag+" \"$(kit scratch-dir)/<name>\"", "download-to-repo")
}

func (c *command) redirectTargets() []string {
	var out []string
	for _, r := range c.redirs {
		out = append(out, r.Target.Value)
	}
	return out
}

// curl: -O/-J name the file after the server and write to cwd, unless
// --output-dir says otherwise.
func (c *command) curl() error {
	targets, remote, outDir := curlTargets(c.values())
	if remote && outDir == "" {
		if err := c.block("curl -O/-J writes a server-named file into the current directory; use: curl -o \"$(kit scratch-dir)/<name>\"", "download-to-repo"); err != nil {
			return err
		}
	}
	return c.checkTargets("curl", "-o", targets)
}

// curlTargets reads curl's words for the files it writes: -o and
// --output-dir values, and whether -O/-J names one after the server.
func curlTargets(words []string) (targets []string, remote bool, outDir string) {
	wantTarget := 0
	for _, w := range words {
		if wantTarget > 0 {
			targets = append(targets, w)
			if wantTarget == 2 {
				outDir = w
			}
			wantTarget = 0
			continue
		}
		switch {
		case w == "--":
			return targets, remote, outDir
		case w == "--remote-name" || w == "--remote-name-all" || w == "--remote-header-name":
			remote = true
		case w == "--output":
			wantTarget = 1
		case w == "--output-dir":
			wantTarget = 2
		case strings.HasPrefix(w, "--output="):
			targets = append(targets, strings.TrimPrefix(w, "--output="))
		case strings.HasPrefix(w, "--output-dir="):
			outDir = strings.TrimPrefix(w, "--output-dir=")
			targets = append(targets, outDir)
		case strings.HasPrefix(w, "--"):
		case strings.HasPrefix(w, "-"):
			// A short cluster: O and J are flags, o takes the rest of the
			// word or the next one, and any other value-taking option
			// (-XPOST, -Hx) ends the scan: the rest of the word is its value.
		cluster:
			for j := 1; j < len(w); j++ {
				switch f := w[j]; {
				case f == 'O' || f == 'J':
					remote = true
				case f == 'o':
					if j == len(w)-1 {
						wantTarget = 1
					} else {
						targets = append(targets, w[j+1:])
					}
					break cluster
				case strings.IndexByte("AbcCdDeEFHKmPQrtTuUwxXyYz", f) >= 0:
					break cluster
				}
			}
		}
	}
	return targets, remote, outDir
}

// wget writes into cwd by default, so an output flag is mandatory; -O - is
// stdout.
func (c *command) wget() error {
	targets, hasOut := wgetTargets(c.values())
	if !hasOut {
		if err := c.block("wget writes into the current directory by default; use: wget -P \"$(kit scratch-dir)\" <url>", "download-to-repo"); err != nil {
			return err
		}
	}
	return c.checkTargets("wget", "-O", targets)
}

// wgetTargets reads wget's words for the -O document and -P directory it
// writes, and whether any output flag is there.
func wgetTargets(words []string) (targets []string, hasOut bool) {
	wantDoc, wantDir := false, false
	for _, w := range words {
		if wantDoc || wantDir {
			targets = append(targets, w)
			wantDoc, wantDir, hasOut = false, false, true
			continue
		}
		switch {
		case w == "--":
			return targets, hasOut
		case strings.HasPrefix(w, "--output-document=") || strings.HasPrefix(w, "--directory-prefix="):
			targets = append(targets, w[strings.IndexByte(w, '=')+1:])
			hasOut = true
		case w == "--output-document":
			wantDoc = true
		case w == "--directory-prefix":
			wantDir = true
		case strings.HasPrefix(w, "--"):
		case strings.HasPrefix(w, "-"):
			// A short cluster like -qO- or -qP dir: O or P takes the rest of
			// the word as its value, or the next word when nothing follows.
			for j := 1; j < len(w); j++ {
				if w[j] != 'O' && w[j] != 'P' {
					continue
				}
				if rest := w[j+1:]; rest != "" {
					targets = append(targets, rest)
					hasOut = true
				} else if w[j] == 'O' {
					wantDoc = true
				} else {
					wantDir = true
				}
				break
			}
		}
	}
	return targets, hasOut
}

// checkTargets blocks every download target, flag value or > redirect,
// outside scratch, and asks about one behind a variable it can't resolve.
func (c *command) checkTargets(tool, flag string, targets []string) error {
	for _, t := range append(targets, c.redirectTargets()...) {
		if c.st.scratchTarget(t) {
			continue
		}
		if strings.ContainsAny(t, "$`") && !strings.Contains(t, "..") {
			c.st.ask(tool + " writes to '" + t + "', and guard-bash can't tell whether that's in scratch; confirm it is")
			continue
		}
		if err := c.blockDownload(tool, t, flag); err != nil {
			return err
		}
	}
	return nil
}

// pmDir is the directory a package manager works in: its own directory
// flag, else the segment's cwd. Only a long flag takes =value.
func (c *command) pmDir(dirFlags []string) string {
	dir, wantDir := c.st.cwd, false
	for _, w := range c.values() {
		if wantDir {
			dir, wantDir = resolveDir(c.st.home, c.st.cwd, w), false
			continue
		}
		flag, value, hasValue := strings.Cut(w, "=")
		switch {
		case slices.Contains(dirFlags, w):
			wantDir = true
		case hasValue && slices.Contains(dirFlags, flag) && strings.HasPrefix(flag, "--"):
			dir = resolveDir(c.st.home, c.st.cwd, value)
		}
	}
	return dir
}

// packageManager guards installs: pnpm install without --frozen-lockfile
// asks, global installs block, and a manager other than the one the nearest
// lockfile of its own ecosystem names blocks with the rerun command.
func (c *command) packageManager() error {
	rest := strings.TrimLeft(c.rest, " ")
	if c.name == "pnpm" && pnpmInstall.MatchString(rest) && !frozen.MatchString(c.text) {
		c.st.ask("pnpm install without --frozen-lockfile can change the lockfile; confirm before running")
	}
	pm, _ := c.st.cfg.Manager(c.name)
	if pm.GlobalInstall != "" {
		// An unreadable pattern blocks: this is a guard.
		global, err := regexp.Compile(pm.GlobalInstall)
		if err != nil || global.MatchString(rest) {
			if err := c.block("global package install. Use a project-local install or asdf shim.", "pkg-global-install"); err != nil {
				return err
			}
		}
	}
	// --version and -v never touch project files.
	if rest == "--version" || rest == "-v" {
		return nil
	}
	if pm.Ecosystem == "" || pm.Ecosystem == "none" {
		return nil
	}
	dir := c.pmDir(pm.DirFlags)
	// Greenfield (no lockfile in this ecosystem) is always allowed.
	lock, ok := project.NearestLockfile(c.st.cfg, dir, pm.Ecosystem)
	if !ok || lock.Manager == pm.Manager {
		return nil
	}
	// A dlx command (npx) suggests the lockfile manager's own.
	other, _ := c.st.cfg.Manager(lock.Manager)
	suggest := lock.Manager
	if c.name == pm.Dlx && other.Dlx != "" {
		suggest = other.Dlx
	}
	return c.block("this repo uses "+lock.Manager+" ("+lock.File+"); rerun as: "+c.rerun(pm, other, suggest), "pm-mismatch")
}

// rerun is the command in the lockfile manager's own terms: its directory
// flag (or a cd when it has none) and its verbs for add and install.
func (c *command) rerun(pm, other config.PackageManager, name string) string {
	words := strings.Fields(c.rest)
	var out []string
	cd := ""
	dir := func(d string) {
		if len(other.DirFlags) == 0 {
			cd = "cd " + d + " && "
			return
		}
		out = append(out, other.DirFlags[0], d)
	}
	verbAt := -1
	for i := 0; i < len(words); i++ {
		w := words[i]
		flag, value, hasValue := strings.Cut(w, "=")
		switch {
		case slices.Contains(pm.DirFlags, w) && i+1 < len(words):
			i++
			dir(words[i])
		case hasValue && slices.Contains(pm.DirFlags, flag):
			dir(value)
		default:
			if verbAt < 0 && !strings.HasPrefix(w, "-") {
				verbAt = len(out)
			}
			out = append(out, w)
		}
	}
	if verbAt >= 0 && slices.Contains([]string{"install", "i", "add"}, out[verbAt]) {
		hasPackage := slices.ContainsFunc(out[verbAt+1:], func(w string) bool { return !strings.HasPrefix(w, "-") })
		switch {
		case hasPackage && other.AddVerb != "":
			out[verbAt] = other.AddVerb
		case !hasPackage && other.SyncVerb != "":
			out[verbAt] = other.SyncVerb
		}
	}
	return cd + strings.Join(append([]string{name}, out...), " ")
}
