package display

import (
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// Unit tests of the terminal width (U4 NFR design Q3) and the counts
// (BR3.6). The characters are written from their code points.

func TestWidthCountsTerminalColumns(t *testing.T) {
	r := func(cps ...rune) string { return string(cps) }
	cases := []struct {
		name string
		s    string
		want int
	}{
		{"ASCII", "env -> Environment", 18},
		{"empty", "", 0},
		{"halfwidth kana", r(0xff76, 0xff9e), 2},            // ｶﾞ: each half width
		{"fullwidth symbols", r(0xff01, 0xff08, 0x3000), 6}, // ！（ and the ideographic space
		{"kanji and kana", "日本語かな", 10},
		{"combining marks", "e" + r(0x0301) + "a" + r(0x20dd), 2}, // Mn and Me take no column
		{"zero width", "a" + r(0x200b) + "b" + r(0x2060) + r(0xfeff), 2},
		{"replacement character", r(0xfffd), 1},
		{"ambiguous width", r(0x00b1, 0x2460, 0x03b1), 3}, // ± ① α are narrow here
		{"emoji", r(0x1f600), 2},
		{"sanitized escape", Sanitize(r(0x202e)), len(`\u{202E}`)},
		{"mixed", "環境 = prod", 11},
	}
	for _, tc := range cases {
		if got := Width(tc.s); got != tc.want {
			t.Errorf("%s: Width(%q) = %d, want %d", tc.name, tc.s, got, tc.want)
		}
	}
	if got := padRight("日本", 6) + "|"; got != "日本  |" {
		t.Errorf("padRight = %q", got)
	}
	if got := padLeft("ｶ", 3); got != "  ｶ" {
		t.Errorf("padLeft = %q", got)
	}
	if padRight("longer", 2) != "longer" || padLeft("longer", 2) != "longer" {
		t.Error("a longer string is kept whole")
	}
}

// versionNumbers reads "17.0.0" as [17 0 0].
func versionNumbers(t *testing.T, v string) []int {
	t.Helper()
	var out []int
	for _, part := range strings.Split(v, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			t.Fatalf("version %q: %v", v, err)
		}
		out = append(out, n)
	}
	return out
}

// newer reports whether version a is newer than b.
func newer(a, b []int) bool {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// cicd-pipeline.md: the table must not be older than the Unicode of the Go
// that builds the tool. An older Go (the oldstable leg of CI) is fine: wide
// ranges only grow.
func TestTheWidthTableIsNotOlderThanUnicode(t *testing.T) {
	table, goVersion := versionNumbers(t, widthTableUnicodeVersion), versionNumbers(t, unicode.Version)
	if newer(goVersion, table) {
		t.Fatalf("Go has Unicode %s but the width table is %s: run \"go generate\" in internal/display "+
			"(gen_width.go downloads EastAsianWidth.txt of the new version) and commit width_table.go",
			unicode.Version, widthTableUnicodeVersion)
	}
	if !newer([]int{17, 0, 1}, []int{17}) || newer([]int{17}, []int{17, 0, 0}) {
		t.Fatal("the version comparison is wrong")
	}
	for i, r := range wideRanges {
		if r.lo > r.hi || (i > 0 && wideRanges[i-1].hi >= r.lo) {
			t.Fatalf("the table is not sorted at %d: %v", i, r)
		}
	}
	if !isWide(0x3000) || !isWide(0x4e00) || !isWide(0xac00) || isWide('a') || isWide(0xff76) || isWide(0x10ffff) {
		t.Fatal("the table misses known wide or narrow characters")
	}
}

func TestFormatCountPutsACommaEveryThreeDigits(t *testing.T) {
	for n, want := range map[int]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 5341: "5,341", 123456: "123,456",
		1234567: "1,234,567", -1234: "-1,234", -12: "-12"} {
		if got := FormatCount(n); got != want {
			t.Errorf("FormatCount(%d) = %q, want %q", n, got, want)
		}
	}
	if Count(English, "unit.resource", 1) != "1 resource" || Count(English, "unit.resource", 5341) != "5,341 resources" ||
		Count(Japanese, "unit.resource", 5341) != "5,341 リソース" || Count(English, "no.such.unit", 3) != "3" {
		t.Fatal("Count must choose the unit and the singular in English")
	}
}
