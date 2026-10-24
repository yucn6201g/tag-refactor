package rules

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Unit tests of the character encodings (NFR9.5, security design §4),
// written after the implementation.

const japaneseRules = "# 環境のルール\nversion: 1\nkeys:\n  Environment:\n    aliases: [env, 環境]\n" +
	"    values:\n      production: [prod, 本番]\nremove: [tmp]\n"

// encodeUTF16 writes s as UTF-16 in the byte order, with or without a byte
// order mark, as Windows PowerShell 5.1 does with ">".
func encodeUTF16(s string, order binary.AppendByteOrder, bom bool) []byte {
	units := utf16.Encode([]rune(s))
	var out []byte
	if bom {
		out = order.AppendUint16(out, 0xFEFF)
	}
	for _, u := range units {
		out = order.AppendUint16(out, u)
	}
	return out
}

func TestUTF16WithAByteOrderMarkReadsLikeUTF8(t *testing.T) {
	want, err := parse("r.yaml", []byte(japaneseRules))
	if err != nil {
		t.Fatal(err)
	}
	crlf := strings.ReplaceAll(japaneseRules, "\n", "\r\n")
	for name, data := range map[string][]byte{
		"UTF-16LE":        encodeUTF16(japaneseRules, binary.LittleEndian, true),
		"UTF-16BE":        encodeUTF16(japaneseRules, binary.BigEndian, true),
		"UTF-16LE (CRLF)": encodeUTF16(crlf, binary.LittleEndian, true),
		"UTF-8 with BOM":  append([]byte{0xEF, 0xBB, 0xBF}, crlf...),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parse("r.yaml", data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v\nwant %+v", got, want)
			}
		})
	}
	// The same through Load, from a file.
	path := filepath.Join(t.TempDir(), "tagrules.yaml")
	if err := os.WriteFile(path, encodeUTF16(crlf, binary.LittleEndian, true), 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := load(path)
	if err != nil || strings.Join(rs.KeyRules[0].Aliases, ",") != "env,環境" || rs.KeyRules[0].Values[0].Spellings[1] != "本番" {
		t.Fatalf("rs = %+v, err = %v", rs, err)
	}
}

func TestUTF16WithoutAByteOrderMarkIsReportedOnLineOne(t *testing.T) {
	for name, data := range map[string][]byte{
		"ASCII only":    encodeUTF16("version: 1\nkeys: {}\n", binary.LittleEndian, false),
		"with Japanese": encodeUTF16(japaneseRules, binary.BigEndian, false),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parse("r.yaml", data)
			e := requireRuleError(t, err, errkind.InvalidInput, 1, "")
			if !strings.Contains(e.Detail, "not UTF-8") || !strings.Contains(e.Hint, "save it as UTF-8") || !strings.Contains(e.Hint, "--output") {
				t.Fatalf("detail = %q, hint = %q", e.Detail, e.Hint)
			}
		})
	}
	// A UTF-8 file that happens to have an even length is not mistaken for
	// UTF-16.
	if _, err := parse("r.yaml", []byte("version: 1\n")); err != nil {
		t.Fatalf("even-length UTF-8: %v", err)
	}
}

func TestAnOddByteAndUnpairedSurrogatesBecomeTheReplacementCharacter(t *testing.T) {
	odd := append(encodeUTF16("version: 1\nremove: [a]\n#", binary.LittleEndian, true), 'x')
	rs, err := parse("r.yaml", odd)
	if err != nil || len(rs.RemoveKeys) != 1 {
		t.Fatalf("an odd last byte must become U+FFFD in the comment: %+v %v", rs, err)
	}

	units := utf16.Encode([]rune("version: 1\nremove: [\"x"))
	units = append(units, 0xD800) // a high surrogate without its pair
	units = append(units, utf16.Encode([]rune("\"]\n"))...)
	data := []byte{0xFE, 0xFF}
	for _, u := range units {
		data = binary.BigEndian.AppendUint16(data, u)
	}
	rs, err = parse("r.yaml", data)
	if err != nil || rs.RemoveKeys[0].Key != "x�" {
		t.Fatalf("an unpaired surrogate must become U+FFFD: %+v %v", rs, err)
	}
}

func TestSizeIsJudgedOnTheUTF8Text(t *testing.T) {
	if MaxInputSize != 128*1024 {
		t.Fatalf("MaxInputSize = %d, want 128 KiB", MaxInputSize)
	}
	comment := func(n int) string { return "version: 1\n#" + strings.Repeat("x", n) + "\n" }
	cases := []struct {
		name string
		data []byte
		ok   bool
	}{
		{"UTF-16 of 40 KiB of ASCII (80 KiB of bytes)", encodeUTF16(comment(40<<10), binary.LittleEndian, true), true},
		{"UTF-16 over 128 KiB of bytes", encodeUTF16(comment(65<<10), binary.LittleEndian, true), false},
		{"UTF-16 of Japanese over 64 KiB once in UTF-8", encodeUTF16("version: 1\n#"+strings.Repeat("あ", 25_000)+"\n", binary.LittleEndian, true), false},
		{"UTF-8 over 64 KiB", []byte(comment(MaxFileSize)), false},
		{"UTF-8 with a BOM and exactly 64 KiB after it", append([]byte{0xEF, 0xBB, 0xBF}, []byte(comment(MaxFileSize-len(comment(0))))...), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse("r.yaml", tc.data)
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			e := requireRuleError(t, err, errkind.InvalidInput, 0, "")
			if e.Detail != "the rule file is larger than 64 KiB" {
				t.Fatalf("detail = %q", e.Detail)
			}
		})
	}
}

func TestInvalidUTF8IsReportedAtItsLine(t *testing.T) {
	_, err := parse("r.yaml", []byte("version: 1\r\nkeys:\r\n  \"a\xff\": {}\r\n"))
	e := requireRuleError(t, err, errkind.InvalidInput, 3, "")
	if !strings.Contains(e.Detail, "not valid UTF-8") || !strings.Contains(e.Hint, "UTF-8") {
		t.Fatalf("detail = %q, hint = %q", e.Detail, e.Hint)
	}
}

func TestLooksLikeUTF16(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"a", false},
		{"ab", false},
		{"a\x00", true},
		{"a\x00b", false},           // odd length
		{"abcdefgh\x00\x00", false}, // fewer than a quarter of zeros
		{"abcdef\x00\x00", true},    // exactly a quarter
		{"a\x00b\x00c\x00", true},   // ASCII in UTF-16LE
	}
	for _, tc := range cases {
		if got := looksLikeUTF16([]byte(tc.in)); got != tc.want {
			t.Errorf("looksLikeUTF16(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
