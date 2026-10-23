//go:build !race

package cli_test

// perfScale divides the size of the large fake account of the U2 performance
// test. Without the race detector the full size is used (NFR1.6, NFR1.7).
const perfScale = 1
