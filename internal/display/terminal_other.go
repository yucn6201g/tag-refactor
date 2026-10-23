//go:build !windows

package display

import "io"

// enableVT reports true: terminals on Linux and macOS understand the cursor
// sequences without any setting.
func enableVT(io.Writer) bool { return true }
