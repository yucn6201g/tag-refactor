//go:build race

package planner

// Under the race detector, which slows the run several times over, the
// performance input is ten times smaller and the time limits, the one of a
// fuzz input included, ten times longer (as in U2 and U3). The limits
// themselves are checked by the runs without -race on the three operating
// systems.
const (
	perfScale  = 10
	timeFactor = 10
)
