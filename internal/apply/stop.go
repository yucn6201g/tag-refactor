package apply

import (
	"sync"

	"github.com/yucn6201g/tag-refactor/internal/errkind"
)

// StopReason says why a run stopped before it wrote everything (security
// design §5).
type StopReason string

// The reasons a run stops writing.
const (
	StopNone StopReason = ""
	// StopSignal: Ctrl+C (the first one; BR4.5).
	StopSignal StopReason = "signal"
	// StopCredentials: the credentials are missing or expired (BR4.2).
	StopCredentials StopReason = "credentials"
	// StopTransient: transientLimit write calls in a row ended Transient
	// (NFR5.17).
	StopTransient StopReason = "transient"
	// StopStorage: the record can no longer be saved (BR4.4).
	StopStorage StopReason = "storage"
	// StopInternal: an unexpected error, such as a panic of a goroutine.
	StopInternal StopReason = "internal"
)

// stopper is the stop signal of a run (the first stage of security design
// §5) and the counter of write calls that ended Transient in a row, shared
// by every region.
type stopper struct {
	mu        sync.Mutex
	ch        chan struct{}
	reason    StopReason
	err       error
	transient int
}

func newStopper() *stopper { return &stopper{ch: make(chan struct{})} }

// TransientLimit is the number of write calls in a row that may end
// Transient before a run stops (NFR5.17), for the message of the stop.
func TransientLimit() int { return transientLimit }

// stop raises the signal; the first reason and error are kept.
func (s *stopper) stop(reason StopReason, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reason != StopNone {
		return
	}
	s.reason, s.err = reason, err
	close(s.ch)
}

// stopped reports whether the signal is up.
func (s *stopper) stopped() bool {
	select {
	case <-s.ch:
		return true
	default:
		return false
	}
}

func (s *stopper) why() (StopReason, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason, s.err
}

// callEnded counts one write call (NFR5.17): a call that worked, even with
// failures of some resources, starts the count again; a call that ended
// Transient as a whole adds one, and transientLimit of them stop the run;
// missing or expired credentials stop it at once.
func (s *stopper) callEnded(err error) {
	switch errkind.KindOf(err) {
	case errkind.CredentialsMissing:
		s.stop(StopCredentials, err)
		return
	case errkind.Transient:
	default:
		if err == nil {
			s.mu.Lock()
			s.transient = 0
			s.mu.Unlock()
		}
		return
	}
	s.mu.Lock()
	s.transient++
	n := s.transient
	s.mu.Unlock()
	if n >= transientLimit {
		s.stop(StopTransient, err)
	}
}
