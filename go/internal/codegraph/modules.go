package codegraph

import (
	"strings"
)

// findSplit descends while one directory holds over 80% of files.
func findSplit(files []string) string {
	prefix := ""
	for {
		children, inside := map[string]int{}, 0
		for _, f := range files {
			rest, ok := strings.CutPrefix(f, prefix)
			if !ok {
				continue
			}
			inside++
			if child, _, nested := strings.Cut(rest, "/"); nested {
				children[child]++
			}
		}
		if len(children) == 0 {
			return strings.TrimSuffix(prefix, "/")
		}
		top := byCount(children, strings.Compare)[0]
		if float64(children[top])/float64(inside) <= 0.8 {
			return strings.TrimSuffix(prefix, "/")
		}
		prefix += top + "/"
	}
}

// modules maps a path to its module: a child of the split directory, or
// a top-level directory outside it; each file when the split holds only
// files. tracked tells a file from an import's resolved stem.
type modules struct {
	split    string
	children map[string]bool
	tracked  map[string]bool
}

// newModules reads the split's child modules from sources: a directory of
// assets beside the split's files is no module.
func newModules(split string, sources, files []string) modules {
	m := modules{split: split, children: map[string]bool{}, tracked: map[string]bool{}}
	for _, f := range files {
		m.tracked[f] = true
	}
	for _, p := range sources {
		if rest, ok := strings.CutPrefix(p, split+"/"); ok && split != "" {
			if child, _, nested := strings.Cut(rest, "/"); nested {
				m.children[child] = true
			}
		}
	}
	return m
}

func (m modules) of(p string) string {
	inSplit := m.split != "" && strings.HasPrefix(p, m.split+"/")
	top, _, nested := strings.Cut(p, "/")
	if !nested {
		top = moduleID(p, m.tracked) // a root file is its own module
	}
	switch {
	case m.split != "" && len(m.children) == 0:
		if p == m.split {
			return "" // the split itself is none of its file modules
		}
		if inSplit {
			return moduleID(p, m.tracked)
		}
		return top
	case inSplit:
		child, _, _ := strings.Cut(strings.TrimPrefix(p, m.split+"/"), "/")
		if m.children[child] {
			return m.split + "/" + child
		}
		return m.split
	}
	return top
}
