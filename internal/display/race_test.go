//go:build race

package display

// Under the race detector the performance input is ten times smaller and
// the limit ten times longer (as in U2 and U3); the runs without -race check
// the limit itself.
const (
	perfScale  = 10
	timeFactor = 10
)
