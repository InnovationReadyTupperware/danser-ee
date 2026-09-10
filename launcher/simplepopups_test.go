package launcher

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/wieku/rplpa"
)

func TestNormalizeScreenshotTimeInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "leading decimal", input: ".25", want: "0.25"},
		{name: "decimal point only", input: ".", want: "0."},
		{name: "already normalized", input: "0.25", want: "0.25"},
		{name: "whole number", input: "2", want: "2"},
		{name: "negative value unchanged", input: "-.25", want: "-.25"},
		{name: "invalid text unchanged", input: "abc", want: "abc"},
		{name: "empty input unchanged", input: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeScreenshotTimeInput(test.input); got != test.want {
				t.Fatalf("normalizeScreenshotTimeInput(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestValidateScreenshotTimeInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		maxValue    float64
		want        float32
		wantMessage string
	}{
		{name: "fractional value", input: "0.25", maxValue: 10, want: 0.25},
		{name: "leading decimal", input: ".25", maxValue: 10, want: 0.25},
		{name: "whole number", input: "2", maxValue: 10, want: 2},
		{name: "zero", input: "0", maxValue: 10},
		{name: "exact upper bound", input: "10", maxValue: 10, want: 10},
		{name: "decimal upper bound", input: "115.48", maxValue: 115.48, want: 115.48},
		{
			name:        "negative value",
			input:       "-0.1",
			maxValue:    10,
			wantMessage: "Enter a time between 0 and 10 seconds",
		},
		{
			name:        "value above end",
			input:       "10.1",
			maxValue:    10,
			wantMessage: "Enter a time between 0 and 10 seconds",
		},
		{name: "invalid text", input: "abc", maxValue: 10, wantMessage: "Enter a time in seconds"},
		{name: "empty input", input: "", maxValue: 10, wantMessage: "Enter a time in seconds"},
		{name: "non-finite value", input: "NaN", maxValue: 10, wantMessage: "Enter a time in seconds"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, message := validateScreenshotTimeInput(test.input, test.maxValue)
			if got != test.want || message != test.wantMessage {
				t.Fatalf(
					"validateScreenshotTimeInput(%q, %v) = (%v, %q), want (%v, %q)",
					test.input,
					test.maxValue,
					got,
					message,
					test.want,
					test.wantMessage,
				)
			}
		})
	}
}

func TestScreenshotTimeLimitRequiresSelectedSource(t *testing.T) {
	tests := []struct {
		name   string
		bld    *builder
		mode   Mode
		want   float64
		wantOK bool
	}{
		{name: "nil builder"},
		{name: "no selected map", bld: newBuilder()},
		{
			name: "selected map without duration",
			bld:  &builder{currentMap: &beatmap.BeatMap{}},
		},
		{
			name:   "map mode with selected map",
			bld:    &builder{currentMap: &beatmap.BeatMap{Length: 12_345}},
			want:   12.345,
			wantOK: true,
		},
		{
			name: "replay mode requires replay",
			bld:  &builder{currentMap: &beatmap.BeatMap{Length: 12_345}},
			mode: Replay,
		},
		{
			name: "replay mode with replay",
			bld: &builder{
				currentMap:    &beatmap.BeatMap{Length: 12_345},
				currentReplay: new(rplpa.Replay),
			},
			mode:   Replay,
			want:   12.345,
			wantOK: true,
		},
		{
			name: "knockout mode requires replay selection",
			bld:  &builder{currentMap: &beatmap.BeatMap{Length: 12_345}},
			mode: Knockout,
		},
		{
			name: "knockout mode with replay selection",
			bld: &builder{
				currentMap:      &beatmap.BeatMap{Length: 12_345},
				knockoutReplays: []*knockoutReplay{{included: true}},
			},
			mode:   Knockout,
			want:   12.345,
			wantOK: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := screenshotTimeLimit(test.bld, test.mode)
			if got != test.want || ok != test.wantOK {
				t.Fatalf(
					"screenshotTimeLimit(%v) = (%v, %t), want (%v, %t)",
					test.mode,
					got,
					ok,
					test.want,
					test.wantOK,
				)
			}
		})
	}
}

