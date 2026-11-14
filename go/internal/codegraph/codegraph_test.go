package codegraph

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ku5ic/claude-kit/go/internal/resolve/kittool"
	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

// fixtureRepo is testdata/repo as a git repository: deps reads its
// tracked files. Its Go files and go.mod are stored as *.fixture, so the
// go tool never builds them as this module's packages.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/repo")); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(dir, func(p string, _ fs.DirEntry, err error) error {
		if real, ok := strings.CutSuffix(p, ".fixture"); ok && err == nil {
			return os.Rename(p, real)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, dir, "init", "-q")
	testutil.Git(t, dir, "add", "-A")
	return dir
}

func TestImportsGolden(t *testing.T) {
	t.Parallel()
	if _, err := kittool.Path(astGrep...); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err) // CI installs ast-grep, so the rules always run there
		}
		t.Skip(err) // kit: allow ast-grep is optional on a dev machine
	}
	var out bytes.Buffer
	// vendor/ is a skip_dirs entry: third-party code, not the repo's.
	if err := Imports(fixtureRepo(t)+"/", []string{"vendor"}, &out); err != nil {
		t.Fatal(err)
	}
	if want := testutil.Read(t, "testdata/imports.golden"); out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

// A failed or slow scan says why: the start of ast-grep's stderr, or that
// the deadline killed it. Not parallel: it sets PATH and scanTimeout.
func TestImportsNamesAScanFailure(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	testutil.Stub(t, filepath.Join(bin, "ast-grep"), "echo 'Error: Cannot parse rule kotlin' >&2\necho '  unknown kind import_header' >&2\nexit 8\n")
	err := Imports(t.TempDir(), nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "exit status 8: Error: Cannot parse rule kotlin; unknown kind import_header") {
		t.Errorf("a failed scan: %v", err)
	}
	testutil.Stub(t, filepath.Join(bin, "ast-grep"), "exec sleep 5\n")
	defer func(d time.Duration) { scanTimeout = d }(scanTimeout)
	scanTimeout = 100 * time.Millisecond
	if err := Imports(t.TempDir(), nil, io.Discard); err == nil || !strings.Contains(err.Error(), "timed out after 100ms") {
		t.Errorf("a slow scan: %v", err)
	}
}

// The fixture covers each resolver: a relative path through ../, a
// NodeNext .js import of a .ts file, a Go module path, Rust crate:: with a
// {group} and mod, Python's from . import, declared and undeclared
// externals per ecosystem, a test file that doesn't vote, and one upward
// edge, and src and rust, modules with no edge to another. The out dir
// doesn't exist yet: Deps makes it.
func TestDepsGolden(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "raw")
	var stdout bytes.Buffer
	if err := Deps(fixtureRepo(t), "testdata/imports.golden", out, "", nil, &stdout); err != nil {
		t.Fatal(err)
	}
	if want := "split dir: (repo root); main source root: none; other candidates: 0\n"; stdout.String() != want {
		t.Errorf("summary: %q", stdout.String())
	}
	for _, name := range []string{"deps.tsv", "externals.tsv", "upward.tsv", "imports-resolved.tsv", "modules.txt"} {
		got := testutil.Read(t, filepath.Join(out, name))
		if want := testutil.Read(t, filepath.Join("testdata/deps", strings.TrimSuffix(name, filepath.Ext(name))+".golden")); got != want {
			t.Errorf("%s: got:\n%s\nwant:\n%s", name, got, want)
		}
	}
}

// A repo's own types package shadows the stdlib module, as it does on
// Python's path; a manifest that doesn't parse is named, not silently
// empty.
func TestDepsReadsTheRepoBeforePythonsStdlibAndNamesABadManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "app/main.py"), "import types\nimport json\n")
	testutil.Write(t, filepath.Join(root, "types/__init__.py"), "")
	testutil.Write(t, filepath.Join(root, "package.json"), "{not json")
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "add", "-A")
	imports := filepath.Join(t.TempDir(), "imports.tsv")
	testutil.Write(t, imports, "app/main.py\timport types\napp/main.py\timport json\n")
	out := filepath.Join(t.TempDir(), "raw")
	var stdout bytes.Buffer
	if err := Deps(root, imports, out, ".", nil, &stdout); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout.String(), "skipped manifest: package.json: ") {
		t.Errorf("summary:\n%s", stdout.String())
	}
	if got := testutil.Read(t, filepath.Join(out, "deps.tsv")); got != "1\tapp\ttypes\n" {
		t.Errorf("deps:\n%s", got)
	}
	if got := testutil.Read(t, filepath.Join(out, "externals.tsv")); !strings.Contains(got, "\tjson\t") || strings.Contains(got, "\ttypes\t") {
		t.Errorf("externals:\n%s", got)
	}
}

