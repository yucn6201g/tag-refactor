package display

import (
	"runtime"
	"strings"
)

// Lang is the language of everything people read, except --help (BR3.9).
type Lang string

// The two languages (contract C5 --lang).
const (
	English  Lang = "en"
	Japanese Lang = "ja"
)

// ParseLang reads a --lang value: en or ja.
func ParseLang(s string) (Lang, bool) {
	switch Lang(s) {
	case English, Japanese:
		return Lang(s), true
	}
	return "", false
}

// uiLanguage returns the first preferred display language of the user, such
// as "ja-JP", on Windows, and "" elsewhere or when it cannot be read. It is
// a variable so that tests can stand in for the operating system.
var uiLanguage = windowsUILanguage

// ResolveLang decides the language without --lang (BR3.8, NFR9.7): the
// first of LC_ALL, LC_MESSAGES and LANG that is not empty, Japanese when it
// starts with "ja", otherwise English. On Windows only, when the three are
// empty, the first display language of the user decides in the same way. A
// display language that cannot be read gives English.
func ResolveLang(getenv func(string) string) Lang {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(name); v != "" {
			return langOf(v)
		}
	}
	if runtime.GOOS == "windows" {
		return langOf(uiLanguage())
	}
	return English
}

func langOf(locale string) Lang {
	if strings.HasPrefix(strings.ToLower(locale), "ja") {
		return Japanese
	}
	return English
}