func TestScreenshotTimeUnavailableTooltip(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		want string
	}{
		{name: "replay", mode: Replay, want: "Select a replay to set the screenshot time."},
		{name: "knockout", mode: Knockout, want: "Select at least one replay to set the screenshot time."},
		{name: "map mode", mode: CursorDance, want: "Select a map to set the screenshot time."},
		{name: "solo knockout", mode: SoloKnockout, want: "Select a map to set the screenshot time."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := screenshotTimeUnavailableTooltip(test.mode); got != test.want {
				t.Fatalf("screenshotTimeUnavailableTooltip(%v) = %q, want %q", test.mode, got, test.want)
			}
		})
	}
}

func TestFormatScreenshotTime(t *testing.T) {
	if got := formatScreenshotTime(1.25); got != "1.250" {
		t.Fatalf("formatScreenshotTime(1.25) = %q, want %q", got, "1.250")
	}
}

func TestScreenshotTimePlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		maxValue float64
		want     string
	}{
		{name: "compact decimal", maxValue: 115.48, want: "0-115.48"},
		{name: "millisecond precision", maxValue: 12.345, want: "0-12.345"},
		{name: "unavailable", maxValue: 0, want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := screenshotTimePlaceholder(test.maxValue); got != test.want {
				t.Fatalf("screenshotTimePlaceholder(%v) = %q, want %q", test.maxValue, got, test.want)
			}
		})
	}
}

func TestScreenshotTimeFieldStateValidation(t *testing.T) {
	state := newScreenshotTimeFieldState(1.25)
	if state.text != "1.250" || state.validationError != "" || state.validationActive {
		t.Fatalf("initial state = %#v, want formatted text without error", state)
	}

	state.text = "12"
	if _, valid := state.validate(10); valid {
		t.Fatal("out-of-range value validated successfully")
	}
	if state.validationError != "Enter a time between 0 and 10 seconds" || !state.validationActive {
		t.Fatalf("validationError = %q", state.validationError)
	}

	state.text = ".5"
	state.revalidate(10)
	if state.validationError != "" {
		t.Fatalf("corrected validationError = %q, want empty", state.validationError)
	}
	if state.text != ".5" {
		t.Fatalf("live correction changed text to %q, want editing text preserved", state.text)
	}

	value, valid := state.commit(10)
	if !valid || value != 0.5 {
		t.Fatalf("leading decimal commit = (%v, %t), want (0.5, true)", value, valid)
	}
	if state.text != "0.500" || state.validationError != "" {
		t.Fatalf("committed state = %#v, want formatted text without error", state)
	}

	state.text = "5.1236"
	value, valid = state.commit(10)
	if !valid || value != float32(5.124) {
		t.Fatalf("precision commit = (%v, %t), want (5.124, true)", value, valid)
	}
	if state.text != "5.124" {
		t.Fatalf("precision commit text = %q, want %q", state.text, "5.124")
	}
}

func TestScreenshotTimeFieldStateSyncLimit(t *testing.T) {
	state := newScreenshotTimeFieldState(8)

	state.syncLimit(10, true)
	if state.validationError != "" || state.validationActive {
		t.Fatalf("valid initial limit state = %#v, want no active validation", state)
	}

	state.syncLimit(5, true)
	if state.validationError != "Enter a time between 0 and 5 seconds" || !state.validationActive {
		t.Fatalf("shorter source limit state = %#v, want visible range error", state)
	}

	state.text = "4"
	state.revalidate(5)
	if state.validationError != "" || !state.validationActive {
		t.Fatalf("corrected state = %#v, want active validation without error", state)
	}

	state.syncLimit(0, false)
	if state.validationError != "" {
		t.Fatalf("unavailable source validationError = %q, want empty", state.validationError)
	}
}

func TestScreenshotTimeNeedsConfiguration(t *testing.T) {
	tests := []struct {
		name string
		bld  *builder
		mode Mode
		want bool
	}{
		{name: "source unavailable", bld: newBuilder()},
		{
			name: "valid value",
			bld: &builder{
				currentMap: &beatmap.BeatMap{Length: 10_000},
				ssTime:     5,
			},
		},
		{
			name: "float32 value at decimal upper bound",
			bld: &builder{
				currentMap: &beatmap.BeatMap{Length: 115_480},
				ssTime:     115.48,
			},
		},
		{
			name: "value past selected map",
			bld: &builder{
				currentMap: &beatmap.BeatMap{Length: 10_000},
				ssTime:     12,
			},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := screenshotTimeNeedsConfiguration(test.bld, test.mode); got != test.want {
				t.Fatalf("screenshotTimeNeedsConfiguration() = %t, want %t", got, test.want)
			}
		})
	}
}
