//go:build race

package redact

// Under the race detector the time limit of the long-line test is ten times
// longer (as in the other packages); the runs without -race check the limit
// itself.
const timeFactor = 10
