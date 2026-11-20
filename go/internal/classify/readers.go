package classify

import (
	"slices"
	"strings"
)

// PureReaders print or test what they're given and write no file,
// whatever their flags. Commands that write only with a flag (sed -i,
// sort -o, find -delete) aren't here.
var PureReaders = map[string]bool{
	"cat": true, "head": true, "tail": true, "wc": true, "ls": true, "stat": true,
	"realpath": true, "readlink": true, "test": true, "[": true, "echo": true, "printf": true,
	"rg": true, "grep": true, "diff": true, "cmp": true, "cut": true, "nl": true, "basename": true,
	"dirname": true, "du": true, "tokei": true, "shasum": true, "jq": true, "gron": true, "pwd": true,
	"true": true,
}

// ReaderWriteFlags are the flags that make a reader write a file or run a
// command: rg's preprocessor and hostname program, git's output file,
// external diff, and pager.
var ReaderWriteFlags = []string{"--pre", "--hostname-bin", "--output", "--ext-diff", "--textconv", "-O", "--open-files-in-pager"}

// WritingFlag is whether word is one of ReaderWriteFlags.
func WritingFlag(word string) bool {
	return slices.ContainsFunc(ReaderWriteFlags, func(f string) bool { return strings.HasPrefix(word, f) })
}

// ReadsOnly is whether words, one call, is a pure reader with none of the
// flags that make it write a file or run a command.
func ReadsOnly(words []string) bool {
	return len(words) > 0 && PureReaders[words[0]] && !slices.ContainsFunc(words[1:], WritingFlag)
}
