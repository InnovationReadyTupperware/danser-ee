package pp260706

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func loadRegressionBeatmap(t *testing.T, mods difficulty.Modifier, mode difficulty.GameplayMode) *beatmap.BeatMap {
	t.Helper()

	fixtureDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("resolve pp fixture directory: %v", err)
	}

	// This helper mutates process-global settings, so tests using it must not run
	// in parallel.
	previousSongsDir := settings.General.OsuSongsDir
	settings.General.OsuSongsDir = fixtureDir
	t.Cleanup(func() {
		settings.General.OsuSongsDir = previousSongsDir
	})

	file, err := os.Open(filepath.Join(fixtureDir, "representative.osu"))
	if err != nil {
		t.Fatalf("open pp fixture: %v", err)
	}
	t.Cleanup(func() {
		_ = file.Close()
	})

	beatMap, err := beatmap.ParseBeatMapFileWithError(file)
	if err != nil {
		t.Fatalf("parse pp fixture metadata: %v", err)
	}

	beatMap.Diff.SetGameplayMode(mode)
	beatMap.Diff.SetMods(mods)
	beatmap.ParseObjects(beatMap, true, false)

	return beatMap
}
