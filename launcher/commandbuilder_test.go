package launcher

import (
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/wieku/rplpa"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
)

func TestReplayOverrideArgumentsRetainFileScoreRevision(t *testing.T) {
	oldMode, oldPMode := launcherConfig.CurrentMode, launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode, launcherConfig.CurrentPMode = oldMode, oldPMode
	})
	launcherConfig.CurrentMode, launcherConfig.CurrentPMode = Replay, Watch
	for _, version := range []int32{20250101, 30000016, 30000017} {
		replay := &rplpa.Replay{OsuVersion: version, Mods: uint32(difficulty.Hidden)}
		data, err := rplpa.WriteReplay(replay)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "replay with spaces.osr")
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		b := newBuilder()
		b.currentMap = &beatmap.BeatMap{MD5: "test-map"}
		b.replayPath = path
		b.setReplay(replay)
		b.diff.SetMods(difficulty.HardRock)
		args, err := b.getArgumentsChecked()
		if err != nil {
			t.Fatal(err)
		}
		// Read the file and override carried by the launcher's actual arguments
		// The child resolves score provenance from this file, not from mods JSON
		flags := flag.NewFlagSet("replay-child", flag.ContinueOnError)
		flags.Bool("nodbcheck", false, "")
		flags.Bool("noupdatecheck", false, "")
		flags.String("beatmap-path", "", "")
		replayPath := flags.String("replay", "", "")
		modsJSON := flags.String("mods2", "", "")
		if err = flags.Parse(args); err != nil {
			t.Fatal(err)
		}
		data, err = os.ReadFile(*replayPath)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := rplpa.ParseReplay(data)
		if err != nil {
			t.Fatal(err)
		}
		var override []rplpa.ModInfo
		if err = json.Unmarshal([]byte(*modsJSON), &override); err != nil {
			t.Fatal(err)
		}
		child := difficulty.NewDifficulty(5, 5, 5, 5)
		child.SetGameplayMode(difficulty.GameplayModeFromReplayVersion(int(decoded.OsuVersion)))
		child.SetModsFromReplay(decoded)
		child.SetMods2(override)
		if !child.IsLazer() {
			child.AddMod(difficulty.Classic)
		}
		want := 1.06
		switch version {
		case 20250101:
			want *= 0.96
		case 30000017:
			want = 1.09
		}
		if child.ScoreVersion != version || math.Abs(child.GetScoreMultiplier()-want) > 1e-12 {
			t.Fatalf("version %d child revision=%d multiplier=%g, want %g", version, child.ScoreVersion, child.GetScoreMultiplier(), want)
		}
	}
}

func TestKnockoutArgumentsIncludeSelectedReplays(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	oldPMode := launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
		launcherConfig.CurrentPMode = oldPMode
	})

	launcherConfig.CurrentMode = Knockout
	launcherConfig.CurrentPMode = Watch

	builder := newBuilder()
	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5"}
	builder.knockoutReplays = []*knockoutReplay{
		{path: "first.osr", included: true},
		{path: "excluded.osr", included: false},
	}

	args := builder.getArguments()
	for i, arg := range args {
		if arg == "-knockout2" {
			if i+1 >= len(args) {
				t.Fatal("-knockout2 is missing its replay-list argument")
			}
			if got := args[i+1]; got != `["first.osr"]` {
				t.Fatalf("selected knockout replay list = %q, want [\"first.osr\"]", got)
			}
			return
		}
	}

	t.Fatal("knockout arguments did not include -knockout2")
}

func TestSetReplayAddsClassicForStableReplay(t *testing.T) {
	builder := newBuilder()
	builder.setReplay(&rplpa.Replay{
		OsuVersion: difficulty.LazerReplayVersion - 1,
		Mods:       uint32(difficulty.Hidden),
	})

	difficulties := []struct {
		name string
		diff *difficulty.Difficulty
	}{
		{name: "source", diff: builder.sourceDiff},
		{name: "base", diff: builder.baseDiff},
		{name: "active", diff: builder.diff},
	}

	for _, test := range difficulties {
		t.Run(test.name, func(t *testing.T) {
			if got := test.diff.Mods; got != difficulty.Hidden|difficulty.Classic {
				t.Errorf("replay mods = %v, want HDCL", got)
			}
			if got := test.diff.GetGameplayMode(); got != difficulty.GameplayStable {
				t.Errorf("gameplay mode = %v, want Stable", got)
			}
		})
	}
}

func TestReplayScoreVersionResetsForGeneratedPlayback(t *testing.T) {
	for _, resetMap := range []bool{false, true} {
		b := newBuilder()
		b.setReplay(&rplpa.Replay{OsuVersion: 30000016})
		for _, diff := range []*difficulty.Difficulty{b.sourceDiff, b.baseDiff, b.diff} {
			if diff.ScoreVersion != 30000016 {
				t.Fatal("launcher difficulty lost replay score revision")
			}
		}
		if resetMap {
			b.setMap(&beatmap.BeatMap{Diff: difficulty.NewDifficulty(5, 5, 5, 5)})
		} else {
			b.removeReplay()
		}
		for _, diff := range []*difficulty.Difficulty{b.sourceDiff, b.baseDiff, b.diff} {
			if diff.ScoreVersion != 0 {
				t.Fatalf("reset map %t retained score revision %d", resetMap, diff.ScoreVersion)
			}
		}
	}
}

