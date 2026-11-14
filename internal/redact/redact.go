// Package redact hides the values that must not reach the screen, the
// --debug lines, or the title, body and commit message of a pull request
// (v0.2.0 U1, security design §4, NFR1.2 to NFR1.4). The list of shapes is
// closed: ARNs, twelve-digit numbers (account IDs), AWS access key IDs, GitHub
// tokens and the user information of URLs. Nothing else is hidden; adding a
// shape means changing this list, the design and the tests together.
//
// It also keeps only known lines of the standard error of cdk synth and of the
// helper (Lines): the other lines can quote the source of the user's app.
package redact

import (
	"regexp"
	"strings"
)

// Placeholder replaces every hidden value (contract X2).
const Placeholder = "<redacted>"

var (
	// userInfo is what lies between :// and @ in a URL, without a slash.
	userInfo = regexp.MustCompile(`://[^/@\s]+@`)
	// arn runs from arn:aws, letters or hyphens and a colon, up to a space, a
	// quote, a comma, a semicolon, a closing bracket or an angle bracket.
	arn = regexp.MustCompile("arn:aws[A-Za-z-]*:[^\\s\"'`,;)\\]}<>]*")
	// accessKeyID is an AWS access key ID.
	accessKeyID = regexp.MustCompile(`(?:AKIA|ASIA)[A-Z0-9]{16}`)
	// githubToken is a GitHub token by its prefix.
	githubToken = regexp.MustCompile(`(?:ghp_|gho_|ghu_|ghs_|ghr_|github_pat_)[A-Za-z0-9_]+`)
	// digits is a run of digits; only runs of exactly twelve are hidden.
	digits = regexp.MustCompile(`[0-9]+`)
)

// String returns s with every shape of the list replaced by Placeholder.
func String(s string) string {
	s = userInfo.ReplaceAllString(s, "://"+Placeholder+"@")
	s = arn.ReplaceAllString(s, Placeholder)
	s = accessKeyID.ReplaceAllString(s, Placeholder)
	s = githubToken.ReplaceAllString(s, Placeholder)
	return digits.ReplaceAllStringFunc(s, func(run string) string {
		if len(run) == 12 {
			return Placeholder
		}
		return run
	})
}

// URL returns a URL without its user information, for showing a remote
// (NFR1.3).
func URL(u string) string {
	return userInfo.ReplaceAllString(u, "://")
}

// The lines of standard error that are kept (security design §4): the first
// line of a TypeScript diagnostic, the name and message of an exception, a
// line of a stack trace, and the lines the helper writes itself.
var keptLines = []*regexp.Regexp{
	regexp.MustCompile(`^\S.*\(\d+,\d+\): error TS\d+: `),
	regexp.MustCompile(`^\S.*:\d+:\d+ - error TS\d+: `),
	regexp.MustCompile(`^\s*[A-Za-z]*Error(?: \[[A-Z0-9_]+\])?: `),
	regexp.MustCompile(`^\s*at `),
	regexp.MustCompile(`^tag-refactor-helper: `),
}

// Lines keeps the known lines of the standard error of synth or the helper,
// hides the shapes of the list in them, and counts the lines it dropped.
func Lines(stderr []byte) (kept []string, dropped int) {
	text := strings.ReplaceAll(string(stderr), "\r\n", "\n")
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if known(line) {
			kept = append(kept, String(line))
			continue
		}
		dropped++
	}
	return kept, dropped
}

func known(line string) bool {
	for _, re := range keptLines {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}
