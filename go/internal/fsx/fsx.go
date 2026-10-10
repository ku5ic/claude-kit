// Package fsx holds the filesystem helpers every layer shares: atomic
// writes, appends, file tests, physical paths, and walking up for a file.
// It imports nothing from the kit.
package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// WriteAtomic replaces path with data through a temp file in its directory,
// creating the directory, so a reader never sees a partial write; on any
// error path is untouched.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	// Rename only after every earlier step succeeded: a failed write must
	// leave path untouched.
	if err = errors.Join(err, tmp.Close()); err == nil {
		if err = os.Chmod(tmp.Name(), perm); err == nil {
			err = os.Rename(tmp.Name(), path)
		}
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// Append adds data to the end of path in one write, creating it and its
// directory.
func Append(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return errors.Join(err, f.Close())
}

// IsFile is true for an existing regular file.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// IsExecutable is true for an existing non-directory with an execute bit.
func IsExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// IsDir is true for an existing directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// PhysicalPath follows a symlink at path itself, then resolves its
// directory. Works for paths that don't exist yet.
func PhysicalPath(path string) string {
	for range 40 {
		target, err := os.Readlink(path)
		if err != nil {
			break
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return path
	}
	return filepath.Join(dir, filepath.Base(path))
}

// FindUp returns the first dir/name for each dir from start up to and
// including stop, never above it; "" when none exists. A start outside stop
// is checked on its own.
func FindUp(start, stop string, names ...string) string {
	dir := start
	for {
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if _, err := os.Lstat(candidate); err == nil {
				return candidate
			}
		}
		if dir == stop || dir == "/" || !strings.HasPrefix(dir, stop+"/") {
			return ""
		}
		dir = filepath.Dir(dir)
	}
}
