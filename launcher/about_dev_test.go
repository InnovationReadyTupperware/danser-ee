//go:build !danser_release

package launcher

import (
	"testing"
	"time"

	appUpdate "github.com/innovationreadytupperware/danser-ee/app/update"
)

func TestAboutUpdateSimulationDelay(t *testing.T) {
	if aboutUpdateSimulationDelay != 2*time.Second {
		t.Fatalf("aboutUpdateSimulationDelay = %v, want 2s", aboutUpdateSimulationDelay)
	}
}

func TestAboutUpdateSimulationsCoverVisibleResults(t *testing.T) {
	want := map[appUpdate.Status]bool{
		appUpdate.StatusUpToDate:        false,
		appUpdate.StatusUpdateAvailable: false,
		appUpdate.StatusNoReleases:      false,
		appUpdate.StatusFailed:          false,
	}

	for _, simulation := range aboutUpdateSimulations {
		if _, ok := want[simulation.result.Status]; ok {
			want[simulation.result.Status] = true
		}
	}

	for status, covered := range want {
		if !covered {
			t.Errorf("developer simulations do not cover update status %v", status)
		}
	}

	if len(aboutUpdateSimulations) == 0 || aboutUpdateSimulations[0].result.Status != appUpdate.StatusUpToDate {
		t.Fatal("first developer simulation should be the normal up-to-date result")
	}
}

func TestAboutReleaseIdentitySimulation(t *testing.T) {
	original := aboutSimulateReleaseIdentity
	t.Cleanup(func() { aboutSimulateReleaseIdentity = original })

	aboutSimulateReleaseIdentity = true
	version, releaseStyle := aboutDisplayedBuildIdentity()
	if version != "2.3.1" || !releaseStyle {
		t.Fatalf("aboutDisplayedBuildIdentity() = %q, %t; want 2.3.1, true", version, releaseStyle)
	}
}

func TestAboutAutomaticChecksDisabledSimulation(t *testing.T) {
	original := aboutSimulateAutomaticChecksDisabled
	originalSetting := launcherConfig.CheckForUpdates
	t.Cleanup(func() {
		aboutSimulateAutomaticChecksDisabled = original
		launcherConfig.CheckForUpdates = originalSetting
	})

	launcherConfig.CheckForUpdates = true
	aboutSimulateAutomaticChecksDisabled = true
	if aboutAutomaticUpdateChecksEnabled() {
		t.Fatal("aboutAutomaticUpdateChecksEnabled() = true while disabled simulation is active")
	}

	aboutSimulateAutomaticChecksDisabled = false
	if !aboutAutomaticUpdateChecksEnabled() {
		t.Fatal("aboutAutomaticUpdateChecksEnabled() = false with enabled launcher setting")
	}
}