// A committed vendor/, a skip_dirs entry, is third-party code. Its files
// are no module, and an import into it counts as no edge.
func TestDepsLeavesASkippedDirOutOfTheModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "app/main.ts"), "import { padded } from \"../vendor/lib/index\";\n")
	testutil.Write(t, filepath.Join(root, "vendor/lib/index.ts"), "import pad from \"left-pad\";\n")
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "add", "-A")
	imports := filepath.Join(t.TempDir(), "imports.tsv")
	testutil.Write(t, imports, "app/main.ts\t\"../vendor/lib/index\"\nvendor/lib/index.ts\t\"left-pad\"\n")
	out := filepath.Join(t.TempDir(), "raw")
	if err := Deps(root, imports, out, "", []string{"vendor"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Read(t, filepath.Join(out, "deps.tsv")); strings.Contains(got, "vendor") {
		t.Errorf("deps:\n%s", got)
	}
	if got := testutil.Read(t, filepath.Join(out, "imports-resolved.tsv")); strings.Contains(got, "vendor/") {
		t.Errorf("imports-resolved:\n%s", got)
	}
}

// The log has a pair with an import, a Go file and a file of the package it
// imports, one without, a same-directory pair, a pair under the minimum, a
// bulk commit, and noise and test files.
func TestCouplingGolden(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := Coupling("testdata/gitlog.txt", "testdata/deps/imports-resolved.golden", &out); err != nil {
		t.Fatal(err)
	}
	if want := testutil.Read(t, "testdata/coupling.golden"); out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

// A Go import names a package directory. One named index, mod, or v1.2
// keeps that name, which moduleStem would cut.
func TestCouplingReadsAGoImportOfAnIndexPackage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var log strings.Builder
	for i := range 5 {
		fmt.Fprintf(&log, "--%040d\n\ncmd/x/main.go\ninternal/index/i.go\npkg/v1.2/v.go\n\n", i)
	}
	gitLog, resolved := filepath.Join(dir, "gitlog.txt"), filepath.Join(dir, "imports-resolved.tsv")
	testutil.Write(t, gitLog, log.String())
	testutil.Write(t, resolved, "cmd/x/main.go\tinternal/index\ncmd/x/main.go\tpkg/v1.2\n")
	var out bytes.Buffer
	if err := Coupling(gitLog, resolved, &out); err != nil {
		t.Fatal(err)
	}
	for _, pair := range []string{"cmd/x/main.go\tinternal/index/i.go", "cmd/x/main.go\tpkg/v1.2/v.go"} {
		if !strings.Contains(out.String(), "\timports\t"+pair+"\n") {
			t.Errorf("%s isn't an import in:\n%s", pair, out.String())
		}
	}
}

func TestRawSpecs(t *testing.T) {
	t.Parallel()
	for text, want := range map[string][]spec{
		`"./x"`:                             {{"./x", plain}},
		`#include "net/http.h"`:             {{"net/http.h", relative}},
		`#include <stdio.h>`:                {{"stdio.h", plain}},
		`require_relative "lib/a"`:          {{"lib/a", relative}},
		`require("x")`:                      {{"x", plain}},
		`use a::{b, c::{d, e}};`:            {{"a::b", plain}, {"a::c::d", plain}, {"a::c::e", plain}},
		`pub(crate) use a::b as c;`:         {{"a::b", plain}},
		`from pkg import a, b as c`:         {{"pkg.a", plain}, {"pkg.b", plain}},
		`from . import (x, y)`:              {{".x", plain}, {".y", plain}},
		`from pkg import *`:                 {{"pkg", plain}},
		`import static a.B.C;`:              {{"a.B.C", plain}},
		`using Json = Newtonsoft.Json;`:     {{"Newtonsoft.Json", plain}},
		`mod net;`:                          {{"self::net", plain}},
		`extern crate clap;`:                {{"clap", plain}},
		`import 'package:app/main.dart';`:   {{"app/main.dart", plain}},
		`alias MyApp.HTTPClient, as: Http`:  {{"MyApp.HTTPClient", plain}},
		`imports.foo`:                       {{"imports.foo", plain}},
		`using System.Collections.Generic;`: {{"System.Collections.Generic", plain}},
	} {
		if got := rawSpecs(text); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", text, got, want)
		}
	}
}

func TestExternalName(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		path   string
		dotted bool
		want   string
	}{
		{"@scope/ui/button", false, "@scope/ui"},
		{"lodash/fp", false, "lodash"},
		{"github.com/acme/kit/log", false, "github.com/acme/kit"},
		{"com/acme/app/Main", true, "com.acme.app"},
		{"java/util/List", true, "java.util"},
		{"requests/adapters", true, "requests"},
	} {
		if got := externalName(c.path, c.dotted); got != c.want {
			t.Errorf("%s: %q, want %q", c.path, got, c.want)
		}
	}
}

func TestSnakeCase(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"MyApp.HTTPClient": "my_app.http_client", "Repo": "repo", "V2Api": "v2_api"} {
		if got := snakeCase(in); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// The split descends while one directory holds over 80% of the files.
func TestFindSplit(t *testing.T) {
	t.Parallel()
	nested := []string{"README", "src/a/x.ts", "src/a/y.ts", "src/b/z.ts", "src/c/w.ts", "src/d/v.ts"}
	if got := findSplit(nested); got != "src" {
		t.Errorf("five of six under src: %q", got)
	}
	if got := findSplit([]string{"a/x", "b/y"}); got != "" {
		t.Errorf("an even spread splits at the root: %q", got)
	}
}

func TestRunUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := Run(nil, nil, &stdout, &stderr); code != 2 || !bytes.Contains(stderr.Bytes(), []byte("usage: kit code-graph")) {
		t.Errorf("no args: exit %d, %q", code, stderr.String())
	}
	if code := Run([]string{"coupling", "missing.txt", "missing.tsv"}, nil, &stdout, &stderr); code != 1 {
		t.Errorf("a missing file: exit %d", code)
	}
}
