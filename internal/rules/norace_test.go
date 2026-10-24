//go:build !race

package rules

// timeLimitFactor keeps the time limits of the fuzz and performance tests as
// they are without the race detector (NFR1.11, NFR1.12, NFR8.13).
const timeLimitFactor = 1

// perfScale keeps the full size of the performance tests.
const perfScale = 1
