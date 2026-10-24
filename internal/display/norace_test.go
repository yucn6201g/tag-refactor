//go:build !race

package display

// perfScale divides the size of the performance input of NFR1.14 and
// timeFactor stretches its limit; without the race detector the full size
// and the 3-second limit apply.
const (
	perfScale  = 1
	timeFactor = 1
)