func TestSoloKnockoutArgumentsUseGeneratedParticipants(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	oldPMode := launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
		launcherConfig.CurrentPMode = oldPMode
	})

	launcherConfig.CurrentMode = SoloKnockout
	launcherConfig.CurrentPMode = Watch

	builder := newBuilder()
	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5"}
	builder.mirrors = 4
	builder.tags = 3

	args := builder.getArguments()
	if !containsArgument(args, "-solo-knockout") {
		t.Fatal("solo knockout arguments did not include -solo-knockout")
	}
	if !containsArgumentPair(args, "-cursors", "4") {
		t.Fatal("solo knockout arguments did not include the configured mirror count")
	}
	if !containsArgumentPair(args, "-tag", "3") {
		t.Fatal("solo knockout arguments did not include the configured participant count")
	}
	if containsArgument(args, "-knockout2") {
		t.Fatal("solo knockout arguments unexpectedly included replay selection")
	}
}

func TestKnockoutReplayCountIgnoresExcludedReplays(t *testing.T) {
	builder := newBuilder()
	builder.knockoutReplays = []*knockoutReplay{
		{included: true},
		{included: false},
		{included: true},
	}

	if got := builder.numKnockoutReplays(); got != 2 {
		t.Fatalf("included knockout replay count = %d, want 2", got)
	}
}

func TestReplayDrivenKnockoutRequiresReplaySelection(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
	})

	launcherConfig.CurrentMode = Knockout
	builder := newBuilder()
	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5"}

	if !builder.launchDisabled() {
		t.Fatal("replay-driven knockout was enabled without a replay")
	}

	builder.knockoutReplays = []*knockoutReplay{{included: true}}
	if builder.launchDisabled() {
		t.Fatal("replay-driven knockout remained disabled with a replay")
	}
}

func TestSoloKnockoutRequiresOnlyMapSelection(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
	})

	launcherConfig.CurrentMode = SoloKnockout
	builder := newBuilder()

	if !builder.launchDisabled() {
		t.Fatal("solo knockout was enabled without a map")
	}

	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5"}
	if builder.launchDisabled() {
		t.Fatal("solo knockout remained disabled without replay selection")
	}
}

func TestRecordingArgumentsRejectOutputPaths(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	oldPMode := launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
		launcherConfig.CurrentPMode = oldPMode
	})

	launcherConfig.CurrentMode = CursorDance
	launcherConfig.CurrentPMode = Record
	builder := newBuilder()
	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5"}
	builder.outputName = `..\outside`

	if _, err := builder.getArgumentsChecked(); err == nil {
		t.Fatal("recording arguments accepted an output path")
	}
}

func TestRecordingArgumentsRejectSurroundingWhitespace(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	oldPMode := launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
		launcherConfig.CurrentPMode = oldPMode
	})

	launcherConfig.CurrentMode = CursorDance
	launcherConfig.CurrentPMode = Record
	builder := newBuilder()
	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5"}
	builder.outputName = " recording"

	if _, err := builder.getArgumentsChecked(); err == nil {
		t.Fatal("recording arguments accepted surrounding whitespace")
	}
}

func TestBeatmapPathArgumentsUseSelectedMapLocation(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	oldPMode := launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
		launcherConfig.CurrentPMode = oldPMode
	})

	launcherConfig.CurrentMode = CursorDance
	launcherConfig.CurrentPMode = Watch

	builder := newBuilder()
	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5", Dir: "12345 Artist - Title", File: "artist - title [hard].osu"}

	args := builder.getArguments()
	if !containsArgumentPair(args, "-md5", "map-md5") {
		t.Fatal("map arguments did not include the map hash")
	}
	wantPath := filepath.Join("12345 Artist - Title", "artist - title [hard].osu")
	if !containsArgumentPair(args, "-beatmap-path", wantPath) {
		t.Fatalf("map arguments did not include the direct map path: %q", args)
	}
}

func TestBeatmapPathArgumentsCoverReplayMode(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	oldPMode := launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
		launcherConfig.CurrentPMode = oldPMode
	})

	launcherConfig.CurrentMode = Replay
	launcherConfig.CurrentPMode = Watch

	builder := newBuilder()
	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5", Dir: "set", File: "map.osu"}
	builder.replayPath = "replay.osr"
	builder.currentReplay = new(rplpa.Replay)

	args := builder.getArguments()
	if !containsArgumentPair(args, "-replay", "replay.osr") {
		t.Fatal("replay arguments did not include the replay path")
	}
	if !containsArgumentPair(args, "-beatmap-path", filepath.Join("set", "map.osu")) {
		t.Fatalf("replay arguments did not include the direct map path: %q", args)
	}
}

