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
