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
	if _, err := kittool.Path("ast-grep"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err) // CI installs ast-grep, so the rules always run there
		}
		t.Skip(err) // kit: allow ast-grep is optional on a dev machine
	}
	var out bytes.Buffer
	// vendor/ is a skip_dirs entry: third-party code, not the repo's.
	if err := Imports(fixtureRepo(t)+"/", []string{"vendor"}, &out, io.Discard); err != nil {
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
	err := Imports(t.TempDir(), nil, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "exit status 8: Error: Cannot parse rule kotlin; unknown kind import_header") {
		t.Errorf("a failed scan: %v", err)
	}
	testutil.Stub(t, filepath.Join(bin, "ast-grep"), "exec sleep 5\n")
	defer func(d time.Duration) { scanTimeout = d }(scanTimeout)
	scanTimeout = 100 * time.Millisecond
	if err := Imports(t.TempDir(), nil, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "timed out after 100ms") {
		t.Errorf("a slow scan: %v", err)
	}
}

// The scan reads TypeScript's import x = require(), a root written with
// ./ or a trailing /., and skips a path a TSV row can't hold, saying so. A
// root that doesn't exist is an error.
func TestImportsReadsEveryFormAndPath(t *testing.T) {
	t.Parallel()
	if _, err := kittool.Path("ast-grep"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err)
		}
		t.Skip(err) // kit: allow ast-grep is optional on a dev machine
	}
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "src/a.ts"), "import x = require(\"../lib/b\");\n")
	testutil.Write(t, filepath.Join(root, "src/tab\tfile.ts"), "import \"./a\";\n")
	for _, r := range []string{root, root + "/.", root + "/src/.."} {
		var out, stderr bytes.Buffer
		if err := Imports(r, nil, &out, &stderr); err != nil {
			t.Fatal(err)
		}
		if want := "src/a.ts\t\"../lib/b\"\n"; out.String() != want {
			t.Errorf("%s: got %q, want %q", r, out.String(), want)
		}
		if !strings.Contains(stderr.String(), `skipped: "src/tab\tfile.ts"`) {
			t.Errorf("%s: stderr %q", r, stderr.String())
		}
	}
	if err := Imports(filepath.Join(root, "gone"), nil, io.Discard, io.Discard); err == nil {
		t.Error("a missing root scanned clean")
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

// A relative import of nothing tracked (a generated file, a path out of
// the repo) is no edge to the nearest directory that exists, and no
// external; a Python from x import name still finds x.
func TestDepsLeavesAnUnresolvedRelativeImportOut(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "src/web/page.ts"), "")
	testutil.Write(t, filepath.Join(root, "src/app/core/models.py"), "")
	testutil.Write(t, filepath.Join(root, "src/app/core/views.py"), "")
	testutil.Write(t, filepath.Join(root, "src/lib/a.ts"), "")
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "add", "-A")
	imports := filepath.Join(t.TempDir(), "imports.tsv")
	testutil.Write(t, imports, "src/web/page.ts\t\"../gen/api\"\nsrc/web/page.ts\t\"../../../outside\"\nsrc/web/page.ts\t\"../lib/a.js\"\n"+
		"src/app/core/views.py\tfrom ...x import y\nsrc/app/core/views.py\tfrom .models import Item\n")
	out := filepath.Join(t.TempDir(), "raw")
	if err := Deps(root, imports, out, "", nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Read(t, filepath.Join(out, "imports-resolved.tsv")); got != "src/web/page.ts\tsrc/lib/a\nsrc/app/core/views.py\tsrc/app/core/models\n" {
		t.Errorf("imports-resolved:\n%s", got)
	}
	if got := testutil.Read(t, filepath.Join(out, "externals.tsv")); got != "" {
		t.Errorf("externals:\n%s", got)
	}
	if got := testutil.Read(t, filepath.Join(out, "modules.txt")); strings.Contains(got, "..") {
		t.Errorf("modules:\n%s", got)
	}
}

// deps runs Deps on a repo of files, its imports read from rows, with
// skipDirs, and returns the out dir and the summary.
func deps(t *testing.T, files map[string]string, rows string, skipDirs []string) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		testutil.Write(t, filepath.Join(root, name), body)
	}
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "add", "-Af") // past a global ignore: pkg/, build/
	imports := filepath.Join(t.TempDir(), "imports.tsv")
	testutil.Write(t, imports, rows)
	out := filepath.Join(t.TempDir(), "raw")
	var summary bytes.Buffer
	if err := Deps(root, imports, out, "", skipDirs, &summary); err != nil {
		t.Fatal(err)
	}
	return out, summary.String()
}

