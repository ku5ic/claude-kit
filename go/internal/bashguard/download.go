package bashguard

import (
	"strings"
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
	remote, wantTarget, outDir := false, 0, ""
	var targets []string
	for _, w := range c.values() {
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
			goto done
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
done:
	if remote && outDir == "" {
		if err := c.block("curl -O/-J writes a server-named file into the current directory; use: curl -o \"$(kit scratch-dir)/<name>\"", "download-to-repo"); err != nil {
			return err
		}
	}
	for _, t := range targets {
		if !c.st.scratchTarget(t) {
			if err := c.blockDownload("curl", t, "-o"); err != nil {
				return err
			}
		}
	}
	for _, t := range c.redirectTargets() {
		if !c.st.scratchTarget(t) {
			if err := c.blockDownload("curl", t, "-o"); err != nil {
				return err
			}
		}
	}
	return nil
}

// wget writes into cwd by default, so an output flag is mandatory; -O - is
// stdout.
func (c *command) wget() error {
	wantDoc, wantDir, hasOut := false, false, false
	var targets []string
	for _, w := range c.values() {
		if wantDoc || wantDir {
			targets = append(targets, w)
			wantDoc, wantDir, hasOut = false, false, true
			continue
		}
		switch {
		case w == "--":
			goto done
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
done:
	if !hasOut {
		if err := c.block("wget writes into the current directory by default; use: wget -P \"$(kit scratch-dir)\" <url>", "download-to-repo"); err != nil {
			return err
		}
	}
	for _, t := range targets {
		if !c.st.scratchTarget(t) {
			if err := c.blockDownload("wget", t, "-O"); err != nil {
				return err
			}
		}
	}
	for _, t := range c.redirectTargets() {
		if !c.st.scratchTarget(t) {
			if err := c.blockDownload("wget", t, "-O"); err != nil {
				return err
			}
		}
	}
	return nil
}

// packageManager guards installs: pnpm install without --frozen-lockfile
// asks, global installs block, and a manager other than the one the nearest
// lockfile of its own ecosystem names blocks with the rerun command.
func (c *command) packageManager() error {
	rest := strings.TrimLeft(c.rest, " ")
	if c.name == "pnpm" && pnpmInstall.MatchString(rest) && !frozen.MatchString(c.text) {
		c.st.ask("pnpm install without --frozen-lockfile can change the lockfile; confirm before running")
	}
	global := "global package install. Use a project-local install or asdf shim."
	switch {
	case (c.name == "npm" || c.name == "pnpm" || c.name == "yarn") && npmGlobal.MatchString(c.text),
		c.name == "yarn" && yarnGlobal.MatchString(c.text),
		c.name == "bun" && bunGlobal.MatchString(c.text):
		if err := c.block(global, "pkg-global-install"); err != nil {
			return err
		}
	}
	// --version and -v never touch project files.
	if rest == "--version" || rest == "-v" {
		return nil
	}
	invoked := map[string]string{"npx": "npm", "bunx": "bun", "pip3": "pip"}[c.name]
	if invoked == "" {
		invoked = c.name
	}
	ecosystem := ""
	for _, pm := range c.st.cfg.PackageManagers {
		if pm.Manager == invoked {
			ecosystem = pm.Ecosystem
			break
		}
	}
	if ecosystem == "" || ecosystem == "none" {
		return nil
	}
	// The manager's own directory flag overrides the segment's cwd.
	dir, wantDir := c.st.cwd, false
	dirFlags := map[string]bool{"uv:--directory": true, "uv:--project": true, "pnpm:--dir": true, "pnpm:-C": true, "yarn:--cwd": true, "npm:--prefix": true}
	for _, w := range c.values() {
		if wantDir {
			dir, wantDir = resolveDir(c.st.home, c.st.cwd, w), false
			continue
		}
		flag, value, hasValue := strings.Cut(w, "=")
		switch {
		case dirFlags[invoked+":"+w]:
			wantDir = true
		case hasValue && dirFlags[invoked+":"+flag] && flag != "-C":
			dir = resolveDir(c.st.home, c.st.cwd, value)
		}
	}
	// Greenfield (no lockfile in this ecosystem) is always allowed.
	lock, ok := c.st.nearestLockfile(dir, ecosystem)
	if !ok || lock.Manager == invoked {
		return nil
	}
	tail := c.rest
	suggest := lock.Manager + tail
	switch c.name {
	case "npx":
		suggest = map[string]string{"bun": "bunx", "pnpm": "pnpm dlx", "yarn": "yarn dlx"}[lock.Manager]
		if suggest == "" {
			suggest = lock.Manager
		}
		suggest += tail
	case "bunx":
		suggest = map[string]string{"npm": "npx", "pnpm": "pnpm dlx", "yarn": "yarn dlx"}[lock.Manager]
		if suggest == "" {
			suggest = lock.Manager
		}
		suggest += tail
	}
	return c.block("this repo uses "+lock.Manager+" ("+lock.File+"); rerun as: "+suggest, "pm-mismatch")
}
