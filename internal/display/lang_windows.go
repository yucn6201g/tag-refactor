//go:build windows

package display

import "golang.org/x/sys/windows"

// windowsUILanguage returns the first of the user's preferred display
// languages as a name such as "ja-JP", or "" when Windows cannot tell
// (NFR9.7).
func windowsUILanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return ""
	}
	return langs[0]
}
