//go:build !race

package redact

// timeFactor stretches the time limit of the long-line test under the race
// detector; without it the limit is checked as it is.
const timeFactor = 1
