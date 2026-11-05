//go:build race

package iac

// largeScale divides the size of the large test under the race detector, which
// slows the run several times over (U2 to U7 do the same).
const largeScale = 10

// largeTimeFactor stretches the time limit under the race detector; the limit
// itself is checked by the runs without -race.
const largeTimeFactor = 10
