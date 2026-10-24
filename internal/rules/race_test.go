//go:build race

package rules

// timeLimitFactor stretches the time limits of the fuzz and performance
// tests under the race detector, which slows the run several times over. The
// limits themselves (NFR1.11, NFR1.12, NFR8.13) are checked by the runs
// without -race on the three operating systems.
const timeLimitFactor = 10

// perfScale divides the size of the performance tests under the race
// detector, so the CI race job stays within its time.
const perfScale = 10
