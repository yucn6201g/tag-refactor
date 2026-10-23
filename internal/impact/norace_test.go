//go:build !race

package impact

// timeLimitFactor keeps the time limits of the performance tests as they are
// without the race detector (NFR1.5).
const timeLimitFactor = 1
