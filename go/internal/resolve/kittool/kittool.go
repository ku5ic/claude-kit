// Package kittool is where a PATH-only copy runs: a tool the kit runs for
// itself, or an on_edit formatter, which no project pins (ADR 0030). It
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

// Path is the PATH copy of the first of names installed.
func Path(names ...string) (string, error) {
	for _, name := range names {
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
