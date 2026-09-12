package update

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	appUtils "github.com/innovationreadytupperware/danser-ee/app/utils"
	"github.com/innovationreadytupperware/danser-ee/build"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
)

const (
	stateSchemaVersion = 2
	stateFileName      = "update-state.json"

	successFreshness    = 24 * time.Hour
	noReleasesFreshness = 6 * time.Hour
	failureFreshness    = time.Hour
)

// Status describes the most recently completed update check. Checking is kept
// separately in Snapshot so the previous result remains available while a new
// request is in flight.
type Status string

const (
	StatusUnknown         Status = "unknown"
	StatusDisabled        Status = "disabled"
	StatusUpToDate        Status = "up_to_date"
	StatusUpdateAvailable Status = "update_available"
	StatusNoReleases      Status = "no_releases"
	StatusFailed          Status = "failed"
)

// CheckMode controls whether a cached result may satisfy the request.
type CheckMode uint8

const (
	Automatic CheckMode = iota
	Manual
)

// Snapshot is the process-wide view of update state. It is safe to copy and
// may be read by UI code without retaining any Service locks.
type Snapshot struct {
	Status              Status
	CurrentVersion      string
	LatestVersion       string
	ReleaseURL          string
	IncludePrerelease   bool
	LastAttempt         time.Time
	LastSuccessfulCheck time.Time
	Error               string
	Checking            bool
}

// Outcome reports what a Check call observed. Performed is false when a fresh
// cached result, a development build, or an already-running check satisfied the
// request without another network request.
type Outcome struct {
	Snapshot  Snapshot
	Performed bool
	Err       error
}

type persistedState struct {
	SchemaVersion       int       `json:"schema_version"`
	CheckedVersion      string    `json:"checked_version"`
	Status              Status    `json:"status"`
	LatestVersion       string    `json:"latest_version,omitempty"`
	ReleaseURL          string    `json:"release_url,omitempty"`
	IncludePrerelease   bool      `json:"include_prerelease"`
	LastAttempt         time.Time `json:"last_attempt"`
	LastSuccessfulCheck time.Time `json:"last_successful_check,omitzero"`
	Error               string    `json:"error,omitempty"`
}

type checkerFunc func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error)

// Service owns update freshness policy, persisted state, and in-process
// coordination. Callers decide whether automatic checking is enabled for their
// surface; the service decides whether a network request is actually needed.
type Service struct {
	mu sync.RWMutex

	path              string
	currentVersion    string
	eligible          bool
	automaticAllowed  bool
	includePrerelease bool
	checker           checkerFunc
	now               func() time.Time

	snapshot   Snapshot
	generation uint64
}

var defaultService = sync.OnceValue(func() *Service {
	service, err := newService(
		filepath.Join(env.DataDir(), stateFileName),
		build.Version,
		build.IsRelease(),
		time.Now,
		appUtils.CheckForUpdateDetailsWithOptionsContext,
	)
	if err != nil {
		log.Printf("Update: couldn't load cached state: %v", err)
	}
	service.automaticAllowed = !strings.HasPrefix(filepath.ToSlash(env.LibDir()), "/usr/lib/")
	return service
})

// Default returns the shared update service for this process.
func Default() *Service {
	return defaultService()
}

func newService(
	path, currentVersion string,
	eligible bool,
	now func() time.Time,
	checker checkerFunc,
) (*Service, error) {
	if now == nil {
		now = time.Now
	}
	if checker == nil {
		checker = appUtils.CheckForUpdateDetailsWithOptionsContext
	}

	service := &Service{
		path:             path,
		currentVersion:   currentVersion,
		eligible:         eligible,
		automaticAllowed: true,
		checker:          checker,
		now:              now,
		snapshot:         initialSnapshot(currentVersion, eligible),
	}

	if !eligible {
		return service, nil
	}

	snapshot, found, err := loadSnapshot(path, currentVersion)
	if err != nil {
		return service, err
	}
	if found {
		service.snapshot = snapshot
	}

	return service, nil
}

// SetIncludePrereleaseUpdates configures whether update checks may advance to
// pre-release versions outside the current release line. Packaged pre-release
// builds can still advance within their current base version when this is off.
func (s *Service) SetIncludePrereleaseUpdates(include bool) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.includePrerelease = include
	s.mu.Unlock()
}

func initialSnapshot(currentVersion string, eligible bool) Snapshot {
	status := StatusUnknown
	if !eligible {
		status = StatusDisabled
	}
	return Snapshot{
		Status:         status,
		CurrentVersion: currentVersion,
	}
}

