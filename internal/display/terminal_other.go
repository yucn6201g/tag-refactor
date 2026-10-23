//go:build !windows

package display

import "io"

// enableVT reports true with nothing to restore: terminals on Linux and macOS
// understand the cursor sequences without any setting.
func enableVT(io.Writer) (bool, func()) { return true, nil }
