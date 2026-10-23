//go:build race

package cli_test

// perfScale divides the size of the large fake account under the race
// detector, which slows the run several times over, so the CI race job stays
// within its 15 minutes. The run without -race keeps the full size; the
// call-count bound is checked at both sizes.
const perfScale = 10

// timeLimitFactor stretches the time limits of the U1 performance tests under
// the race detector. The limits themselves (NFR1.4, NFR1.5) are checked by
// the runs without -race on the three operating systems.
const timeLimitFactor = 10
