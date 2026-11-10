package pkgcheck

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/config"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestUndeclaredGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.26\n\nrequire (\n\tgithub.com/known/lib v1.0.0\n\tgolang.org/x/sync v0.1.0 // indirect\n)\n\nreplace github.com/old/thing => ../thing\n")
	path := filepath.Join(root, "cmd", "main.go")
	src := func(imports ...string) string {
		var b strings.Builder
		b.WriteString("package main\n\nimport (\n")
		for _, i := range imports {
			fmt.Fprintf(&b, "\t%q\n", i)
		}
		b.WriteString(")\n")
		return b.String()
	}
	for _, tc := range []struct {
		name          string
		before, after string
		want          []string
	}{
		{"an undeclared module", src("fmt"), src("fmt", "github.com/made/up/pkg"), []string{"github.com/made/up/pkg"}},
		{"the stdlib, a requirement, its subpackage, our module, a replace", src(), src("net/http", "github.com/known/lib", "github.com/known/lib/sub", "golang.org/x/sync/errgroup", "example.com/app/internal/x", "github.com/old/thing"), nil},
		{"an import that was already there", src("github.com/made/up/pkg"), src("github.com/made/up/pkg", "fmt"), nil},
		{"a file that doesn't parse", src(), "package main\nimport (\n\t\"github.com/made/up/pkg\"\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Undeclared(nil, path, tc.before, tc.after); !slices.Equal(got, tc.want) {
				t.Errorf("Undeclared = %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("a go.work module", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		testutil.Write(t, filepath.Join(ws, "go.work"), "go 1.26\n\nuse (\n\t./app\n\t./shared\n)\n")
		testutil.Write(t, filepath.Join(ws, "app", "go.mod"), "module example.com/app\n")
		testutil.Write(t, filepath.Join(ws, "shared", "go.mod"), "module example.com/shared\n")
		if got := Undeclared(nil, filepath.Join(ws, "app", "main.go"), src(), src("example.com/shared/x")); got != nil {
			t.Errorf("Undeclared = %q, want none", got)
		}
	})
	t.Run("no go.mod: nothing to check against", func(t *testing.T) {
		t.Parallel()
		if got := Undeclared(nil, filepath.Join(t.TempDir(), "main.go"), src(), src("github.com/made/up/pkg")); got != nil {
			t.Errorf("Undeclared = %q, want none", got)
		}
	})
}

