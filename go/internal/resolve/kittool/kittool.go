// Package kittool is where a PATH-only copy runs: a tool the kit runs for
// itself, or an on_edit formatter, which no project pins (ADR 0004). It
// sits under resolve but below every layer, so any package can reach it; a
// project's own commands go through resolve's Bin.
package kittool

import (
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/proc"
)

// versionTimeout bounds the --version run that tells ast-grep's sg from
// shadow-utils' newgrp alias of the same name.
const versionTimeout = 5 * time.Second

// aliases are the other names a tool installs as: ast-grep's deprecated sg,
// and fd's fdfind on Debian and Ubuntu.
var aliases = map[string][]string{"ast-grep": {"sg"}, "fd": {"fdfind"}}

// Path is the PATH copy of the first of names installed, each tried under
// its aliases after its own name.
func Path(names ...string) (string, error) {
	var tried []string
	for _, name := range names {
		tried = append(append(tried, name), aliases[name]...)
	}
	for _, name := range tried {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if name == "sg" && !prints(path, "ast-grep") {
			continue
		}
		return path, nil
	}
	return "", errors.New(strings.Join(names, " or ") + " isn't installed")
}

// prints is true when path's --version output names want.
func prints(path, want string) bool {
	out, err := proc.Command(versionTimeout, path, "--version").CombinedOutput()
	return err == nil && strings.Contains(string(out), want)
}