func TestHiddenCustomizationIsForwardedToGameplay(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	oldPMode := launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
		launcherConfig.CurrentPMode = oldPMode
	})

	launcherConfig.CurrentMode = CursorDance
	launcherConfig.CurrentPMode = Watch

	builder := newBuilder()
	builder.currentMap = &beatmap.BeatMap{MD5: "map-md5"}
	builder.diff.SetMods(difficulty.Hidden)

	hiddenSettings, ok := difficulty.GetModConfig[difficulty.HiddenSettings](builder.diff)
	if !ok {
		t.Fatal("Hidden settings were not initialized")
	}
	hiddenSettings.OnlyFadeApproachCircles = true
	difficulty.SetModConfig(builder.diff, hiddenSettings)

	args := builder.getArguments()
	var modsJSON string
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-mods2" {
			modsJSON = args[i+1]
			break
		}
	}
	if modsJSON == "" {
		t.Fatalf("launcher arguments did not include -mods2: %q", args)
	}

	var mods []rplpa.ModInfo
	if err := json.Unmarshal([]byte(modsJSON), &mods); err != nil {
		t.Fatalf("decode launcher mods: %v", err)
	}
	if len(mods) != 1 || mods[0].Acronym != "HD" {
		t.Fatalf("forwarded mods = %#v, want one HD mod", mods)
	}
	if got, ok := mods[0].Settings["only_fade_approach_circles"].(bool); !ok || !got {
		t.Fatalf("forwarded Hidden settings = %#v, want only_fade_approach_circles=true", mods[0].Settings)
	}
}

func TestBeatmapPathArgumentsRejectUnsafeLocations(t *testing.T) {
	oldMode := launcherConfig.CurrentMode
	oldPMode := launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode = oldMode
		launcherConfig.CurrentPMode = oldPMode
	})

	launcherConfig.CurrentMode = CursorDance
	launcherConfig.CurrentPMode = Watch

	unsafe := []beatmap.BeatMap{
		{MD5: "map-md5"},
		{MD5: "map-md5", Dir: "set", File: "../outside.osu"},
		{MD5: "map-md5", Dir: "../outside", File: "map.osu"},
		{MD5: "map-md5", Dir: "C:/songs", File: "map.osu"},
		{MD5: "map-md5", Dir: "set", File: "nested/map.osu"},
	}

	for i, currentMap := range unsafe {
		builder := newBuilder()
		builder.currentMap = &currentMap

		if args := builder.getArguments(); containsArgument(args, "-beatmap-path") {
			t.Fatalf("unsafe map %d produced a direct map path: %q", i, args)
		}
	}
}

func containsArgument(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}

	return false
}

func containsArgumentPair(args []string, name, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		arg := args[i]
		if arg == name && args[i+1] == value {
			return true
		}
	}

	return false
}

func TestLauncherSuddenDeathArgumentsPreserveTailSetting(t *testing.T) {
	oldMode, oldPMode := launcherConfig.CurrentMode, launcherConfig.CurrentPMode
	t.Cleanup(func() { launcherConfig.CurrentMode, launcherConfig.CurrentPMode = oldMode, oldPMode })
	launcherConfig.CurrentMode, launcherConfig.CurrentPMode = Play, Watch
	b := newBuilder()
	b.currentMap = &beatmap.BeatMap{MD5: "test-map"}
	b.diff.SetMods(difficulty.SuddenDeath)
	difficulty.SetModConfig(b.diff, difficulty.SuddenDeathSettings{FailOnSliderTail: true})
	args, err := b.getArgumentsChecked()
	if err != nil {
		t.Fatal(err)
	}
	var mods []rplpa.ModInfo
	for i, arg := range args {
		if arg == "-mods2" {
			if err := json.Unmarshal([]byte(args[i+1]), &mods); err != nil {
				t.Fatal(err)
			}
		}
	}
	child := difficulty.NewDifficulty(5, 5, 5, 5)
	child.SetMods2(mods)
	conf, ok := difficulty.GetModConfig[difficulty.SuddenDeathSettings](child)
	if !ok || !conf.FailOnSliderTail {
		t.Fatalf("child Sudden Death settings = %#v, %t", conf, ok)
	}
}

func TestLauncherRejectsConflictsIntroducedByAutoplay(t *testing.T) {
	oldMode, oldPMode := launcherConfig.CurrentMode, launcherConfig.CurrentPMode
	t.Cleanup(func() { launcherConfig.CurrentMode, launcherConfig.CurrentPMode = oldMode, oldPMode })
	launcherConfig.CurrentMode, launcherConfig.CurrentPMode = DanserReplay, Watch
	b := newBuilder()
	b.currentMap = &beatmap.BeatMap{MD5: "test-map"}
	b.diff.SetMods(difficulty.Relax)
	if _, err := b.getArgumentsChecked(); err == nil {
		t.Fatal("launcher accepted Relax with its generated Autoplay mod")
	}
}
