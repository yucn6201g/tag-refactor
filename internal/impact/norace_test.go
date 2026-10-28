//go:build !race

package impact

// Without the race detector the time limits of the performance tests and of
// one fuzz input stay as they are (NFR1.5, NFR1.17, NFR8.21), and the large
// tests use the full size of the measuring conditions.
const (
	timeLimitFactor = 1
	perfScale       = 1
)