func TestUndeclaredJS(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "package.json"), `{"name":"root","devDependencies":{"vitest":"^3"}}`)
	app := filepath.Join(root, "packages", "app")
	testutil.Write(t, filepath.Join(app, "package.json"), `{"name":"@acme/app","dependencies":{"react":"^19","@tanstack/react-query":"^5"}}`)
	testutil.Mkdir(t, filepath.Join(app, "src", "components"))
	testutil.Write(t, filepath.Join(app, "tsconfig.json"), `{"compilerOptions": {"paths": {"@ui/*": ["src/ui/*"]}}}`)
	testutil.Write(t, filepath.Join(root, "vite.config.ts"), "export default { resolve: { alias: { 'shared': '/src/shared' } } }")
	path := filepath.Join(app, "src", "App.tsx")
	for _, tc := range []struct {
		name, before, after string
		want                []string
	}{
		{"an undeclared package, scoped and not", "", "import x from 'made-up-lib'\nimport { y } from \"@made/up/sub\"\nconst z = require('another-one')\nconst w = await import('made-up-lib/deep')",
			[]string{"made-up-lib", "@made/up", "another-one"}},
		{"declared, hoisted, builtin, relative, aliased", "", "import React from 'react'\nimport { useQuery } from '@tanstack/react-query'\nimport { test } from 'vitest'\nimport fs from 'node:fs'\nimport path from 'path'\nimport a from './a'\nimport b from '@/lib/b'\nimport c from '~/c'\nimport Button from 'components/Button'\nimport self from '@acme/app/x'\nimport v from 'virtual:icons'\nimport { Card } from '@ui/Card'\nimport s from 'shared/s'",
			nil},
		{"an import already there", "import x from 'made-up-lib'", "import x from 'made-up-lib'\nconst a = 1", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Undeclared(nil, path, tc.before, tc.after); !slices.Equal(got, tc.want) {
				t.Errorf("Undeclared = %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("a skipped directory", func(t *testing.T) {
		t.Parallel()
		if got := Undeclared([]string{"node_modules"}, filepath.Join(app, "node_modules", "x", "i.js"), "", "import 'made-up-lib'"); got != nil {
			t.Errorf("Undeclared = %q, want none", got)
		}
	})
}

// registry is a lookup command answering from names: found when the name
// is in exists, else the npm 404 text, or other text when it's "offline".
func registry(t *testing.T, exists ...string) []string {
	t.Helper()
	dir := t.TempDir()
	stub := filepath.Join(dir, "lookup")
	script := `case "$1" in`
	for _, e := range exists {
		script += fmt.Sprintf("\n  %q) echo found; exit 0;;", e)
	}
	script += "\n  offline*) echo 'getaddrinfo ENOTFOUND registry'; exit 1;;\n  slow*) sleep 5; exit 0;;\n  *) echo 'npm error code E404'; echo 'no matching versions'; exit 1;;\nesac"
	testutil.FakeTool(t, stub, filepath.Join(dir, "calls"), script)
	return []string{stub, "{pkg}"}
}

func TestLookupsMissing(t *testing.T) {
	t.Parallel()
	conf := func(cmd []string) config.PackageLookups {
		return config.PackageLookups{Timeout: 2, Ecosystems: map[string]config.PackageLookup{
			"npm": {Managers: []string{"npm", "pnpm"}, Name: `^(@[a-z0-9][\w.-]*/)?[a-z0-9][\w.-]*$`, Cmd: cmd, Missing: `\bE404\b`},
			"go":  {Managers: []string{"go"}, Name: `^[a-z0-9.-]+\.[a-z]{2,}(/[\w.~-]+)+$`, Cmd: cmd, Missing: `no matching versions`, Parents: true},
		}}
	}
	cmd := registry(t, "react", "@types/node", "golang.org/x/tools")
	for _, tc := range []struct {
		manager, operand string
		want             bool
	}{
		{"pnpm", "react", false},
		{"pnpm", "react@19.1.0", false},
		{"pnpm", "@types/node@22", false},
		{"pnpm", "made-up-lib", true},
		{"pnpm", "offline-lib", false},                   // a failure that isn't a 404
		{"pnpm", "./local-dir", false},                   // a path, not a name
		{"pnpm", "github:user/repo", false},              // a protocol spec
		{"pnpm", "user/repo", false},                     // not an npm name
		{"go", "golang.org/x/tools/cmd/stringer", false}, // a package inside a module
		{"go", "github.com/made/up/pkg@v1.2.0", true},
		{"pip", "made-up-lib", false}, // no ecosystem covers pip
	} {
		t.Run(tc.manager+" "+tc.operand, func(t *testing.T) {
			t.Parallel()
			if got := NewLookups(conf(cmd), t.TempDir(), t.TempDir()).Missing(tc.manager, tc.operand); got != tc.want {
				t.Errorf("Missing = %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("a timeout is not missing", func(t *testing.T) {
		t.Parallel()
		c := conf(cmd)
		c.Timeout = 1
		if NewLookups(c, t.TempDir(), t.TempDir()).Missing("pnpm", "slow-lib") {
			t.Error("a lookup that timed out counted as missing")
		}
	})
	t.Run("timeout 0 turns lookups off", func(t *testing.T) {
		t.Parallel()
		c := conf(cmd)
		c.Timeout = 0
		if NewLookups(c, t.TempDir(), t.TempDir()).Missing("pnpm", "made-up-lib") {
			t.Error("a lookup ran with timeout 0")
		}
	})
	t.Run("a workspace package is never looked up", func(t *testing.T) {
		t.Parallel()
		root := testutil.Physical(t, t.TempDir())
		testutil.Git(t, root, "init", "-q")
		testutil.Write(t, filepath.Join(root, "apps", "web", "package.json"), `{"name":"my-app"}`)
		if NewLookups(conf(cmd), root, root).Missing("pnpm", "my-app") {
			t.Error("a workspace package counted as missing")
		}
	})
	t.Run("a package an .npmrc sends to another registry is never looked up", func(t *testing.T) {
		t.Parallel()
		c := conf(cmd)
		npm := c.Ecosystems["npm"]
		npm.RCFile = ".npmrc"
		c.Ecosystems["npm"] = npm
		root := t.TempDir()
		dir := filepath.Join(root, "apps", "web")
		testutil.Write(t, filepath.Join(root, ".npmrc"), "# private\n@acme:registry = https://npm.acme.dev\n")
		testutil.Mkdir(t, dir)
		for operand, want := range map[string]bool{"@acme/ui": false, "@other/ui": true, "made-up-lib": true} {
			if got := NewLookups(c, dir, root).Missing("pnpm", operand); got != want {
				t.Errorf("Missing(%s) = %v, want %v", operand, got, want)
			}
		}
		testutil.Write(t, filepath.Join(dir, ".npmrc"), "registry=https://npm.acme.dev\n")
		if NewLookups(c, dir, root).Missing("pnpm", "made-up-lib") {
			t.Error("a package under a project-wide registry was looked up")
		}
	})
}
