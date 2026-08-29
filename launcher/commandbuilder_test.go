package launcher

import (
	"testing"

	"github.com/wieku/danser-go/app/beatmap"
)

func TestKnockoutArgumentsPreserveEmptyReplaySelection(t *testing.T) {
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

	args := builder.getArguments()
	for i, arg := range args {
		if arg == "-knockout2" {
			if i+1 >= len(args) {
				t.Fatal("-knockout2 is missing its replay-list argument")
			}
			if got := args[i+1]; got != "[]" {
				t.Fatalf("empty knockout replay list = %q, want []", got)
			}
			return
		}
	}

	t.Fatal("knockout arguments did not include -knockout2")
}
