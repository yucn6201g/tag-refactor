//go:build !race

package planner

// perfScale divides the size of the performance input; timeFactor stretches
// the time limits. Without the race detector the full size and the limits of
// NFR1.13 and NFR1.15 apply, and one fuzz input has 1 second (NFR8.18).
const (
	perfScale  = 1
	timeFactor = 1
)
