package bashguard

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ku5ic/claude-kit/go/internal/testutil"
)

func TestReadTargets(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "run.sh")
	testutil.Write(t, file, "echo hi\n")
	for _, tc := range []struct {
		name, cmd string
		want      []string
	}{
		{"cat reads its file", "cat run.sh", []string{file}},
		{"a flag's value before the file is skipped", "head -n 5 run.sh | wc -l", []string{file}},
		{"sed -n's script is skipped", "sed -n 1,20p run.sh", []string{file}},
		{"rg's -g glob joins the directory", "rg -g '*.sh' hi", []string{filepath.Join(dir, "*.sh")}},
		{"grep's --include= glob joins the directory", "grep -r --include=*.sh hi .", []string{filepath.Join(dir, "*.sh")}},
		{"an attached -g glob's braces expand", "rg -g*.{sh,go} hi", []string{filepath.Join(dir, "*.sh"), filepath.Join(dir, "*.go")}},
		{"a reader after cd reads from the new directory", "cd / && cat " + file, []string{file}},
		{"awk reads its file, past the script", "awk '{print}' run.sh", []string{file}},
		{"any command naming the file reads it", "python3 run.sh", []string{file}},
		{"a < redirect reads its file", "sort < run.sh", []string{file}},
		{"git show's path reads it", "git show HEAD:run.sh", []string{file}},
		{"a file that doesn't exist is nothing", "cat missing.sh", nil},
		{"a command that only names the file is nothing", "echo run.sh", nil},
		{"a metadata command is nothing", "wc -l run.sh && ls run.sh", nil},
		{"a directory is nothing", "rg hi " + dir, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ReadTargets(tc.cmd, "/home/nobody", dir); !slices.Equal(got, tc.want) {
				t.Errorf("ReadTargets(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

func TestOnlyReads(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{"cat a.go | head -5", true},
		{"rg -n foo 2>/dev/null && git -C /repo log -3", true},
		{"git diff --stat 2>&1 | tail -3", true},
		{"kit scratch-dir", true},
		{"cat a > b", false},
		{"sed -i '' s/x/y/ a.go", false},
		{"git commit -m x", false},
		{"FOO=1 cat a", false},
		{"echo $(touch x)", false},
		{"go test ./...", false},
		{"rg --pre ./x.sh foo", false},
		{"rg --hostname-bin=./x.sh foo", false},
		{"./test x", false},
		{"scripts/diff a b", false},
		{"git annotate a.go", true},
		{"git diff --output=x.patch", false},
		{"git log -O order --oneline", false},
		{"rg -o foo a.go", true},
		{"cat a &> b.go", false},
		{"cat a &>> b.go", false},
		{"cat a >& b.go", false},
		{"cat a >&2", true},
		{"cat a 2>&-", true},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			t.Parallel()
			segs, _ := parseAll(tc.cmd)
			if got := onlyReads(segs); got != tc.want {
				t.Errorf("onlyReads(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}

func TestWriteTargets(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	at := func(name string) string { return filepath.Join(dir, name) }
	for _, tc := range []struct {
		name, cmd string
		want      []string
	}{
		{"a > target", "echo K=1 > .env", []string{at(".env")}},
		{"a >> target, ~ expanded", "echo k >> ~/.ssh/authorized_keys", []string{"/home/nobody/.ssh/authorized_keys"}},
		{"a redirect alone", "> .env", []string{at(".env")}},
		{"a compound command's redirect", "{ echo x; } > ci.yml", []string{at("ci.yml")}},
		{"an fd duplication is no file", "echo x >&2", nil},
		{"tee's files", "echo x | tee -a a.txt b.txt", []string{at("a.txt"), at("b.txt")}},
		{"cp's destination, not its source", "cp hook .git/hooks/pre-commit", []string{at(".git/hooks/pre-commit")}},
		{"mv's and install's destination", "mv a b && install -m 755 c d", []string{at("b"), at("d")}},
		{"sed -i's files, not its script", "sed -i 's/a/b/' go.sum", []string{at("go.sum")}},
		{"sed without -i writes nothing", "sed 's/a/b/' go.sum", nil},
		{"a write after cd lands in the new directory", "cd / && echo x > /tmp/w", []string{"/tmp/w"}},
		{"a reader writes nothing", "cat .env.example", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := WriteTargets(tc.cmd, "/home/nobody", dir); !slices.Equal(got, tc.want) {
				t.Errorf("WriteTargets(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}
