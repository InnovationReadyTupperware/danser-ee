package states

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

type generatedCursorPolicyTestController struct {
	cursor *graphics.Cursor
}

func (controller generatedCursorPolicyTestController) IsGeneratedCursor(cursor *graphics.Cursor) bool {
	return cursor == controller.cursor
}

func TestCursorDanceFailurePoliciesKeepVisualPlaybackAlive(t *testing.T) {
	generated := &graphics.Cursor{}
	replay := &graphics.Cursor{IsReplay: true}
	controller := generatedCursorPolicyTestController{cursor: generated}

	if got := scoreFailurePolicy(true, generated); got != osu.FailurePolicySuppress {
		t.Fatalf("standalone cursor-dance failure policy = %v, want suppress", got)
	}
	if got := scoreFailurePolicy(false, replay); got != osu.FailurePolicySuppress {
		t.Fatalf("replay failure policy = %v, want suppress", got)
	}
	if got := knockoutFailurePolicy(true, controller, generated); got != osu.FailurePolicySuppress {
		t.Fatalf("standalone AT knockout policy = %v, want suppress", got)
	}
	if got := knockoutFailurePolicy(false, controller, generated); got != osu.FailurePolicyContinue {
		t.Fatalf("explicit generated knockout policy = %v, want continue", got)
	}
	if got := scoreFailurePolicy(false, &graphics.Cursor{}); got != osu.FailurePolicyAllow {
		t.Fatalf("playback failure policy = %v, want allow", got)
	}
}

func TestShouldTrimBeatmapForPlaybackIncludesStandaloneAutomatedPlayback(t *testing.T) {
	previousPlay := settings.PLAY
	previousKnockout := settings.KNOCKOUT
	previousSoloKnockout := settings.SOLOKNOCKOUT
	t.Cleanup(func() {
		settings.PLAY = previousPlay
		settings.KNOCKOUT = previousKnockout
		settings.SOLOKNOCKOUT = previousSoloKnockout
	})

	settings.PLAY = false
	settings.KNOCKOUT = true
	settings.SOLOKNOCKOUT = false

	tests := []struct {
		name              string
		play              bool
		knockout          bool
		soloKnockout      bool
		automatedPlayback bool
		want              bool
	}{
		{name: "replay-backed knockout", knockout: true, want: false},
		{name: "standalone automated playback", knockout: true, automatedPlayback: true, want: true},
		{name: "normal play", play: true, want: true},
		{name: "generic cursor dance", want: true},
		{name: "solo knockout", knockout: true, soloKnockout: true, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings.PLAY = test.play
			settings.KNOCKOUT = test.knockout
			settings.SOLOKNOCKOUT = test.soloKnockout

			if got := shouldTrimBeatmapForPlayback(test.automatedPlayback); got != test.want {
				t.Fatalf("shouldTrimBeatmapForPlayback(%t) = %t, want %t", test.automatedPlayback, got, test.want)
			}
		})
	}
}

func TestTrimBeatmapToWindowRemovesObjectsBeforeSeekWithoutChangingFutureIDs(t *testing.T) {
	beatMap := beatmap.NewBeatMap()
	oldObject := objects.DummyCircle(vector.NewVec2f(100, 100), 1000)
	retainedObject := objects.DummyCircle(vector.NewVec2f(200, 200), 3000)
	oldObject.SetID(20)
	retainedObject.SetID(21)
	beatMap.HitObjects = []objects.IHitObject{oldObject, retainedObject}

	if !trimBeatmapToWindow(beatMap, 2000, math.Inf(1)) {
		t.Fatal("trimBeatmapToWindow reported no removed objects")
	}

	if len(beatMap.HitObjects) != 1 || beatMap.HitObjects[0] != retainedObject {
		t.Fatalf("retained hit objects = %#v, want only the post-seek object", beatMap.HitObjects)
	}
	if retainedObject.GetID() != 0 {
		t.Fatalf("retained object ID = %d, want 0 after trimming", retainedObject.GetID())
	}
}
