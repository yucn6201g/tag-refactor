//go:build !windows

package display

// windowsUILanguage is "" outside Windows: Linux and macOS have only the
// environment variables (NFR9.7).
func windowsUILanguage() string { return "" }
