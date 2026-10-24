package planner

import (
	"io"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// SafeJSONWriter is the post-processing of every JSON tag-refactor writes:
// the plan file (contract C2) and the JSON output of scan and plan (contract
// C3) (U4 security design §2, NFR8.15). It receives the bytes encoding/json
// wrote and replaces the characters that encoding/json leaves as they are
// although they can rewrite a terminal or hide text: DEL (U+007F), the C1
// control characters (U+0080 to U+009F) and the format characters (Cf, the
// bidirectional controls such as U+202E included). Each becomes a lowercase
// \uxxxx escape; a format character outside the basic plane, such as
// U+E0001, becomes the two escapes of its UTF-16 surrogate pair. JSON's own
// syntax is ASCII, so these characters only occur inside strings, where the
// escapes keep the JSON valid and decode to the same string. encoding/json
// already escapes the other control characters, <, >, &, U+2028 and U+2029,
// and replaces invalid UTF-8 with U+FFFD.
//
// A UTF-8 character split between two calls of Write is carried over to the
// next call; Close writes what is left.
type SafeJSONWriter struct {
	w io.Writer
	// carry holds the first bytes of a character the last Write ended in.
	carry []byte
	out   []byte
	err   error
}

// NewSafeJSONWriter returns a SafeJSONWriter that writes to w.
func NewSafeJSONWriter(w io.Writer) *SafeJSONWriter {
	return &SafeJSONWriter{w: w}
}

// needsEscape reports whether r is left unescaped by encoding/json but must
// not be written as it is.
func needsEscape(r rune) bool {
	return r == 0x7f || (r >= 0x80 && r <= 0x9f) || unicode.Is(unicode.Cf, r)
}

const hexDigits = "0123456789abcdef"

func appendEscape(out []byte, r rune) []byte {
	if r > 0xffff {
		hi, lo := utf16.EncodeRune(r)
		return appendEscape(appendEscape(out, hi), lo)
	}
	return append(out, '\\', 'u', hexDigits[r>>12&0xf], hexDigits[r>>8&0xf], hexDigits[r>>4&0xf], hexDigits[r&0xf])
}

// Write implements io.Writer. It reports len(p) once p, apart from the end
// of a character carried over, has been written.
func (s *SafeJSONWriter) Write(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	data := p
	if len(s.carry) > 0 {
		data = append(s.carry, p...)
		s.carry = nil
	}
	end := completeLength(data)
	if end < len(data) {
		s.carry = append([]byte(nil), data[end:]...)
	}
	data = data[:end]
	if !hasHighByte(data) {
		if len(data) > 0 {
			if _, err := s.w.Write(data); err != nil {
				s.err = err
				return 0, err
			}
		}
		return len(p), nil
	}
	out := s.out[:0]
	for i := 0; i < len(data); {
		b := data[i]
		if b < 0x7f {
			out = append(out, b)
			i++
			continue
		}
		r, size := utf8.DecodeRune(data[i:])
		if (size > 1 || r == 0x7f) && needsEscape(r) {
			out = appendEscape(out, r)
		} else {
			out = append(out, data[i:i+size]...)
		}
		i += size
	}
	s.out = out
	if _, err := s.w.Write(out); err != nil {
		s.err = err
		return 0, err
	}
	return len(p), nil
}

// Close writes the bytes of a character that never ended. encoding/json
// never leaves one, so this only matters for a writer fed by hand.
func (s *SafeJSONWriter) Close() error {
	if s.err != nil {
		return s.err
	}
	if len(s.carry) > 0 {
		_, err := s.w.Write(s.carry)
		s.carry = nil
		if err != nil {
			s.err = err
			return err
		}
	}
	return nil
}

func hasHighByte(b []byte) bool {
	for _, c := range b {
		if c >= 0x7f {
			return true
		}
	}
	return false
}

// completeLength returns the length of data without the first bytes of a
// character that may still be completed by the next Write.
func completeLength(data []byte) int {
	for back := 1; back <= utf8.UTFMax-1 && back <= len(data); back++ {
		i := len(data) - back
		c := data[i]
		if c < 0x80 {
			return len(data)
		}
		if c >= 0xc0 {
			if !utf8.FullRune(data[i:]) {
				return i
			}
			return len(data)
		}
	}
	return len(data)
}
