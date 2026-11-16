package cdk

import (
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The home folder in the lines of --debug (v0.2.0 U3 NFR1.12, security design
// §5): the paths of the apps, of the .venv, of the --app command, of the
// answer of python --version and of the kept lines of standard error write
// the home folder as ~, so that the name of the user does not show.

// homeForms returns the ways of writing home, longest first: its clean form
// and, when eval (filepath.EvalSymlinks) makes another, its real form too. The
// paths of the apps come from the real root, so both must be found.
func homeForms(home string, eval func(string) (string, error)) []string {
	clean := filepath.Clean(home)
	forms := []string{clean}
	if real, err := eval(clean); err == nil && real != clean && real != "" {
		forms = append(forms, filepath.Clean(real))
	}
	slices.SortStableFunc(forms, func(a, b string) int { return len(b) - len(a) })
	return forms
}

// shortHome writes each part of s that is one of homes as ~: from the left, at
// each place the first of homes (the longest) that matches is used, when the
// end of s or a character that is not a letter, a digit, _, - or . follows
// it. Windows compares without case.
func shortHome(s string, homes []string, windows bool) string {
	if len(homes) == 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		matched := 0
		for _, h := range homes {
			if h == "" || len(s)-i < len(h) {
				continue
			}
			part := s[i : i+len(h)]
			if part != h && (!windows || !strings.EqualFold(part, h)) {
				continue
			}
			if next := i + len(h); next < len(s) {
				r, _ := utf8.DecodeRuneInString(s[next:])
				if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
					continue
				}
			}
			matched = len(h)
			break
		}
		if matched > 0 {
			b.WriteByte('~')
			i += matched
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(s[i : i+size])
		i += size
	}
	return b.String()
}
