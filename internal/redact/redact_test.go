package redact

// Unit tests of redact, written after the implementation (team testing
// posture): the five shapes of the closed list, their edges, and the lines of
// standard error that are kept.

import (
	"strings"
	"testing"
)

func TestARNsAreHiddenUpToTheirEnd(t *testing.T) {
	cases := map[string]string{
		"arn:aws:s3:::bucket-1 is there":                       "<redacted> is there",
		`"arn:aws-cn:iam::123456789012:role/x" quoted`:         `"<redacted>" quoted`,
		"(arn:aws:sqs:ap-northeast-1:111122223333:q)":          "(<redacted>)",
		"`arn:aws-us-gov:kms:us-gov-west-1:1:key/k`, next":     "`<redacted>`, next",
		"[arn:aws:sns:us-east-1:1:t];<arn:aws:lambda:x:1:f:y>": "[<redacted>];<<redacted>>",
		"not an arn: arnaws:s3:::b":                            "not an arn: arnaws:s3:::b",
	}
	for in, want := range cases {
		if got := String(in); got != want {
			t.Errorf("String(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTwelveDigitsAloneAreHidden(t *testing.T) {
	cases := map[string]string{
		"account 123456789012.":   "account <redacted>.",
		"plan-123456789012-x":     "plan-<redacted>-x",
		"a123456789012b":          "a<redacted>b",
		"11 digits 12345678901":   "11 digits 12345678901",
		"13 digits 1234567890123": "13 digits 1234567890123",
	}
	for in, want := range cases {
		if got := String(in); got != want {
			t.Errorf("String(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAccessKeysAndGitHubTokensAreHidden(t *testing.T) {
	in := "AKIAABCDEFGHIJKLMNOP ASIA0123456789ABCDEF ghp_abcDEF123 gho_x ghu_y ghs_z ghr_w github_pat_11AB_cd"
	got := String(in)
	if strings.Contains(got, "AKIA") || strings.Contains(got, "ASIA") || strings.Contains(got, "gh") {
		t.Fatalf("String(%q) = %q", in, got)
	}
	if n := strings.Count(got, Placeholder); n != 8 {
		t.Fatalf("%d placeholders in %q, want 8", n, got)
	}
	if kept := String("AKIA is a word"); kept != "AKIA is a word" {
		t.Errorf("a word was hidden: %q", kept)
	}
}

func TestUserInformationOfURLsIsHidden(t *testing.T) {
	remote := "https://user:ghp_secret@github.com/o/r.git"
	if got := String(remote); got != "https://<redacted>@github.com/o/r.git" {
		t.Errorf("String = %q", got)
	}
	if got := URL(remote); got != "https://github.com/o/r.git" {
		t.Errorf("URL = %q", got)
	}
	if got := URL("git@github.com:o/r.git"); got != "git@github.com:o/r.git" {
		t.Errorf("an scp-like URL changed: %q", got)
	}
}

func TestNothingElseChanges(t *testing.T) {
	for _, s := range []string{"", "plain text", "Tags.of(bucket).add('Env', 'dev')", "日本語のメッセージ"} {
		if got := String(s); got != s {
			t.Errorf("String(%q) = %q", s, got)
		}
	}
}

// Security design §4: only the known lines of standard error are kept, and
// they are redacted too.
func TestLinesKeepsOnlyKnownLines(t *testing.T) {
	stderr := strings.Join([]string{
		"lib/stack.ts(9,7): error TS2304: Cannot find name 'bucket'.",
		"lib/stack.ts:9:7 - error TS2304: Cannot find name 'x'.",
		"    const secret = 'do not show this';",
		"           ~~~~~~",
		"Error: Need to perform AWS calls for account 123456789012",
		"TypeError [ERR_X]: something",
		"    at new SkeletonStack (C:\\app\\lib\\stack.ts:9:20)",
		"tag-refactor-helper: typescript is not found from the app",
		"",
	}, "\r\n")

	kept, dropped := Lines([]byte(stderr))

	if dropped != 2 {
		t.Errorf("dropped %d lines, want 2", dropped)
	}
	if len(kept) != 6 {
		t.Fatalf("kept %d lines: %q", len(kept), kept)
	}
	joined := strings.Join(kept, "\n")
	if strings.Contains(joined, "do not show") || strings.Contains(joined, "123456789012") {
		t.Fatalf("kept %q", joined)
	}
	if !strings.Contains(joined, "Error: Need to perform AWS calls for account <redacted>") {
		t.Fatalf("the error line is not kept redacted: %q", joined)
	}
}
