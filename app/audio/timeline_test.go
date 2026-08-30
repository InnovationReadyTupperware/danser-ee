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
		{name: "stable floor", timingVolume: 0.03, want: minimumVolume},
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

func TestSampleBalanceClampsOutOfPlayfieldPositionsBeforeScaling(t *testing.T) {
	previousDivides := settings.DIVIDES
	previousMultiplier := settings.Audio.HitsoundPositionMultiplier
	t.Cleanup(func() {
		settings.DIVIDES = previousDivides
		settings.Audio.HitsoundPositionMultiplier = previousMultiplier
	})

	settings.DIVIDES = 1
	settings.Audio.HitsoundPositionMultiplier = 0.2

	tests := []struct {
		name string
		xPos float64
		want float64
	}{
		{name: "far left authored point", xPos: -29156, want: -0.1},
		{name: "far right authored point", xPos: 3322, want: 0.1},
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
}
