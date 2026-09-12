package update

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	appUtils "github.com/innovationreadytupperware/danser-ee/app/utils"
)

func TestServiceAutomaticFreshness(t *testing.T) {
	base := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		status    Status
		age       time.Duration
		wantCalls int32
	}{
		{name: "success fresh", status: StatusUpToDate, age: 23 * time.Hour},
		{name: "success stale", status: StatusUpToDate, age: 24 * time.Hour, wantCalls: 1},
		{name: "available fresh", status: StatusUpdateAvailable, age: 12 * time.Hour},
		{name: "no releases fresh", status: StatusNoReleases, age: 5 * time.Hour},
		{name: "no releases stale", status: StatusNoReleases, age: 6 * time.Hour, wantCalls: 1},
		{name: "failure fresh", status: StatusFailed, age: 30 * time.Minute},
		{name: "failure stale", status: StatusFailed, age: time.Hour, wantCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), stateFileName)
			if err := savePersistedState(path, persistedState{
				SchemaVersion:  stateSchemaVersion,
				CheckedVersion: "1.0.0",
				Status:         test.status,
				LastAttempt:    base.Add(-test.age),
			}); err != nil {
				t.Fatalf("savePersistedState() error = %v", err)
			}

			var calls atomic.Int32
			service, err := newService(path, "1.0.0", true, func() time.Time { return base }, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
				calls.Add(1)
				return appUtils.UpdateResult{Status: appUtils.UpToDate, LatestVersion: "1.0.0"}, nil
			})
			if err != nil {
				t.Fatalf("newService() error = %v", err)
			}

			outcome := service.Check(t.Context(), Automatic)
			if got := calls.Load(); got != test.wantCalls {
				t.Fatalf("checker calls = %d, want %d", got, test.wantCalls)
			}
			if outcome.Performed != (test.wantCalls > 0) {
				t.Fatalf("outcome.Performed = %t", outcome.Performed)
			}
		})
	}
}

func TestServicePrereleasePolicyInvalidatesFreshCache(t *testing.T) {
	now := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), stateFileName)
	if err := savePersistedState(path, persistedState{
		SchemaVersion:     stateSchemaVersion,
		CheckedVersion:    "1.2.0",
		Status:            StatusUpToDate,
		IncludePrerelease: false,
		LastAttempt:       now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("savePersistedState() error = %v", err)
	}

	var calls atomic.Int32
	var gotOptions appUtils.UpdateCheckOptions
	service, err := newService(path, "1.2.0", true, func() time.Time { return now }, func(_ context.Context, options appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		calls.Add(1)
		gotOptions = options
		return appUtils.UpdateResult{Status: appUtils.UpToDate, LatestVersion: "1.3.0-rc.1"}, nil
	})
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}
	service.SetIncludePrereleaseUpdates(true)

	outcome := service.Check(t.Context(), Automatic)
	if !outcome.Performed || calls.Load() != 1 {
		t.Fatalf("automatic check performed=%t calls=%d, want a new check", outcome.Performed, calls.Load())
	}
	if !gotOptions.IncludePrerelease {
		t.Fatal("checker did not receive IncludePrerelease=true")
	}
	if !outcome.Snapshot.IncludePrerelease {
		t.Fatal("snapshot did not record IncludePrerelease=true")
	}
}

func TestServiceDoesNotRestorePrereleasePreferenceFromCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), stateFileName)
	if err := savePersistedState(path, persistedState{
		SchemaVersion:     stateSchemaVersion,
		CheckedVersion:    "1.2.0",
		Status:            StatusUpToDate,
		IncludePrerelease: true,
		LastAttempt:       time.Now(),
	}); err != nil {
		t.Fatalf("savePersistedState() error = %v", err)
	}

	var gotOptions appUtils.UpdateCheckOptions
	service, err := newService(path, "1.2.0", true, time.Now, func(_ context.Context, options appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		gotOptions = options
		return appUtils.UpdateResult{Status: appUtils.UpToDate, LatestVersion: "1.2.0"}, nil
	})
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}

	outcome := service.Check(t.Context(), Automatic)
	if !outcome.Performed {
		t.Fatal("automatic check reused cache from a different prerelease policy")
	}
	if gotOptions.IncludePrerelease {
		t.Fatal("new service restored IncludePrerelease=true from cached state")
	}
}

