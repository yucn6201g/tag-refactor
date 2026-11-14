package redact

// Unit tests of Scan (v0.2.0 U2, security design §2), written after the
// implementation. The shapes of tokens and keys are put together here at run
// time: they are no real values.

import (
	"math/rand/v2"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func token(n int) string      { return "gh" + "p_" + strings.Repeat("x", n) }
func accessKey() string       { return "AK" + "IA" + strings.Repeat("Q7", 8) }
func urlWithPassword() string { return "https://" + "u" + ":" + "p" + "@h/x" }

// NFR1.6, NFR1.7: every shape is found with its lines, in the fixed order of
// the kinds, whatever the order in the text; the first three stop the run.
func TestScanFindsEachShapeOnItsLines(t *testing.T) {
	text := strings.Join([]string{
		"see arn:aws:sqs:ap-northeast-1:111122223333:orders",
		"plain line",
		"url " + urlWithPassword(),
		"key " + accessKey(),
		"token " + token(20),
	}, "\n")

	got := Scan(text)

	want := []Finding{
		{Kind: KindGitHubToken, Stop: true, Lines: []int{5}},
		{Kind: KindAccessKeyID, Stop: true, Lines: []int{4}},
		{Kind: KindURLPassword, Stop: true, Lines: []int{3}},
		{Kind: KindARN, Lines: []int{1}},
		{Kind: KindAccountID, Lines: []int{1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v\nwant %+v", got, want)
	}
	if Scan("nothing here\n") != nil || Scan("") != nil {
		t.Fatal("a text without shapes has findings")
	}
}

// The edges of a GitHub token: twenty characters after the prefix and nothing
// of a word right before it.
func TestScanEdgesOfAGitHubToken(t *testing.T) {
	for text, want := range map[string]bool{
		token(19):                              false,
		token(20):                              true,
		"(" + token(20) + ")":                  true,
		"a" + token(20):                        false,
		"_" + token(20):                        false,
		"github_pat_" + "x123456789_123456789": true,
		"ghx_" + strings.Repeat("x", 30):       false,
	} {
		if got := len(Scan(text)) == 1 && Scan(text)[0].Kind == KindGitHubToken; got != want {
			t.Errorf("Scan(%q) finds a token: %v, want %v", text, got, want)
		}
	}
}

// The edges of an AWS access key ID: exactly sixteen upper-case letters or
// digits, with none of them right before or after.
func TestScanEdgesOfAnAccessKeyID(t *testing.T) {
	key := accessKey()
	for text, want := range map[string]bool{
		key:                                true,
		"key=" + key + ".":                 true,
		"x" + key:                          true,
		"X" + key:                          false,
		"7" + key:                          false,
		key + "Z":                          false,
		key[:len(key)-1]:                   false,
		"AS" + "IA" + key[4:]:              true,
		strings.ToLower(key[:4]) + key[4:]: false,
	} {
		got := false
		for _, f := range Scan(text) {
			got = got || f.Kind == KindAccessKeyID
		}
		if got != want {
			t.Errorf("Scan(%q) finds a key: %v, want %v", text, got, want)
		}
	}
}

// Security design §2: after a match that does not count, the search goes on in
// the same line, from the byte after its start; a run of thirteen digits is no
// twelve-digit number at any place.
func TestScanLooksAgainAfterAMatchThatDoesNotCount(t *testing.T) {
	if got := Scan("a" + token(20) + " " + token(25)); len(got) != 1 || got[0].Kind != KindGitHubToken {
		t.Fatalf("the second token was missed: %+v", got)
	}
	if got := Scan("X" + accessKey() + " " + accessKey()); len(got) != 1 || got[0].Kind != KindAccessKeyID {
		t.Fatalf("the second key was missed: %+v", got)
	}
	if got := Scan("1234567890123 and 98765432109876"); got != nil {
		t.Fatalf("a long run of digits counts: %+v", got)
	}
	if got := Scan("id 1234567890123 or 123456789012"); len(got) != 1 || got[0].Kind != KindAccountID {
		t.Fatalf("the twelve digits after a long run were missed: %+v", got)
	}
}

// Review N1: a line of 60,000 characters made of tokens that do not count (a
// word character right before each prefix), and other long lines of passed
// over matches, are read in a time that grows with their length. Before the
// fix, the first line took about 8.7 seconds.
func TestScanOfALongLineIsQuick(t *testing.T) {
	lines := []string{
		strings.Repeat("a"+"gh"+"p_a", 10000),
		strings.Repeat("x"+"github"+"_pat_", 6000),
		strings.Repeat("g", 60000),
		strings.Repeat("7", 60000),
		strings.Repeat("X"+accessKey(), 3000),
	}
	for _, line := range lines {
		begin := time.Now()

		got := Scan(line)

		if took := time.Since(begin); took > timeFactor*time.Second {
			t.Errorf("a line of %q... took %v", line[:12], took)
		}
		if got != nil {
			t.Errorf("a line of %q... has %+v", line[:12], got)
		}
	}
	if got := Scan(strings.Repeat("a"+"gh"+"p_a", 10000) + " " + token(20)); len(got) != 1 || got[0].Kind != KindGitHubToken {
		t.Fatalf("the token at the end of a long line was missed: %+v", got)
	}
}

// Review N1: the search of a GitHub token finds what the pattern with the
// check of the left edge finds (the search before the fix), on lines made of
// the pieces that matter at the edges.
func TestScanFindsTokensAsThePatternDoes(t *testing.T) {
	pattern := regexp.MustCompile(`(?:gh` + `p_|gho_|ghu_|ghs_|ghr_|github` + `_pat_)[A-Za-z0-9_]{20,}`)
	byPattern := func(line string) bool {
		for from := 0; from < len(line); {
			loc := pattern.FindStringIndex(line[from:])
			if loc == nil {
				return false
			}
			start := from + loc[0]
			if start == 0 || !wordByte(line[start-1]) {
				return true
			}
			from = start + 1
		}
		return false
	}
	pieces := []string{"gh", "p_", "o_", "github", "_pat_", "ghx_", "a", "_", "-", " ", "Z9", strings.Repeat("b", 9), strings.Repeat("c", 19)}
	r := rand.New(rand.NewPCG(1, 2))
	found := 0
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := r.IntN(12); n >= 0; n-- {
			b.WriteString(pieces[r.IntN(len(pieces))])
		}
		line := b.String()
		want := byPattern(line)
		if got := gitHubTokenIn(line); got != want {
			t.Fatalf("gitHubTokenIn(%q) = %v, the pattern gives %v", line, got, want)
		}
		if want {
			found++
		}
	}
	if found < 100 {
		t.Fatalf("only %d lines with a token were tried", found)
	}
}

// A line counts once for a shape, however many times it has it; the lines are
// all kept, in increasing order (the display shows ten of them).
func TestScanCountsLinesOnce(t *testing.T) {
	var lines []string
	for i := 0; i < 12; i++ {
		lines = append(lines, "x "+token(20)+" y "+token(21))
	}

	got := Scan(strings.Join(lines, "\n"))

	if len(got) != 1 || len(got[0].Lines) != 12 || got[0].Lines[0] != 1 || got[0].Lines[11] != 12 {
		t.Fatalf("Scan = %+v", got)
	}
}

// Lines end at \n only: a \r\n counts as one end; an ssh URL with a user and no
// password is no URL with a password.
func TestScanLinesAndURLs(t *testing.T) {
	got := Scan("a\r\nssh://git@host/x\r\n" + urlWithPassword() + "\r\nb\rc " + urlWithPassword())

	want := []Finding{{Kind: KindURLPassword, Stop: true, Lines: []int{3, 4}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v", got)
	}
}
