package redact

import (
	"regexp"
	"strings"
)

// The shapes Scan looks for in the body a user gives to a pull request
// (v0.2.0 U2, security design §2, NFR1.6, NFR1.7). The first three stop the
// run: they are values that must not be made public. The last two are only
// told about: an ARN and a twelve-digit number can be meant by the user.
const (
	KindGitHubToken = "githubToken"
	KindAccessKeyID = "accessKeyID"
	KindURLPassword = "urlPassword"
	KindARN         = "arn"
	KindAccountID   = "accountID"
)

// Finding is one shape found in a text: its kind and the lines it is on. It
// holds no value.
type Finding struct {
	Kind string
	// Stop is true when the shape stops the run.
	Stop bool
	// Lines are the numbers of the lines with the shape, from 1, in increasing
	// order, each once.
	Lines []int
}

// shape is one shape of Scan: its pattern and the check of its edges, or a
// search of its own (find) when the pattern has no fixed length.
type shape struct {
	kind string
	stop bool
	re   *regexp.Regexp
	// counts reports whether the match line[start:end] counts.
	counts func(line string, start, end int) bool
	// find, when set, is used instead of re and counts.
	find func(line string) bool
}

var (
	scanURLPassword = regexp.MustCompile(`://[^/@\s:]+:[^/@\s]+@`)
	scanAccountID   = regexp.MustCompile(`[0-9]{12}`)
)

// shapes are the shapes of Scan, in the order of its result.
var shapes = []shape{
	{kind: KindGitHubToken, stop: true, find: gitHubTokenIn},
	{kind: KindAccessKeyID, stop: true, re: accessKeyID, counts: func(line string, start, end int) bool {
		return (start == 0 || !upperOrDigit(line[start-1])) && (end == len(line) || !upperOrDigit(line[end]))
	}},
	{kind: KindURLPassword, stop: true, re: scanURLPassword},
	{kind: KindARN, re: arn},
	{kind: KindAccountID, re: scanAccountID, counts: func(line string, start, end int) bool {
		return (start == 0 || !digit(line[start-1])) && (end == len(line) || !digit(line[end]))
	}},
}

// gitHubTokenPrefixes are the prefixes of a GitHub token; all start with g.
var gitHubTokenPrefixes = []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_"}

// gitHubTokenRest is how many word characters a token has at least after its
// prefix.
const gitHubTokenRest = 20

// gitHubTokenIn reports whether line holds a GitHub token: a prefix with no
// word character right before it, then at least twenty word characters. It is
// the same as the pattern
//
//	(?:ghp_|gho_|ghu_|ghs_|ghr_|github_pat_)[A-Za-z0-9_]{20,}
//
// with the check of the left edge, but it looks at each place of g once and
// at no more than twenty characters after it, so that a long line of words is
// read in a time that grows with its length (review N1): the pattern matches
// up to the end of such a line at each place it is tried.
func gitHubTokenIn(line string) bool {
	for from := 0; ; {
		i := strings.IndexByte(line[from:], 'g')
		if i < 0 {
			return false
		}
		start := from + i
		from = start + 1
		if start > 0 && wordByte(line[start-1]) {
			continue
		}
		for _, prefix := range gitHubTokenPrefixes {
			if strings.HasPrefix(line[start:], prefix) && wordRun(line[start+len(prefix):], gitHubTokenRest) {
				return true
			}
		}
	}
}

// wordRun reports whether s starts with at least n word characters.
func wordRun(s string, n int) bool {
	if len(s) < n {
		return false
	}
	for i := 0; i < n; i++ {
		if !wordByte(s[i]) {
			return false
		}
	}
	return true
}

func digit(b byte) bool        { return '0' <= b && b <= '9' }
func upperOrDigit(b byte) bool { return digit(b) || 'A' <= b && b <= 'Z' }
func wordByte(b byte) bool {
	return upperOrDigit(b) || 'a' <= b && b <= 'z' || b == '_'
}

// Scan looks for the shapes in text, line by line; lines end at \n only. It
// returns the kinds found, in the fixed order of the kinds, with their lines.
func Scan(text string) []Finding {
	lines := strings.Split(text, "\n")
	var found []Finding
	for _, s := range shapes {
		var at []int
		for i, line := range lines {
			if s.in(line) {
				at = append(at, i+1)
			}
		}
		if len(at) > 0 {
			found = append(found, Finding{Kind: s.kind, Stop: s.stop, Lines: at})
		}
	}
	return found
}

// in reports whether the shape is in line. A match whose edges do not count is
// passed over, and the search starts again at the byte after its start, so
// that a real shape overlapping it is still found. The patterns searched this
// way have a fixed length, so a match passed over costs a few bytes only.
func (s shape) in(line string) bool {
	if s.find != nil {
		return s.find(line)
	}
	for from := 0; from < len(line); {
		loc := s.re.FindStringIndex(line[from:])
		if loc == nil {
			return false
		}
		start, end := from+loc[0], from+loc[1]
		if s.counts == nil || s.counts(line, start, end) {
			return true
		}
		from = start + 1
	}
	return false
}
