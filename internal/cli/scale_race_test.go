//go:build race

package cli_test

// perfScale divides the size of the large fake account under the race
// detector, which slows the run several times over, so the CI race job stays
// within its 15 minutes. The run without -race keeps the full size; the
// call-count bound is checked at both sizes.
const perfScale = 10
