//go:build !danser_release

package update

import (
	"context"
	"errors"
	"time"
)

// SimulatedResult is a developer-only update outcome used by the launcher to
// exercise the real shared update presentation without performing network I/O.
type SimulatedResult struct {
	Status        Status
	LatestVersion string
	ReleaseURL    string
	Error         string
}

// Simulate transitions the shared service through its normal checking state,
// waits for delay, then publishes result. Simulated results are never written
// to disk.
func (s *Service) Simulate(ctx context.Context, result SimulatedResult, delay time.Duration) Outcome {
	if s == nil {
		return Outcome{}
	}
	if ctx == nil {
		return Outcome{Snapshot: s.Snapshot(), Err: errors.New("nil context")}
	}

	s.mu.Lock()
	if s.snapshot.Checking {
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot}
	}
	s.generation++
	requestID := s.generation
	s.snapshot.Checking = true
	s.mu.Unlock()

	timer := time.NewTimer(max(delay, time.Duration(0)))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		s.mu.Lock()
		if requestID == s.generation {
			s.snapshot.Checking = false
		}
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot, Performed: true, Err: ctx.Err()}
	}

	completedAt := s.now()
	s.mu.Lock()
	if requestID != s.generation {
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot, Performed: true}
	}

	lastSuccessful := s.snapshot.LastSuccessfulCheck
	if result.Status == StatusUpToDate || result.Status == StatusUpdateAvailable {
		lastSuccessful = completedAt
	}
	s.snapshot = Snapshot{
		Status:              result.Status,
		CurrentVersion:      s.currentVersion,
		LatestVersion:       result.LatestVersion,
		ReleaseURL:          result.ReleaseURL,
		LastAttempt:         completedAt,
		LastSuccessfulCheck: lastSuccessful,
		Error:               result.Error,
	}
	snapshot := s.snapshot
	s.mu.Unlock()

	return Outcome{Snapshot: snapshot, Performed: true}
}

// ResetSimulation restores the real persisted/default snapshot and invalidates
// any developer simulation that is still waiting to complete.
func (s *Service) ResetSimulation() {
	s.resetFromStore()
}
