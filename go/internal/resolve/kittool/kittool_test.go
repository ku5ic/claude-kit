package kittool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// onPath makes dir the whole PATH and writes each script into it.
func onPath(t *testing.T, scripts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	for name, body := range scripts {
		onPathIn(t, dir, name, body)
	}
	return dir
}

func TestAToolOnlyOnPATHIsFound(t *testing.T) {
	dir := onPath(t, map[string]string{"gitleaks": ""})
	if got, err := Path("gitleaks"); err != nil || got != filepath.Join(dir, "gitleaks") {
		t.Errorf("Path(gitleaks) = %q, %v", got, err)
	}
}

func TestTheFirstNameInstalledWins(t *testing.T) {
	dir := onPath(t, map[string]string{"prettier": "", "prettierd": ""})
	if got, err := Path("prettierd", "prettier"); err != nil || got != filepath.Join(dir, "prettierd") {
		t.Errorf("Path(prettierd, prettier) = %q, %v", got, err)
	}
	if _, err := Path("tsc", "deno"); err == nil || !strings.Contains(err.Error(), "tsc or deno isn't installed") {
		t.Errorf("Path with none installed: err %v", err)
	}
}

func TestSgCountsOnlyWhenItIsAstGrep(t *testing.T) {
	dir := onPath(t, map[string]string{"sg": "echo \"sg: group '$1' does not exist\" >&2\nexit 1\n"})
	if got, err := Path("ast-grep"); err == nil {
		t.Errorf("newgrp's sg resolved as ast-grep: %q", got)
	}
	onPathIn(t, dir, "sg", "echo 'ast-grep 0.51.2'\n")
	if got, err := Path("ast-grep"); err != nil || got != filepath.Join(dir, "sg") {
		t.Errorf("Path(ast-grep) = %q, %v", got, err)
	}
}

func TestFdInstalledAsFdfindIsFound(t *testing.T) {
	dir := onPath(t, map[string]string{"fdfind": ""})
	if got, err := Path("fd"); err != nil || got != filepath.Join(dir, "fdfind") {
		t.Errorf("Path(fd) = %q, %v", got, err)
	}
}

// onPathIn rewrites one script in an existing PATH dir.
func onPathIn(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}