// The split counts every source file, not only the ones that import, and
// a source directory skip_dirs drops is named.
func TestDepsSplitsOnEverySourceFileAndNamesASkippedDir(t *testing.T) {
	t.Parallel()
	out, summary := deps(t, map[string]string{"src/app/a.ts": "", "src/core/c.ts": "", "src/util/u.ts": "", "src/build/b.ts": "", "README.md": ""},
		"src/app/a.ts\t\"../core/c\"\nsrc/app/a.ts\t\"../build/b\"\n", []string{"build"})
	if got := testutil.Read(t, filepath.Join(out, "deps.tsv")); got != "1\tsrc/app\tsrc/core\n" {
		t.Errorf("deps:\n%s", got)
	}
	if !strings.Contains(summary, "skipped dir: src/build (skip_dirs)\n") || !strings.Contains(summary, "split dir: src;") {
		t.Errorf("summary:\n%s", summary)
	}
}

// A Go import of a package nothing tracks is no edge to its parent
// directory: a Go import names the package's directory exactly.
func TestDepsLeavesAnUntrackedGoPackageOut(t *testing.T) {
	t.Parallel()
	out, _ := deps(t, map[string]string{"go.mod": "module ex.com/m\n", "internal/app/a.go": "", "internal/core/c.go": ""},
		"internal/app/a.go\t\"ex.com/m/internal/gone\"\ninternal/app/a.go\t\"ex.com/m/internal/core\"\n", nil)
	if got := testutil.Read(t, filepath.Join(out, "deps.tsv")); got != "1\tinternal/app\tinternal/core\n" {
		t.Errorf("deps:\n%s", got)
	}
}

// A root go.mod's own package is the repo root's files, which no tracked
// directory names.
func TestDepsResolvesAGoImportOfTheRootPackage(t *testing.T) {
	t.Parallel()
	out, _ := deps(t, map[string]string{"go.mod": "module ex.com/m\n", "lib.go": "", "lib_test.go": "", "cmd/x/main.go": ""},
		"cmd/x/main.go\t\"ex.com/m\"\n", nil)
	if got := testutil.Read(t, filepath.Join(out, "imports-resolved.tsv")); got != "cmd/x/main.go\tlib.go\n" {
		t.Errorf("imports-resolved:\n%s", got)
	}
}

// A split holding only source files keeps its file modules beside a
// directory of assets. Only source files make a child module.
func TestDepsKeepsFileModulesBesideANonSourceDir(t *testing.T) {
	t.Parallel()
	out, _ := deps(t, map[string]string{"src/a.ts": "", "src/b.ts": "", "src/assets/x.png": ""}, "src/a.ts\t\"./b\"\n", nil)
	if got := testutil.Read(t, filepath.Join(out, "deps.tsv")); got != "1\tsrc/a\tsrc/b\n" {
		t.Errorf("deps:\n%s", got)
	}
}

// A file and an import that resolves to it are one module, at the repo
// root and for a dotted name, so mutual imports form a cycle.
func TestDepsGivesAFileAndAnImportOfItOneModule(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		files map[string]string
		rows  string
		want  string
	}{
		"flat":   {map[string]string{"a.ts": "", "b.ts": ""}, "a.ts\t\"./b\"\nb.ts\t\"./a\"\n", "1\ta\tb\n1\tb\ta\n"},
		"dotted": {map[string]string{"src/a.ts": "", "src/b.client.ts": ""}, "src/a.ts\t\"./b.client\"\nsrc/b.client.ts\t\"./a\"\n", "1\tsrc/a\tsrc/b.client\n1\tsrc/b.client\tsrc/a\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, _ := deps(t, tc.files, tc.rows, nil)
			if got := testutil.Read(t, filepath.Join(out, "deps.tsv")); got != tc.want {
				t.Errorf("deps:\n%s", got)
			}
		})
	}
}

