package display

//go:generate go run gen_width.go

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// runeRange is a range of characters, both ends included.
type runeRange struct{ lo, hi rune }

// Width returns the number of terminal columns s takes (U4 NFR design Q3).
// Measure the text as it is printed: after Sanitize and before colors, so
// the sequences of colors are not counted and a replaced character counts
// as its visible form. Combining marks (Mn, Me) and zero-width characters
// take no column; characters of East Asian Width W and F take two; every
// other character, U+FFFD, the halfwidth kana and the ambiguous ones
// included, takes one.
func Width(s string) int {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return len(s)
	}
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}
	return n
}

func runeWidth(r rune) int {
	switch {
	case r < utf8.RuneSelf:
		return 1
	case r == 0x200b || r == 0x200c || r == 0x200d || r == 0x2060 || r == 0xfeff:
		return 0
	case unicode.In(r, unicode.Mn, unicode.Me):
		return 0
	case isWide(r):
		return 2
	}
	return 1
}

// isWide reports whether r has East Asian Width W or F.
func isWide(r rune) bool {
	i := sort.Search(len(wideRanges), func(i int) bool { return wideRanges[i].hi >= r })
	return i < len(wideRanges) && wideRanges[i].lo <= r
}

// padRight pads s with spaces to width columns. A longer s is kept whole.
func padRight(s string, width int) string {
	if w := Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// padLeft pads s with spaces on the left to width columns.
func padLeft(s string, width int) string {
	if w := Width(s); w < width {
		return strings.Repeat(" ", width-w) + s
	}
	return s
}

// FormatCount writes a count with a comma every three digits: 1,234 (BR3.6).
// Every language uses the same form. JSON keeps plain numbers.
func FormatCount(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(s[:first])
	for i := first; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
