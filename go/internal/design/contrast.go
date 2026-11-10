// Package design checks claims about a UI against something measurable: a
// contrast ratio computed, and a design token the project defines.
package design

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// levels are WCAG 2.2's contrast minimums (1.4.3 AA, 1.4.6 AAA), for
// normal and large text.
var levels = []struct {
	name string
	min  float64
}{{"AA normal text", 4.5}, {"AA large text", 3}, {"AAA normal text", 7}, {"AAA large text", 4.5}}

// Contrast is `kit contrast <fg> <bg>`: the WCAG contrast ratio of two
// colors, and each level it passes. 2 on a usage error or an unread color.
func Contrast(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: kit contrast <fg> <bg>  (#rgb, #rrggbb, or rgb(r, g, b))")
		return 2
	}
	var lum [2]float64
	for i, arg := range args {
		rgb, err := parseColor(arg)
		if err != nil {
			fmt.Fprintln(stderr, "kit contrast:", err)
			return 2
		}
		lum[i] = luminance(rgb)
	}
	ratio := (max(lum[0], lum[1]) + 0.05) / (min(lum[0], lum[1]) + 0.05)
	// WCAG forbids rounding a ratio up to a threshold, so truncate.
	fmt.Fprintf(stdout, "%.2f:1\n", math.Floor(ratio*100)/100)
	for _, l := range levels {
		verdict := "fail"
		if ratio >= l.min {
			verdict = "pass"
		}
		fmt.Fprintf(stdout, "%s (%g:1): %s\n", l.name, l.min, verdict)
	}
	return 0
}

// parseColor reads #rgb, #rrggbb (the # optional), or rgb(r, g, b) with
// spaces or commas.
func parseColor(s string) ([3]float64, error) {
	var rgb [3]float64
	t := strings.ToLower(strings.TrimSpace(s))
	if inner, ok := strings.CutPrefix(t, "rgb("); ok && strings.HasSuffix(inner, ")") {
		parts := strings.FieldsFunc(strings.TrimSuffix(inner, ")"), func(r rune) bool { return r == ',' || r == ' ' })
		if len(parts) != 3 {
			return rgb, fmt.Errorf("%q: rgb() takes three channels", s)
		}
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil || n < 0 || n > 255 {
				return rgb, fmt.Errorf("%q: channel %q isn't 0 to 255", s, p)
			}
			rgb[i] = float64(n)
		}
		return rgb, nil
	}
	hex := strings.TrimPrefix(t, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	n, err := strconv.ParseUint(hex, 16, 32)
	if len(hex) != 6 || err != nil {
		return rgb, fmt.Errorf("%q: not #rgb, #rrggbb, or rgb(r, g, b)", s)
	}
	return [3]float64{float64(n >> 16 & 0xff), float64(n >> 8 & 0xff), float64(n & 0xff)}, nil
}

// luminance is WCAG's relative luminance of an sRGB color.
func luminance(rgb [3]float64) float64 {
	var lin [3]float64
	for i, c := range rgb {
		c /= 255
		if c <= 0.04045 {
			lin[i] = c / 12.92
		} else {
			lin[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*lin[0] + 0.7152*lin[1] + 0.0722*lin[2]
}
