package display

import (
	"runtime"
	"testing"
)

// Unit tests of the language (BR3.8, NFR9.7).

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func standInUILanguage(t *testing.T, lang string) {
	t.Helper()
	old := uiLanguage
	t.Cleanup(func() { uiLanguage = old })
	uiLanguage = func() string { return lang }
}

// The first of LC_ALL, LC_MESSAGES and LANG that is set decides; ja means
// Japanese, anything else English.
func TestResolveLangReadsTheVariablesInOrder(t *testing.T) {
	standInUILanguage(t, "en-US")
	cases := []struct {
		vars map[string]string
		want Lang
	}{
		{map[string]string{"LANG": "ja_JP.UTF-8"}, Japanese},
		{map[string]string{"LANG": "fr_FR.UTF-8"}, English},
		{map[string]string{"LANG": "C"}, English},
		{map[string]string{"LANG": "JA"}, Japanese},
		{map[string]string{"LC_ALL": "ja_JP.UTF-8", "LANG": "en_US.UTF-8"}, Japanese},
		{map[string]string{"LC_ALL": "en_US.UTF-8", "LC_MESSAGES": "ja", "LANG": "ja"}, English},
		{map[string]string{"LC_ALL": "", "LC_MESSAGES": "ja_JP", "LANG": "en"}, Japanese},
		{map[string]string{"LC_MESSAGES": "", "LANG": "ja_JP.eucJP"}, Japanese},
		{map[string]string{"LANG": "jv_ID"}, English}, // Javanese is not Japanese
	}
	for _, tc := range cases {
		if got := ResolveLang(env(tc.vars)); got != tc.want {
			t.Errorf("%v: %s, want %s", tc.vars, got, tc.want)
		}
	}
}

// Without the variables, Windows asks for the display language; the other
// systems use English. A display language that cannot be read is English.
func TestResolveLangUsesTheWindowsDisplayLanguageLast(t *testing.T) {
	for _, tc := range []struct {
		ui   string
		want Lang
	}{
		{"ja-JP", Japanese},
		{"en-US", English},
		{"", English},
		{"fr-FR", English},
	} {
		standInUILanguage(t, tc.ui)
		got := ResolveLang(env(nil))
		want := tc.want
		if runtime.GOOS != "windows" {
			want = English
		}
		if got != want {
			t.Errorf("display language %q: %s, want %s", tc.ui, got, want)
		}
		if got := ResolveLang(env(map[string]string{"LANG": "en_US.UTF-8"})); got != English {
			t.Errorf("a variable wins over the display language %q: %s", tc.ui, got)
		}
	}
}

func TestParseLangAcceptsEnAndJaOnly(t *testing.T) {
	for _, s := range []string{"en", "ja"} {
		if l, ok := ParseLang(s); !ok || string(l) != s {
			t.Errorf("ParseLang(%q) = %q, %v", s, l, ok)
		}
	}
	for _, s := range []string{"", "EN", "jp", "ja_JP", "fr"} {
		if _, ok := ParseLang(s); ok {
			t.Errorf("ParseLang(%q) must fail", s)
		}
	}
}

// The language is used by every line of the printer, not by --help.
func TestSetLangChangesTheMessages(t *testing.T) {
	p, _, errOut := newPrinter()
	p.SetLang(Japanese)
	p.ExitCode(2, ExitMeaningReviewNeeded)
	p.Error("x", "y")
	if p.Lang() != Japanese || errOut.String() != "終了コード: 2(要確認)\nエラー: x\n  直し方: y\n" {
		t.Fatalf("stderr = %q", errOut.String())
	}
}
