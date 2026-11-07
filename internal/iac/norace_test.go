//go:build !race

package iac

// largeScale keeps the full size of the large test without the race detector
// (NFR5.25).
const largeScale = 1

// largeTimeFactor keeps the time limit of NFR5.25 as it is.
const largeTimeFactor = 1
