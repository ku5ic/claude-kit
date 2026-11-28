package bashguard

import (
	"strings"

	"github.com/ku5ic/claude-kit/go/internal/layout"
)

// Downloads land only in scratch (skills/kit-rules/reference/tooling.md): every output file and
// directory, and every > redirect, must be a scratch path.

func (c *command) blockDownload(tool, target, flag string) error {
	return c.block(tool+" would write '"+target+"' outside scratch\n"+layout.KV("do", tool+" "+flag+" \"$(kit scratch-dir)/<name>\"; downloads go only to scratch"), "download-to-repo")
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
		if err := c.block("curl -O/-J writes a server-named file into the current directory\n"+layout.KV("do", "curl -o \"$(kit scratch-dir)/<name>\""), "download-to-repo"); err != nil {
			return err
		}
	}
	return c.checkTargets("curl", "-o", targets)
}

// curlTargets reads curl's words for the files it writes: -o and
// --output-dir values, and whether -O/-J names one after the server.
func curlTargets(words []string) (targets []string, remote bool, outDir string) {
	want := curlNone
	for _, w := range words {
		if want != curlNone {
			targets = append(targets, w)
			if want == curlDir {
				outDir = w
			}
			want = curlNone
			continue
		}
		if w == "--" {
			break
		}
		o := curlOption(w)
		remote, want = remote || o.remote, o.want
		if o.target != "" {
			targets = append(targets, o.target)
		}
		if o.dir {
			outDir = o.target
		}
	}
	return targets, remote, outDir
}

// curlWant is what curl's next word is: nothing it writes, an output file,
// or the output directory.
type curlWant int

const (
	curlNone curlWant = iota
	curlFile
	curlDir
)

// curlWord is what one curl word says about the files it writes: a
// server-named file, a target given in the word (dir when it's the output
// directory), or one the next word gives.
type curlWord struct {
	remote, dir bool
	target      string
	want        curlWant
}

// curlOption reads one curl word.
func curlOption(w string) curlWord {
	switch {
	case w == "--remote-name" || w == "--remote-name-all" || w == "--remote-header-name":
		return curlWord{remote: true}
	case w == "--output":
		return curlWord{want: curlFile}
	case w == "--output-dir":
		return curlWord{want: curlDir}
	case strings.HasPrefix(w, "--output="):
		return curlWord{target: strings.TrimPrefix(w, "--output=")}
	case strings.HasPrefix(w, "--output-dir="):
		return curlWord{target: strings.TrimPrefix(w, "--output-dir="), dir: true}
	case strings.HasPrefix(w, "--"):
		return curlWord{}
	case strings.HasPrefix(w, "-"):
		return curlCluster(w)
	}
	return curlWord{}
}

// curlCluster reads a short cluster: O and J are flags, o takes the rest of
// the word or the next one, and any other value-taking option (-XPOST,
// -Hx) ends the scan: the rest of the word is its value.
func curlCluster(w string) curlWord {
	var o curlWord
	for j := 1; j < len(w); j++ {
		switch f := w[j]; {
		case f == 'O' || f == 'J':
			o.remote = true
		case f == 'o' && j == len(w)-1:
			o.want = curlFile
			return o
		case f == 'o':
			o.target = w[j+1:]
			return o
		case strings.IndexByte("AbcCdDeEFHKmPQrtTuUwxXyYz", f) >= 0:
			return o
		}
	}
	return o
}

// wget writes into cwd by default, so an output flag is mandatory; -O - is
// stdout.
func (c *command) wget() error {
	targets, hasOut := wgetTargets(c.values())
	if !hasOut {
		if err := c.block("wget writes into the current directory by default\n"+layout.KV("do", "wget -P \"$(kit scratch-dir)\" <url>"), "download-to-repo"); err != nil {
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
			flag, rest := wgetCluster(w)
			switch {
			case rest != "":
				targets, hasOut = append(targets, rest), true
			case flag == 'O':
				wantDoc = true
			case flag == 'P':
				wantDir = true
			}
		}
	}
	return targets, hasOut
}

// wgetCluster reads a short cluster like -qO- or -qP dir: its first O or P
// flag, 0 for none, and the rest of the word, that flag's value when it
// isn't empty; empty, the next word is.
func wgetCluster(w string) (flag byte, rest string) {
	if j := strings.IndexAny(w[1:], "OP"); j >= 0 {
		return w[1+j], w[2+j:]
	}
	return 0, ""
}

// scratchVarTarget is true for a download target that is one quoted scratch
// variable and maybe a path ("$S/a"), as rm -r takes it, whose path stays in
// scratch with its last link followed, since curl writes through it. A
// fresh mktemp -d dir has no path to follow, and a link the same command
// makes there can't be checked, so it isn't one.
func (st *state) scratchVarTarget(w Word) bool {
	m := quotedVar.FindStringSubmatch(w.Raw)
	if m == nil || strings.Contains(m[4], "..") || !st.scratchVar(m[1]+m[2], 1) || !st.nonEmpty(m[1]+m[2], m[3] != "", st.at) {
		return false
	}
	path, _ := st.varPath(m[1]+m[2], 1)
	return path != "" && st.onDisk(path+m[4], true)
}

// checkTargets blocks every download target, flag value or > redirect,
// outside scratch, and asks about one behind a variable it can't resolve.
func (c *command) checkTargets(tool, flag string, targets []string) error {
	words := map[string]Word{}
	for _, w := range c.args {
		words[w.Value] = w
	}
	for _, r := range c.redirs {
		words[r.Target.Value] = r.Target
	}
	for _, t := range append(targets, c.redirectTargets()...) {
		if w, ok := words[t]; ok && c.st.scratchVarTarget(w) || c.st.scratchTarget(t, true) {
			continue
		}
		if strings.ContainsAny(t, "$`") && !strings.Contains(t, "..") {
			c.st.ask(tool+" writes to '"+t+"', and guard-bash can't tell whether that's in scratch; confirm it is", "download-target")
			continue
		}
		if err := c.blockDownload(tool, t, flag); err != nil {
			return err
		}
	}
	return nil
}
