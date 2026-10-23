//go:build race

package impact

// timeLimitFactor stretches the time limits of the performance tests under
// the race detector, which slows the run several times over. The limits
// themselves (NFR1.5) are checked by the runs without -race.
const timeLimitFactor = 10
