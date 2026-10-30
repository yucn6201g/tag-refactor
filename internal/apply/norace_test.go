//go:build !race

package apply

// largeScale keeps the full size of the large tests without the race
// detector (NFR5.16).
const largeScale = 1

// largeTimeFactor keeps the time limit of NFR5.16 as it is.
const largeTimeFactor = 1
