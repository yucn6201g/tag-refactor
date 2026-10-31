package journal

import (
	"context"
	"sync"
	"time"
)

// The saving while apply or rollback writes (U6 NFR6.2, U7 NFR6.4, security
// design §3). The interval, the ticker and the limit are variables for the
// tests.
var (
	autosaveInterval = 5 * time.Second
	// newTicker starts the clock of the periodic save; stop ends it.
	newTicker = func(d time.Duration) (ticks <-chan time.Time, stop func()) {
		t := time.NewTicker(d)
		return t.C, t.Stop
	}
	// failLimit is the number of periodic saves that may fail in a row
	// before the record counts as unsavable.
	failLimit = 3
	// saveTimeout bounds one save to S3 of the periodic saver.
	saveTimeout = 30 * time.Second
)

// saver saves a record every autosaveInterval while a run writes, beside the
// writes: a tick that finds a save still running, or nothing changed since
// the last save, is skipped, and after failLimit failures in a row the record
// counts as broken. Run (apply) and RollbackRun (rollback) each have one.
//
// mu is the lock of the record it saves; every field is guarded by it.
// snapshot encodes the record with the lock held, and save writes an
// encoding without the lock and calls saved once it is stored.
type saver struct {
	mu       *sync.Mutex
	snapshot func() ([]byte, uint64, error)
	save     func(ctx context.Context, data []byte, v uint64) error

	// version counts the changes of the record; saved is the version of the
	// last save that worked.
	version, saved uint64
	saving         bool
	fails          int
	broken         bool
	closed         bool

	saves    sync.WaitGroup
	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
}

// changed notes a change of the record; the caller holds the lock.
func (s *saver) changed() { s.version++ }

// stored notes that the encoding of version v was saved; the caller holds
// the lock.
func (s *saver) stored(v uint64) { s.saved = max(s.saved, v) }

// start starts the periodic saves.
func (s *saver) start() {
	s.stop, s.stopped = make(chan struct{}), make(chan struct{})
	ticks, stopTicker := newTicker(autosaveInterval)
	go func() {
		defer close(s.stopped)
		defer stopTicker()
		for {
			select {
			case <-s.stop:
				return
			case <-ticks:
				s.tick()
			}
		}
	}()
}

// tick starts one periodic save when one is due.
func (s *saver) tick() {
	s.mu.Lock()
	if s.closed || s.saving || s.version == s.saved {
		s.mu.Unlock()
		return
	}
	data, v, err := s.snapshot()
	if err != nil {
		s.noteFailureLocked()
		s.mu.Unlock()
		return
	}
	s.saving = true
	s.saves.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.saves.Done()
		ok := false
		defer func() {
			// A panic of the save counts as a failed save (security
			// design §3: no goroutine may take the process down).
			_ = recover()
			s.mu.Lock()
			s.saving = false
			if ok {
				s.fails = 0
			} else {
				s.noteFailureLocked()
			}
			s.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
		defer cancel()
		ok = s.save(ctx, data, v) == nil
	}()
}

func (s *saver) noteFailureLocked() {
	s.fails++
	if s.fails >= failLimit {
		s.broken = true
	}
}

// isBroken reports whether the periodic saves failed failLimit times in a
// row.
func (s *saver) isBroken() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.broken
}

// halt ends the periodic saves and waits for one that runs.
func (s *saver) halt() {
	if s.stop != nil {
		s.stopOnce.Do(func() { close(s.stop) })
		<-s.stopped
	}
	s.saves.Wait()
}