func TestServiceManualCheckBypassesFreshCache(t *testing.T) {
	now := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), stateFileName)
	if err := savePersistedState(path, persistedState{
		SchemaVersion:  stateSchemaVersion,
		CheckedVersion: "1.0.0",
		Status:         StatusUpToDate,
		LastAttempt:    now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("savePersistedState() error = %v", err)
	}

	var calls atomic.Int32
	service, err := newService(path, "1.0.0", true, func() time.Time { return now }, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		calls.Add(1)
		return appUtils.UpdateResult{
			Status:        appUtils.UpdateAvailable,
			LatestVersion: "1.1.0",
			ReleaseURL:    "https://example.invalid/1.1.0",
		}, nil
	})
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}

	outcome := service.Check(t.Context(), Manual)
	if calls.Load() != 1 || !outcome.Performed {
		t.Fatalf("manual check calls=%d performed=%t, want one performed check", calls.Load(), outcome.Performed)
	}
	if outcome.Snapshot.Status != StatusUpdateAvailable || outcome.Snapshot.LatestVersion != "1.1.0" {
		t.Fatalf("manual snapshot = %#v", outcome.Snapshot)
	}
}

func TestServiceCompletedCheckIsSharedThroughCache(t *testing.T) {
	checkedAt := time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), stateFileName)

	first, err := newService(path, "1.0.0", true, func() time.Time { return checkedAt }, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		return appUtils.UpdateResult{
			Status:        appUtils.UpdateAvailable,
			LatestVersion: "1.1.0",
			ReleaseURL:    "https://example.invalid/1.1.0",
		}, nil
	})
	if err != nil {
		t.Fatalf("newService(first) error = %v", err)
	}
	if outcome := first.Check(t.Context(), Manual); outcome.Snapshot.Status != StatusUpdateAvailable {
		t.Fatalf("first check snapshot = %#v", outcome.Snapshot)
	}

	var secondCalls atomic.Int32
	second, err := newService(path, "1.0.0", true, func() time.Time { return checkedAt.Add(time.Hour) }, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		secondCalls.Add(1)
		return appUtils.UpdateResult{Status: appUtils.UpToDate}, nil
	})
	if err != nil {
		t.Fatalf("newService(second) error = %v", err)
	}
	loaded := second.Snapshot()
	if loaded.Status != StatusUpdateAvailable || loaded.LatestVersion != "1.1.0" || loaded.ReleaseURL != "https://example.invalid/1.1.0" {
		t.Fatalf("loaded snapshot = %#v", loaded)
	}
	if !loaded.LastAttempt.Equal(checkedAt) || !loaded.LastSuccessfulCheck.Equal(checkedAt) {
		t.Fatalf("loaded timestamps = attempt %v success %v", loaded.LastAttempt, loaded.LastSuccessfulCheck)
	}

	outcome := second.Check(t.Context(), Automatic)
	if outcome.Performed || secondCalls.Load() != 0 {
		t.Fatalf("fresh shared cache performed=%t checker calls=%d", outcome.Performed, secondCalls.Load())
	}
}

func TestServicePublishesCheckingStateAndDeduplicates(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	service, err := newService("", "1.0.0", true, time.Now, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		calls.Add(1)
		close(started)
		<-release
		return appUtils.UpdateResult{Status: appUtils.UpToDate, LatestVersion: "1.0.0"}, nil
	})
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}

	done := make(chan Outcome, 1)
	go func() {
		done <- service.Check(t.Context(), Manual)
	}()
	<-started

	if snapshot := service.Snapshot(); !snapshot.Checking {
		t.Fatal("Snapshot().Checking = false while checker is blocked")
	}
	second := service.Check(t.Context(), Manual)
	if second.Performed {
		t.Fatal("second concurrent Check() performed another request")
	}
	if calls.Load() != 1 {
		t.Fatalf("checker calls = %d, want 1", calls.Load())
	}

	close(release)
	if outcome := <-done; outcome.Snapshot.Checking || outcome.Snapshot.Status != StatusUpToDate {
		t.Fatalf("completed snapshot = %#v", outcome.Snapshot)
	}
}

func TestServiceFailurePreservesLastSuccessfulCheck(t *testing.T) {
	lastSuccess := time.Date(2026, time.September, 10, 8, 0, 0, 0, time.UTC)
	now := lastSuccess.Add(48 * time.Hour)
	path := filepath.Join(t.TempDir(), stateFileName)
	if err := savePersistedState(path, persistedState{
		SchemaVersion:       stateSchemaVersion,
		CheckedVersion:      "1.0.0",
		Status:              StatusUpToDate,
		LastAttempt:         lastSuccess,
		LastSuccessfulCheck: lastSuccess,
	}); err != nil {
		t.Fatalf("savePersistedState() error = %v", err)
	}

	wantErr := errors.New("network unavailable")
	service, err := newService(path, "1.0.0", true, func() time.Time { return now }, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		return appUtils.UpdateResult{Status: appUtils.Failed}, wantErr
	})
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}

	outcome := service.Check(t.Context(), Manual)
	if !errors.Is(outcome.Err, wantErr) {
		t.Fatalf("outcome.Err = %v, want %v", outcome.Err, wantErr)
	}
	if outcome.Snapshot.Status != StatusFailed {
		t.Fatalf("status = %v, want failed", outcome.Snapshot.Status)
	}
	if !outcome.Snapshot.LastSuccessfulCheck.Equal(lastSuccess) {
		t.Fatalf("LastSuccessfulCheck = %v, want %v", outcome.Snapshot.LastSuccessfulCheck, lastSuccess)
	}
	if !outcome.Snapshot.LastAttempt.Equal(now) {
		t.Fatalf("LastAttempt = %v, want %v", outcome.Snapshot.LastAttempt, now)
	}
}