// An import naming a barrel file without its extension resolves to it.
func TestDepsResolvesAnExtensionlessIndexImport(t *testing.T) {
	t.Parallel()
	out, _ := deps(t, map[string]string{"src/index.ts": "", "src/main.ts": ""}, "src/main.ts\t\"./index\"\n", nil)
	if got := testutil.Read(t, filepath.Join(out, "imports-resolved.tsv")); got != "src/main.ts\tsrc/index.ts\n" {
		t.Errorf("imports-resolved:\n%s", got)
	}
}

// A Python import's name isn't always its distribution's (yaml is PyYAML),
// and a workspace package imported by its package.json name is an edge.
func TestDepsMatchesImportNamesToTheirPackages(t *testing.T) {
	t.Parallel()
	out, _ := deps(t, map[string]string{
		"pyproject.toml":            "[project]\ndependencies = [\"PyYAML>=6\", \"Pillow\", \"python-dateutil\"]\n",
		"py/a.py":                   "",
		"packages/ui/package.json":  `{"name": "@org/ui"}`,
		"packages/ui/index.ts":      "",
		"packages/ui/src/theme.ts":  "",
		"packages/app/package.json": `{"name": "@org/app", "dependencies": {"@org/ui": "workspace:*"}}`,
		"packages/app/src/main.ts":  "",
	}, "py/a.py\timport yaml\npy/a.py\tfrom PIL import Image\npy/a.py\timport dateutil\n"+
		"packages/app/src/main.ts\t\"@org/ui\"\npackages/app/src/main.ts\t\"@org/ui/src/theme\"\n", nil)
	if got := testutil.Read(t, filepath.Join(out, "externals.tsv")); strings.Contains(got, "undeclared") || strings.Contains(got, "@org") {
		t.Errorf("externals:\n%s", got)
	}
	if got := testutil.Read(t, filepath.Join(out, "imports-resolved.tsv")); got != "packages/app/src/main.ts\tpackages/ui\npackages/app/src/main.ts\tpackages/ui/src/theme\n" {
		t.Errorf("imports-resolved:\n%s", got)
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
	if err := Coupling("testdata/gitlog.txt", "testdata/deps/imports-resolved.golden", fixtureRepo(t), nil, &out); err != nil {
		t.Fatal(err)
	}
	if want := testutil.Read(t, "testdata/coupling.golden"); out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

// Coupling marks a pair an import whatever form deps names the import in,
// including the ones moduleStem would cut.
func TestCouplingReadsEachFormOfAnImport(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, resolved string
		files, pairs   []string
	}{
		// A Go import names a package directory, one named index, mod, or v1.2 too.
		{"a Go import of an index and a v1.2 package", "cmd/x/main.go\tinternal/index\ncmd/x/main.go\tpkg/v1.2\n",
			[]string{"cmd/x/main.go", "internal/index/i.go", "pkg/v1.2/v.go"},
			[]string{"cmd/x/main.go\tinternal/index/i.go", "cmd/x/main.go\tpkg/v1.2/v.go"}},
		// A dotted file's whole stem, b.client, not b.
		{"an import of a dotted file", "src/a.ts\tlib/b.client\n",
			[]string{"src/a.ts", "lib/b.client.ts"},
			[]string{"lib/b.client.ts\tsrc/a.ts"}},
		// The root package by its first .go file, matching every root .go file.
		{"a Go import of the root package", "cmd/x/main.go\tlib.go\n",
			[]string{"cmd/x/main.go", "lib.go", "zz.go"},
			[]string{"cmd/x/main.go\tlib.go", "cmd/x/main.go\tzz.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			var log strings.Builder
			for i := range 5 {
				fmt.Fprintf(&log, "--%040d\n\n%s\n\n", i, strings.Join(tc.files, "\n"))
			}
			gitLog, resolved := filepath.Join(dir, "gitlog.txt"), filepath.Join(dir, "imports-resolved.tsv")
			testutil.Write(t, gitLog, log.String())
			testutil.Write(t, resolved, tc.resolved)
			var out bytes.Buffer
			if err := Coupling(gitLog, resolved, tracked(t, tc.files...), nil, &out); err != nil {
				t.Fatal(err)
			}
			for _, pair := range tc.pairs {
				if !strings.Contains(out.String(), "\timports\t"+pair+"\n") {
					t.Errorf("%s isn't an import in:\n%s", pair, out.String())
				}
			}
		})
	}
}

// tracked is a git repo tracking files, each empty.
func tracked(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		testutil.Write(t, filepath.Join(root, f), "")
	}
	testutil.Git(t, root, "init", "-q")
	testutil.Git(t, root, "add", "-Af") // past a global ignore: pkg/, build/
	return root
}

