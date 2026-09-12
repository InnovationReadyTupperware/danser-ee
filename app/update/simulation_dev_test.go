//go:build !danser_release

package update

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSimulationUsesSharedCheckingStateWithoutPersisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), stateFileName)
	service, err := newService(path, "main-test", false, time.Now, nil)
	if err != nil {
		t.Fatalf("newService() error = %v", err)
	}

	started := make(chan struct{})
	done := make(chan Outcome, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		close(started)
		done <- service.Simulate(ctx, SimulatedResult{
			Status:        StatusUpdateAvailable,
			LatestVersion: "1.1.0",
			ReleaseURL:    "https://example.invalid/1.1.0",
		}, 10*time.Millisecond)
	}()
	<-started

	deadline := time.Now().Add(time.Second)
	for !service.Snapshot().Checking && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !service.Snapshot().Checking {
		t.Fatal("simulation never published Checking state")
	}

	outcome := <-done
	if outcome.Snapshot.Status != StatusUpdateAvailable || outcome.Snapshot.Checking {
		t.Fatalf("simulation snapshot = %#v", outcome.Snapshot)
	}
	_, found, err := loadSnapshot(path, "main-test")
	if err != nil {
		t.Fatalf("loadSnapshot() error = %v", err)
	} else if found {
		t.Fatal("simulation unexpectedly persisted update state")
	}

	service.ResetSimulation()
	if snapshot := service.Snapshot(); snapshot.Status != StatusDisabled || snapshot.Checking {
		t.Fatalf("reset snapshot = %#v, want disabled development state", snapshot)
	}
}
