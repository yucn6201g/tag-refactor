package planner

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Unit tests of SafeJSONWriter (NFR8.15, U4 security design §2).

// esc is the JSON escape of the UTF-16 code unit hex. It is built from two
// parts so that the source holds neither the escape nor the character.
func esc(hex string) string { return `\` + "u" + hex }

// unprintable has a character of each kind the writer escapes, written from
// their code points so the source holds none of them.
var unprintable = "del" + string(rune(0x7f)) + " c1" + string(rune(0x80)) + string(rune(0x9b)) + string(rune(0x9f)) +
	" bidi" + string(rune(0x202e)) + string(rune(0x2066)) + " zw" + string(rune(0x200b)) + string(rune(0xfeff)) +
	" soft" + string(rune(0xad)) + " tag" + string(rune(0xe0001)) + string(rune(0xe007f)) + " music" + string(rune(0x1d173)) +
	" kept é日本語😀" + string(rune(0x00a0))

func safeJSON(t *testing.T, v any, chunk int) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	w := NewSafeJSONWriter(&out)
	if chunk <= 0 {
		chunk = len(data)
	}
	for i := 0; i < len(data); i += chunk {
		end := min(i+chunk, len(data))
		n, err := w.Write(data[i:end])
		if err != nil || n != end-i {
			t.Fatalf("Write = %d, %v", n, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestSafeJSONWriterEscapesWhatEncodingJSONLeaves(t *testing.T) {
	got := safeJSON(t, unprintable, 0)

	want := `"del` + esc("007f") + ` c1` + esc("0080") + esc("009b") + esc("009f") +
		` bidi` + esc("202e") + esc("2066") + ` zw` + esc("200b") + esc("feff") + ` soft` + esc("00ad") +
		` tag` + esc("db40") + esc("dc01") + esc("db40") + esc("dc7f") + ` music` + esc("d834") + esc("dd73") +
		` kept é日本語😀` + string(rune(0x00a0)) + `"`
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	var back string
	if err := json.Unmarshal([]byte(got), &back); err != nil || back != unprintable {
		t.Fatalf("read back %q, %v", back, err)
	}
}

// A character split between two calls of Write is carried over: writing one
// byte at a time gives the same bytes.
func TestSafeJSONWriterGivesTheSameBytesWhateverTheWrites(t *testing.T) {
	v := map[string]any{"k" + unprintable: []string{unprintable, "plain", "\xff\xfe broken"}}
	whole := safeJSON(t, v, 0)
	for _, chunk := range []int{1, 2, 3, 5, 7} {
		if got := safeJSON(t, v, chunk); got != whole {
			t.Fatalf("chunks of %d bytes:\n%q\nwhole:\n%q", chunk, got, whole)
		}
	}
}

func TestSafeJSONWriterWritesWhatIsLeftOnClose(t *testing.T) {
	var out bytes.Buffer
	w := NewSafeJSONWriter(&out)
	// The first two bytes of a three-byte character, then nothing more.
	if n, err := w.Write([]byte("ab\xe6\x97")); n != 4 || err != nil {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if out.String() != "ab" {
		t.Fatalf("before Close: %q", out.String())
	}
	if err := w.Close(); err != nil || out.String() != "ab\xe6\x97" {
		t.Fatalf("after Close: %q, %v", out.String(), err)
	}
	if err := w.Close(); err != nil {
		t.Fatal("a second Close does nothing")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestSafeJSONWriterKeepsTheFirstError(t *testing.T) {
	w := NewSafeJSONWriter(brokenWriter{})
	if _, err := w.Write([]byte("plain")); err == nil {
		t.Fatal("the error of the writer below must come back")
	}
	if _, err := w.Write([]byte("x")); err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("later writes keep failing: %v", err)
	}
	if w.Close() == nil {
		t.Fatal("Close keeps failing")
	}
	w2 := NewSafeJSONWriter(brokenWriter{})
	if _, err := w2.Write([]byte("caf\xc3\xa9" + string(rune(0x202e)))); err == nil {
		t.Fatal("the escaped path returns the error too")
	}
	w3 := NewSafeJSONWriter(brokenWriter{})
	_, _ = w3.Write([]byte("\xe6"))
	if w3.Close() == nil {
		t.Fatal("Close returns the error of writing what was left")
	}
}

// ASCII passes through as it is, invalid bytes too.
func TestSafeJSONWriterLeavesOtherBytesAlone(t *testing.T) {
	var out bytes.Buffer
	w := NewSafeJSONWriter(&out)
	in := "{\"a\": \"plain text, " + esc("001b") + " already escaped\"}\x80 stray"
	if _, err := w.Write([]byte(in)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if out.String() != in {
		t.Fatalf("got %q", out.String())
	}
}