// A file the repo no longer tracks couples with nothing, and a bulk commit
// is bulk by every file it touched, tests and docs too.
func TestCouplingSkipsGoneFilesAndBulkCommits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var log strings.Builder
	for i := range 6 {
		fmt.Fprintf(&log, "--%040d\n\na/x.py\nb/gone.py\n\n", i)
	}
	for i := range 5 {
		fmt.Fprintf(&log, "--%040d\n\na/x.py\nc/z.py\n", 10+i)
		for j := range 40 {
			fmt.Fprintf(&log, "tests/test_%d.py\n", j)
		}
		log.WriteString("\n")
	}
	gitLog, resolved := filepath.Join(dir, "gitlog.txt"), filepath.Join(dir, "imports-resolved.tsv")
	testutil.Write(t, gitLog, log.String())
	testutil.Write(t, resolved, "")
	var out bytes.Buffer
	if err := Coupling(gitLog, resolved, tracked(t, "a/x.py", "c/z.py"), nil, &out); err != nil {
		t.Fatal(err)
	}
	if want := "shared\tdegree\tlink\tfile_a\tfile_b\n"; out.String() != want {
		t.Errorf("got:\n%s", out.String())
	}
}

// A file under skip_dirs (a committed vendor/) couples with nothing, as
// imports and deps leave it out.
func TestCouplingLeavesSkipDirsOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var log strings.Builder
	for i := range 6 {
		fmt.Fprintf(&log, "--%040d\n\nsrc/a.py\nvendor/lib/v.py\n\n", i)
	}
	gitLog, resolved := filepath.Join(dir, "gitlog.txt"), filepath.Join(dir, "imports-resolved.tsv")
	testutil.Write(t, gitLog, log.String())
	testutil.Write(t, resolved, "")
	var out bytes.Buffer
	if err := Coupling(gitLog, resolved, tracked(t, "src/a.py", "vendor/lib/v.py"), []string{"vendor"}, &out); err != nil {
		t.Fatal(err)
	}
	if want := "shared\tdegree\tlink\tfile_a\tfile_b\n"; out.String() != want {
		t.Errorf("got:\n%s", out.String())
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
		`import "../(shop)/p"`:              {{"../(shop)/p", plain}},
		`require("../(shop)/p")`:            {{"../(shop)/p", plain}},
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

// A Go import names a package's directory, so a package is never split
// into its files: the import would name the split itself, no module.
func TestDepsKeepsAGoPackageWhole(t *testing.T) {
	t.Parallel()
	files := map[string]string{"go.mod": "module ex.com/m\n", "cmd/tool/main.go": ""}
	for _, f := range []string{"a", "b", "c", "d", "e"} {
		files["internal/core/"+f+".go"] = ""
	}
	out, _ := deps(t, files, "cmd/tool/main.go\t\"ex.com/m/internal/core\"\n", nil)
	if got := testutil.Read(t, filepath.Join(out, "deps.tsv")); got != "1\tcmd\tinternal/core\n" {
		t.Errorf("deps:\n%s", got)
	}
}

// Only the file's own language names a directory by a basename, once:
// index for JS and TS, __init__ for Python, mod for Rust.
func TestModuleStem(t *testing.T) {
	t.Parallel()
	for p, want := range map[string]string{
		"src/index.ts": "src", "src/mod/index.ts": "src/mod", "src/mod.ts": "src/mod", "src/__init__.ts": "src/__init__",
		"pkg/__init__.py": "pkg", "pkg/index.py": "pkg/index", "src/net/mod.rs": "src/net", "src/index/mod.rs": "src/index",
		"lib/util.go": "lib/util", "index.ts": "index",
	} {
		if got := moduleStem(p); got != want {
			t.Errorf("moduleStem(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestRunUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := Run(nil, nil, &stdout, &stderr); code != 2 || !bytes.Contains(stderr.Bytes(), []byte("usage: kit code-graph")) {
		t.Errorf("no args: exit %d, %q", code, stderr.String())
	}
	if code := Run([]string{"coupling", "missing.txt", "missing.tsv", "."}, nil, &stdout, &stderr); code != 1 {
		t.Errorf("a missing file: exit %d", code)
	}
}