// Snapshot returns the latest update state.
func (s *Service) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// Check performs or reuses an update check according to mode. Automatic checks
// respect the shared freshness policy; manual checks always bypass it.
func (s *Service) Check(ctx context.Context, mode CheckMode) Outcome {
	if s == nil {
		return Outcome{Err: errors.New("nil update service")}
	}
	if ctx == nil {
		return Outcome{Snapshot: s.Snapshot(), Err: errors.New("nil context")}
	}

	s.mu.Lock()
	if !s.eligible {
		s.snapshot = initialSnapshot(s.currentVersion, false)
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot}
	}
	if mode == Automatic && !s.automaticAllowed {
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot}
	}
	if s.snapshot.Checking {
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot}
	}
	includePrerelease := s.includePrerelease
	if mode == Automatic && s.snapshot.IncludePrerelease == includePrerelease && snapshotFreshAt(s.snapshot, s.now()) {
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot}
	}

	s.generation++
	requestID := s.generation
	s.snapshot.Checking = true
	s.mu.Unlock()

	result, checkErr := s.checker(ctx, appUtils.UpdateCheckOptions{IncludePrerelease: includePrerelease})
	if ctx.Err() != nil {
		s.mu.Lock()
		if requestID == s.generation {
			s.snapshot.Checking = false
		}
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot, Performed: true, Err: ctx.Err()}
	}

	completedAt := s.now()
	status := statusFromResult(result.Status, checkErr)

	s.mu.Lock()
	if requestID != s.generation {
		snapshot := s.snapshot
		s.mu.Unlock()
		return Outcome{Snapshot: snapshot, Performed: true, Err: checkErr}
	}

	lastSuccessful := s.snapshot.LastSuccessfulCheck
	if status == StatusUpToDate || status == StatusUpdateAvailable {
		lastSuccessful = completedAt
	}

	errorText := ""
	if checkErr != nil {
		errorText = checkErr.Error()
	}
	s.snapshot = Snapshot{
		Status:              status,
		CurrentVersion:      s.currentVersion,
		LatestVersion:       result.LatestVersion,
		ReleaseURL:          result.ReleaseURL,
		IncludePrerelease:   includePrerelease,
		LastAttempt:         completedAt,
		LastSuccessfulCheck: lastSuccessful,
		Error:               errorText,
	}
	snapshot := s.snapshot
	state := persistedStateFromSnapshot(snapshot)
	s.mu.Unlock()

	if status != StatusDisabled {
		if err := savePersistedState(s.path, state); err != nil {
			log.Printf("Update: couldn't save cached state: %v", err)
		}
	}

	return Outcome{
		Snapshot:  snapshot,
		Performed: true,
		Err:       checkErr,
	}
}

func statusFromResult(status appUtils.UpdateStatus, err error) Status {
	if errors.Is(err, appUtils.ErrNoReleases) {
		return StatusNoReleases
	}
	if err != nil || status == appUtils.Failed {
		return StatusFailed
	}

	switch status {
	case appUtils.Ignored:
		return StatusDisabled
	case appUtils.UpToDate:
		return StatusUpToDate
	case appUtils.UpdateAvailable:
		return StatusUpdateAvailable
	default:
		return StatusFailed
	}
}

func snapshotFreshAt(snapshot Snapshot, now time.Time) bool {
	if snapshot.LastAttempt.IsZero() || now.Before(snapshot.LastAttempt) {
		return false
	}

	var freshness time.Duration
	switch snapshot.Status {
	case StatusUpToDate, StatusUpdateAvailable:
		freshness = successFreshness
	case StatusNoReleases:
		freshness = noReleasesFreshness
	case StatusFailed:
		freshness = failureFreshness
	default:
		return false
	}

	return now.Sub(snapshot.LastAttempt) < freshness
}

func persistedStateFromSnapshot(snapshot Snapshot) persistedState {
	return persistedState{
		SchemaVersion:       stateSchemaVersion,
		CheckedVersion:      snapshot.CurrentVersion,
		Status:              snapshot.Status,
		LatestVersion:       snapshot.LatestVersion,
		ReleaseURL:          snapshot.ReleaseURL,
		IncludePrerelease:   snapshot.IncludePrerelease,
		LastAttempt:         snapshot.LastAttempt,
		LastSuccessfulCheck: snapshot.LastSuccessfulCheck,
		Error:               snapshot.Error,
	}
}

func loadSnapshot(path, currentVersion string) (Snapshot, bool, error) {
	if path == "" {
		return Snapshot{}, false, nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("read %s: %w", path, err)
	}

	var state persistedState
	if err = json.Unmarshal(data, &state); err != nil {
		return Snapshot{}, false, fmt.Errorf("decode %s: %w", path, err)
	}
	if state.SchemaVersion != stateSchemaVersion || state.CheckedVersion != currentVersion {
		return Snapshot{}, false, nil
	}
	if !persistableStatus(state.Status) {
		return Snapshot{}, false, fmt.Errorf("decode %s: invalid status %q", path, state.Status)
	}

	return Snapshot{
		Status:              state.Status,
		CurrentVersion:      currentVersion,
		LatestVersion:       state.LatestVersion,
		ReleaseURL:          state.ReleaseURL,
		IncludePrerelease:   state.IncludePrerelease,
		LastAttempt:         state.LastAttempt,
		LastSuccessfulCheck: state.LastSuccessfulCheck,
		Error:               state.Error,
	}, true, nil
}

func persistableStatus(status Status) bool {
	switch status {
	case StatusUpToDate, StatusUpdateAvailable, StatusNoReleases, StatusFailed:
		return true
	default:
		return false
	}
}

func savePersistedState(path string, state persistedState) error {
	if path == "" {
		return nil
	}

	data, err := json.Marshal(&state)
	if err != nil {
		return fmt.Errorf("encode update state: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create update state directory: %w", err)
	}
	if err = os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (s *Service) resetFromStore() {
	if s == nil {
		return
	}

	snapshot := initialSnapshot(s.currentVersion, s.eligible)
	if s.eligible {
		loaded, found, err := loadSnapshot(s.path, s.currentVersion)
		if err != nil {
			log.Printf("Update: couldn't reload cached state: %v", err)
		} else if found {
			snapshot = loaded
		}
	}

	s.mu.Lock()
	s.generation++
	s.snapshot = snapshot
	s.mu.Unlock()
}
