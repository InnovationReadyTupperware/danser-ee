package launcher

import (
	"testing"

	"github.com/wieku/danser-go/app/beatmap"
)

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
