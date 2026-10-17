package states

import (
	"errors"
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/effects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

func TestBloomEffectInitializesOnlyWhenEnabledAndReusesGPUResources(t *testing.T) {
	player := &Player{activeMSAA: 4}
	created := &effects.BloomEffect{}
	callCount := 0
	create := func(width, height, samples int) (*effects.BloomEffect, error) {
		callCount++
		if width != int(settings.Graphics.GetWidth()) || height != int(settings.Graphics.GetHeight()) || samples != 4 {
			t.Errorf("bloom dimensions/samples = %dx%d/%d", width, height, samples)
		}
		return created, nil
	}

	if err := player.ensureBloomEffect(false, create); err != nil {
		t.Fatal(err)
	}
	if callCount != 0 || player.bloomEffect != nil {
		t.Fatal("disabled bloom allocated an effect")
	}

	if err := player.ensureBloomEffect(true, create); err != nil {
		t.Fatal(err)
	}
	if callCount != 1 || player.bloomEffect != created {
		t.Fatal("first bloom enable did not allocate its effect")
	}

	if err := player.ensureBloomEffect(false, create); err != nil {
		t.Fatal(err)
	}
	if err := player.ensureBloomEffect(true, create); err != nil {
		t.Fatal(err)
	}
	if callCount != 1 || player.bloomEffect != created {
		t.Fatal("bloom disable/re-enable allocated replacement resources")
	}
}

func TestBloomFailedLiveEnableWaitsForToggleBeforeRetry(t *testing.T) {
	player := &Player{activeMSAA: 4}
	previousEnabled := settings.Playfield.Bloom.Enabled
	t.Cleanup(func() { settings.Playfield.Bloom.Enabled = previousEnabled })
	settings.Playfield.Bloom.Enabled = true

	failure := errors.New("GPU allocation failed")
	attempts := 0
	create := func(int, int, int) (*effects.BloomEffect, error) {
		attempts++
		if attempts == 1 {
			return nil, failure
		}
		return &effects.BloomEffect{}, nil
	}

	if active, err := player.bloomForFrame(true, create); active || !errors.Is(err, failure) {
		t.Fatalf("first enable = active %t, error %v; want allocation failure", active, err)
	}
	if !settings.Playfield.Bloom.Enabled {
		t.Fatal("failed bloom allocation changed the user's enabled setting")
	}
	// A settings reload can attach a fresh config with Bloom.Enabled=true.
	// That should not retry allocation or produce another error dialog.
	for range 4 {
		if active, err := player.bloomForFrame(true, create); active || err != nil {
			t.Fatalf("enabled reload = active %t, error %v; want suppressed bloom", active, err)
		}
	}
	if attempts != 1 {
		t.Fatalf("reloaded enabled settings caused %d attempts, want 1", attempts)
	}

	if active, err := player.bloomForFrame(false, create); active || err != nil {
		t.Fatalf("disabling bloom = active %t, error %v", active, err)
	}
	if active, err := player.bloomForFrame(true, create); !active || err != nil {
		t.Fatalf("retry after toggle = active %t, error %v", active, err)
	}
	if attempts != 2 || player.bloomEffect == nil {
		t.Fatalf("retry = %d attempts, bloom effect %v; want two attempts and a valid effect", attempts, player.bloomEffect)
	}
	allocated := player.bloomEffect
	if active, err := player.bloomForFrame(false, create); active || err != nil {
		t.Fatalf("disabling after recovery = active %t, error %v", active, err)
	}
	if active, err := player.bloomForFrame(true, create); !active || err != nil {
		t.Fatalf("re-enabling after recovery = active %t, error %v", active, err)
	}
	if attempts != 2 || player.bloomEffect != allocated {
		t.Fatal("bloom re-enable allocated GPU resources instead of reusing them")
	}
}

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

func TestReplayControllerOverlayFollowsPlaybackMode(t *testing.T) {
	previousPlayers := settings.PLAYERS
	previousReplay := settings.REPLAY
	t.Cleanup(func() {
		settings.PLAYERS = previousPlayers
		settings.REPLAY = previousReplay
	})

	tests := []struct {
		name              string
		players           int
		replay            string
		automatedPlayback bool
		wantScoreOverlay  bool
	}{
		{name: "watch replay", players: 1, replay: "play.osr", wantScoreOverlay: true},
		{name: "cursor dance with ui", players: 1, automatedPlayback: true, wantScoreOverlay: true},
		{name: "single participant knockout", players: 1, wantScoreOverlay: false},
		{name: "multi participant knockout", players: 50, wantScoreOverlay: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings.PLAYERS = test.players
			settings.REPLAY = test.replay

			if got := shouldUseScoreOverlayForReplayController(test.automatedPlayback); got != test.wantScoreOverlay {
				t.Fatalf("shouldUseScoreOverlayForReplayController(%t) = %t, want %t", test.automatedPlayback, got, test.wantScoreOverlay)
			}
		})
	}
}

func TestCursorAdditiveAllowed(t *testing.T) {
	tests := []struct {
		name               string
		hasKnockoutOverlay bool
		activePlayers      int
		players            int
		divides            int
		tag                int
		mode               settings.KnockoutCursorBlendingMode
		want               bool
	}{
		{name: "normal solo", players: 1, divides: 1, tag: 1},
		{name: "normal multiplayer", players: 50, divides: 1, tag: 1, want: true},
		{name: "normal mirrored solo", players: 1, divides: 4, tag: 1, want: true},
		{name: "normal TAG", players: 1, divides: 1, tag: 2, want: true},
		{name: "normal ignores knockout preference", players: 50, divides: 1, tag: 1, mode: settings.KnockoutCursorBlendingDisabled, want: true},
		{name: "automated knockout with score overlay", players: 1, divides: 1, tag: 1, mode: settings.KnockoutCursorBlendingAlways},
		{name: "knockout full field", hasKnockoutOverlay: true, activePlayers: 50, players: 50, divides: 1, tag: 1, want: true},
		{name: "knockout final survivor", hasKnockoutOverlay: true, activePlayers: 1, players: 50, divides: 1, tag: 1},
		{name: "knockout last survivor mirrored", hasKnockoutOverlay: true, activePlayers: 1, players: 50, divides: 4, tag: 1, want: true},
		{name: "knockout all eliminated", hasKnockoutOverlay: true, players: 50, divides: 1, tag: 1},
		{name: "knockout revived pair", hasKnockoutOverlay: true, activePlayers: 2, players: 50, divides: 1, tag: 1, want: true},
		{name: "knockout always even without survivors", hasKnockoutOverlay: true, players: 50, divides: 1, tag: 1, mode: settings.KnockoutCursorBlendingAlways, want: true},
		{name: "knockout always last survivor", hasKnockoutOverlay: true, activePlayers: 1, players: 50, divides: 1, tag: 1, mode: settings.KnockoutCursorBlendingAlways, want: true},
		{name: "knockout disabled", hasKnockoutOverlay: true, activePlayers: 50, players: 50, divides: 1, tag: 1, mode: settings.KnockoutCursorBlendingDisabled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := cursorAdditiveAllowed(test.hasKnockoutOverlay, test.activePlayers, test.players, test.divides, test.tag, test.mode)
			if got != test.want {
				t.Fatalf("cursorAdditiveAllowed() = %t, want %t", got, test.want)
			}
		})
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
