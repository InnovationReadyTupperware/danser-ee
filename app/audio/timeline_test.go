package audio

import (
	"math"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestScheduledMixerTime(t *testing.T) {
	defer ResetClock()

	SetClockSnapshot(ClockSnapshot{
		GameplayTimeMs:   1000,
		MixerTimeSeconds: 2,
		PlaybackRate:     2,
	})

	tests := []struct {
		name      string
		eventTime float64
		want      float64
	}{
		{name: "future event", eventTime: 1500, want: 2.25},
		{name: "past event", eventTime: 500, want: 1.75},
		{name: "invalid event", eventTime: math.NaN(), want: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := scheduledMixerTime(test.eventTime); got != test.want && !(math.IsNaN(got) && math.IsNaN(test.want)) {
				t.Fatalf("scheduledMixerTime(%v) = %v, want %v", test.eventTime, got, test.want)
			}
		})
	}

	SetClockSnapshot(ClockSnapshot{
		GameplayTimeMs:   1000,
		MixerTimeSeconds: 2,
		PlaybackRate:     0,
		Valid:            true,
	})
	if got := scheduledMixerTime(1000); got != -1 {
		t.Fatalf("invalid clock scheduled mixer time = %v, want -1", got)
	}
}

func TestEffectiveSampleVolume(t *testing.T) {
	previous := settings.Audio.IgnoreBeatmapSampleVolume
	defer func() { settings.Audio.IgnoreBeatmapSampleVolume = previous }()

	settings.Audio.IgnoreBeatmapSampleVolume = false
	tests := []struct {
		name         string
		timingVolume float64
		customVolume float64
		want         float64
	}{
		{name: "lazer floor", timingVolume: 0.03, want: minimumVolume},
		{name: "custom overrides timing", timingVolume: 0.2, customVolume: 0.5, want: 0.5},
		{name: "zero custom uses timing", timingVolume: 0.2, customVolume: 0, want: 0.2},
		{name: "invalid timing defaults to unity", timingVolume: math.NaN(), want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EffectiveSampleVolume(test.timingVolume, test.customVolume); got != test.want {
				t.Fatalf("EffectiveSampleVolume(%v, %v) = %v, want %v", test.timingVolume, test.customVolume, got, test.want)
			}
		})
	}

	settings.Audio.IgnoreBeatmapSampleVolume = true
	if got := EffectiveSampleVolume(0.01, 0.01); got != 1 {
		t.Fatalf("ignored sample volume = %v, want 1", got)
	}
}

func TestNormalizeSampleSetUsesNormalForLegacyNone(t *testing.T) {
	if got := normalizeSampleSet(0); got != 1 {
		t.Fatalf("normalizeSampleSet(0) = %d, want normal bank 1", got)
	}
}

func TestPlaySampleAtKeepsComponentVolumesEqual(t *testing.T) {
	const objectID = int64(0x5a11d3)

	type component struct {
		hitsoundIndex int
		volume        float64
	}

	var got []component
	AddListener(func(_ int, hitsoundIndex, _ int, volume float64, objNum int64) {
		if objNum != objectID {
			return
		}

		got = append(got, component{hitsoundIndex: hitsoundIndex, volume: volume})
	})

	PlaySampleAt(1000, 1, 2, 1|2|4|8, 1, 0.4, 0, objectID, 256)

	if len(got) != 4 {
		t.Fatalf("component count = %d, want 4", len(got))
	}

	for _, sample := range got {
		if sample.volume != 0.4 {
			t.Fatalf("component %d volume = %g, want 0.4", sample.hitsoundIndex, sample.volume)
		}
	}
}

func TestBeatmapFileSamplePreservesAdditionLayers(t *testing.T) {
	const objectID = int64(0x5a11d4)

	var got []int
	AddListener(func(_ int, hitsoundIndex, _ int, _ float64, objNum int64) {
		if objNum == objectID {
			got = append(got, hitsoundIndex)
		}
	})

	// A missing custom file falls back to hitnormal, but the authored whistle
	// must still play. The same addition path is used when the custom file is
	// present; only the normal layer lookup changes.
	PlayBeatmapFileSampleAt(1000, "missing-custom.wav", 2, 2, 1, 0.4, 0, objectID, 256)

	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("custom filename layers = %v, want normal fallback + whistle", got)
	}
}

func TestSplitBeforeDigit(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{name: "hitnormal", want: []string{"hitnormal"}},
		{name: "hitnormal2", want: []string{"hitnormal", "2"}},
		{name: "slidertick12", want: []string{"slidertick", "12"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := splitBeforeDigit(test.name)
			if len(got) != len(test.want) {
				t.Fatalf("splitBeforeDigit(%q) = %#v, want %#v", test.name, got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("splitBeforeDigit(%q) = %#v, want %#v", test.name, got, test.want)
				}
			}
		})
	}
}

func TestSampleBalanceMatchesLazerPositionalHitsounds(t *testing.T) {
	previousDivides := settings.DIVIDES
	previousSeparation := settings.Audio.HitsoundStereoSeparation
	t.Cleanup(func() {
		settings.DIVIDES = previousDivides
		settings.Audio.HitsoundStereoSeparation = previousSeparation
	})

	settings.DIVIDES = 1
	settings.Audio.HitsoundStereoSeparation = 0.2

	tests := []struct {
		name string
		xPos float64
		want float64
	}{
		{name: "left playfield edge", xPos: 0, want: -0.2},
		{name: "right playfield edge", xPos: 512, want: 0.2},
		{name: "far left authored point", xPos: -29156, want: -1},
		{name: "far right authored point", xPos: 3322, want: 1},
		{name: "center", xPos: 256, want: 0},
		{name: "invalid point", xPos: math.NaN(), want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sampleBalance(test.xPos); got != test.want {
				t.Fatalf("sampleBalance(%v) = %v, want %v", test.xPos, got, test.want)
			}
		})
	}

	settings.Audio.HitsoundStereoSeparation = 1
	if got := sampleBalance(512); got != 1 {
		t.Fatalf("stereo separation 1 edge balance = %v, want 1", got)
	}
}
