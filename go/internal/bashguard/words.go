package bashguard

import (
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
	"strings"
)

// literal is whether parts are plain text, single-quoted text with no $,
// or double-quoted plain text, with no brace or glob character outside
// quotes. An allow for a path reads only such words, since any other form
// (an expansion, $'...', {a,b}, *) isn't the text the command gets.
func literal(parts []syntax.WordPart) bool {
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			if strings.ContainsAny(p.Value, "{}*?[") {
				return false
			}
		case *syntax.SglQuoted:
			if p.Dollar {
				return false
			}
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				if _, ok := inner.(*syntax.Lit); !ok {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// wordValue removes quotes and resolves escapes, leaving every expansion as
// its source text.
func wordValue(src string, w *syntax.Word) string {
	var b strings.Builder
	for _, part := range w.Parts {
		b.WriteString(partValue(src, part, false))
	}
	return b.String()
}

func partValue(src string, part syntax.WordPart, inDouble bool) string {
	switch p := part.(type) {
	case *syntax.Lit:
		return unescape(p.Value, inDouble)
	case *syntax.SglQuoted:
		if p.Dollar {
			// $'\x72m' is rm. Format reads printf verbs too, so a % stays
			// literal; the shell cuts the value at a NUL. Workaround: a nil
			// config makes mvdan/sh v3.14.1 write its shared zeroConfig, a race.
			v, _, err := expand.Format(&expand.Config{}, strings.ReplaceAll(p.Value, "%", "%%"), nil)
			if err != nil {
				return src[p.Pos().Offset():p.End().Offset()]
			}
			v, _, _ = strings.Cut(v, "\x00")
			return v
		}
		return p.Value
	case *syntax.DblQuoted:
		var b strings.Builder
		for _, inner := range p.Parts {
			b.WriteString(partValue(src, inner, true))
		}
		return b.String()
	default:
		return src[part.Pos().Offset():part.End().Offset()]
	}
}

// unescape resolves backslash escapes as bash does: anything outside
// quotes, only $ ` " \ and newline inside double quotes.
func unescape(s string, inDouble bool) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		next := s[i+1]
		switch {
		case next == '\n':
			i++
		case !inDouble || strings.IndexByte("$`\"\\", next) >= 0:
			b.WriteByte(next)
			i++
		default:
			b.WriteByte('\\')
		}
	}
	return b.String()
}

func baseName(s string) string { return s[strings.LastIndexByte(s, '/')+1:] }
