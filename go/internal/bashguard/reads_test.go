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