func TestServiceNoReleasesHasDistinctStatus(t *testing.T) {
	service, err := newService("", "1.0.0", true, time.Now, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		return appUtils.UpdateResult{Status: appUtils.Failed}, appUtils.ErrNoReleases
	})
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}

	outcome := service.Check(t.Context(), Manual)
	if outcome.Snapshot.Status != StatusNoReleases {
		t.Fatalf("status = %v, want %v", outcome.Snapshot.Status, StatusNoReleases)
	}
}

func TestServiceCacheIsScopedToCurrentVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), stateFileName)
	if err := savePersistedState(path, persistedState{
		SchemaVersion:  stateSchemaVersion,
		CheckedVersion: "1.0.0",
		Status:         StatusUpdateAvailable,
		LatestVersion:  "2.0.0",
		LastAttempt:    time.Now(),
	}); err != nil {
		t.Fatalf("savePersistedState() error = %v", err)
	}

	service, err := newService(path, "2.0.0", true, time.Now, nil)
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}
	if snapshot := service.Snapshot(); snapshot.Status != StatusUnknown || !snapshot.LastAttempt.IsZero() {
		t.Fatalf("snapshot = %#v, want empty state for new current version", snapshot)
	}
}

func TestDevelopmentServiceNeverCallsChecker(t *testing.T) {
	var calls atomic.Int32
	service, err := newService("", "main-test", false, time.Now, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		calls.Add(1)
		return appUtils.UpdateResult{}, nil
	})
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}

	outcome := service.Check(t.Context(), Manual)
	if calls.Load() != 0 {
		t.Fatalf("checker calls = %d, want 0", calls.Load())
	}
	if outcome.Snapshot.Status != StatusDisabled {
		t.Fatalf("status = %v, want disabled", outcome.Snapshot.Status)
	}
}

func TestAutomaticPolicyCanSuppressPackageManagedChecks(t *testing.T) {
	var calls atomic.Int32
	service, err := newService("", "1.0.0", true, time.Now, func(context.Context, appUtils.UpdateCheckOptions) (appUtils.UpdateResult, error) {
		calls.Add(1)
		return appUtils.UpdateResult{Status: appUtils.UpToDate}, nil
	})
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}
	service.automaticAllowed = false

	if outcome := service.Check(t.Context(), Automatic); outcome.Performed {
		t.Fatal("automatic package-managed check unexpectedly performed network work")
	}
	if calls.Load() != 0 {
		t.Fatalf("automatic checker calls = %d, want 0", calls.Load())
	}

	if outcome := service.Check(t.Context(), Manual); !outcome.Performed {
		t.Fatal("manual package-managed check should still be allowed")
	}
	if calls.Load() != 1 {
		t.Fatalf("manual checker calls = %d, want 1", calls.Load())
	}
}

func TestPersistedStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), stateFileName)
	want := persistedState{
		SchemaVersion:       stateSchemaVersion,
		CheckedVersion:      "1.2.3",
		Status:              StatusUpdateAvailable,
		LatestVersion:       "1.3.0",
		ReleaseURL:          "https://example.invalid/1.3.0",
		IncludePrerelease:   true,
		LastAttempt:         time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC),
		LastSuccessfulCheck: time.Date(2026, time.September, 12, 8, 0, 0, 0, time.UTC),
	}
	if err := savePersistedState(path, want); err != nil {
		t.Fatalf("savePersistedState() error = %v", err)
	}

	got, found, err := loadSnapshot(path, want.CheckedVersion)
	if err != nil {
		t.Fatalf("loadSnapshot() error = %v", err)
	}
	if !found {
		t.Fatal("loadSnapshot() found = false")
	}
	if got.Status != want.Status || got.LatestVersion != want.LatestVersion || got.ReleaseURL != want.ReleaseURL || got.IncludePrerelease != want.IncludePrerelease ||
		!got.LastAttempt.Equal(want.LastAttempt) || !got.LastSuccessfulCheck.Equal(want.LastSuccessfulCheck) {
		t.Fatalf("round-trip snapshot = %#v", got)
	}
}
