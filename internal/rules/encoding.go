package rules

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// Byte order marks (NFR9.5).
var (
	utf8BOM    = []byte{0xEF, 0xBB, 0xBF}
	utf16LEBOM = []byte{0xFF, 0xFE}
	utf16BEBOM = []byte{0xFE, 0xFF}
)

const (
	tooLargeDetail = "the rule file is larger than 64 KiB"
	tooLargeHint   = "a rule file only lists tag keys; reduce it below 64 KiB"
	notUTF8Hint    = "save it as UTF-8 (use rules suggest --output tagrules.yaml)"
)

// decode returns the text of the rule file in UTF-8 (security design §4): a
// UTF-8 byte order mark is removed, UTF-16 with a byte order mark (what
// Windows PowerShell 5.1 writes with ">") is converted, and UTF-16 without
// one is reported, since it is UTF-8 only by accident. The 64 KiB limit
// applies to the UTF-8 text.
func (l *loader) decode(data []byte) (string, error) {
	if len(data) > MaxInputSize {
		return "", l.fail(errkind.InvalidInput, 0, "", tooLargeDetail, tooLargeHint)
	}
	var text string
	switch {
	case bytes.HasPrefix(data, utf8BOM):
		text = string(data[len(utf8BOM):])
	case bytes.HasPrefix(data, utf16LEBOM):
		text = decodeUTF16(data[len(utf16LEBOM):], binary.LittleEndian)
	case bytes.HasPrefix(data, utf16BEBOM):
		text = decodeUTF16(data[len(utf16BEBOM):], binary.BigEndian)
	case looksLikeUTF16(data):
		return "", l.fail(errkind.InvalidInput, 1, "", "the rule file is not UTF-8 (it looks like UTF-16 without a byte order mark)", notUTF8Hint)
	default:
		text = string(data)
	}
	if len(text) > MaxFileSize {
		return "", l.fail(errkind.InvalidInput, 0, "", tooLargeDetail, tooLargeHint)
	}
	return text, nil
}

// looksLikeUTF16 reports whether data without a byte order mark looks like
// UTF-16: an even length and at least a quarter of the bytes zero. ASCII-only
// UTF-16 is valid UTF-8, so validity cannot tell them apart; a rule file
// never contains NUL.
func looksLikeUTF16(data []byte) bool {
	if len(data) < 2 || len(data)%2 != 0 {
		return false
	}
	return bytes.Count(data, []byte{0})*4 >= len(data)
}

// decodeUTF16 converts UTF-16 to UTF-8. Unpaired surrogates and a last odd
// byte become U+FFFD; the YAML reading then reports the line.
func decodeUTF16(b []byte, order binary.ByteOrder) string {
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = order.Uint16(b[2*i:])
	}
	var s strings.Builder
	s.Grow(len(b))
	for _, r := range utf16.Decode(units) {
		s.WriteRune(r)
	}
	if len(b)%2 == 1 {
		s.WriteRune(utf8.RuneError)
	}
	return s.String()
}

// checkUTF8 reports the line of the first byte that is not UTF-8.
func (l *loader) checkUTF8(text string) error {
	if utf8.ValidString(text) {
		return nil
	}
	at := 0
	for at < len(text) {
		r, size := utf8.DecodeRuneInString(text[at:])
		if r == utf8.RuneError && size == 1 {
			break
		}
		at += size
	}
	line := strings.Count(text[:at], "\n") + 1
	return l.fail(errkind.InvalidInput, line, "", "the rule file is not valid UTF-8 at this line", notUTF8Hint)
}
