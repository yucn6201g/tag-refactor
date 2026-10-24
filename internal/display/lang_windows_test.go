//go:build windows

package display

import (
	"regexp"
	"testing"
)

// NFR9.7: on the Windows runners, the real call of the display language
// does not fail: it gives "" or a language name such as "ja-JP".
func TestWindowsUILanguageCallsTheSystem(t *testing.T) {
	got := windowsUILanguage()
	if got != "" && !regexp.MustCompile(`^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{1,8})*$`).MatchString(got) {
		t.Fatalf("display language = %q", got)
	}
	t.Logf("GetUserPreferredUILanguages(MUI_LANGUAGE_NAME) first language: %q", got)
}
