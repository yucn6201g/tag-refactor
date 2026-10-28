//go:build race

package impact

// Under the race detector, which slows the run several times over, the time
// limits are ten times longer and the large tests ten times smaller (as in
// U2 to U4). The limits themselves are checked by the runs without -race.
const (
	timeLimitFactor = 10
	perfScale       = 10
)
