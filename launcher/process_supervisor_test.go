package launcher

import "testing"

func TestParseProcessProgress(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantStatus string
		wantValue  float32
		wantSpeed  float64
	}{
		{
			name:       "standard progress line",
			line:       "Progress: 37.5%, Speed: 2.25x, ETA: 00:12",
			wantOK:     true,
			wantStatus: "37.5%",
			wantValue:  0.375,
			wantSpeed:  2.25,
		},
		{
			name:       "prefix before progress",
			line:       "Encoder output - Progress: 100%, Speed: 1x, ETA: done, extra",
			wantOK:     true,
			wantStatus: "100%",
			wantValue:  1,
			wantSpeed:  1,
		},
		{
			name:   "missing percentage sign",
			line:   "Progress: 50, Speed: 1x, ETA: 00:10",
			wantOK: false,
		},
		{
			name:   "missing fields",
			line:   "Progress: 50%, Speed: 1x",
			wantOK: false,
		},
		{
			name:   "invalid percentage",
			line:   "Progress: NaN%, Speed: 1x, ETA: 00:10",
			wantOK: false,
		},
		{
			name:   "out of range percentage",
			line:   "Progress: 101%, Speed: 1x, ETA: 00:10",
			wantOK: false,
		},
		{
			name:   "missing separator",
			line:   "Progress 50%, Speed: 1x, ETA: 00:10",
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseProcessProgress(test.line)
			if ok != test.wantOK {
				t.Fatalf("parseProcessProgress() ok = %v, want %v", ok, test.wantOK)
			}
			if !test.wantOK {
				return
			}

			if got.status != test.wantStatus || got.value != test.wantValue || got.speedValue != test.wantSpeed {
				t.Fatalf("parseProcessProgress() = %#v, want status=%q value=%v speed=%v", got, test.wantStatus, test.wantValue, test.wantSpeed)
			}
		})
	}
}

func TestParseLabeledFloatRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []string{"Speed: NaN", "Speed: +Inf", "Speed: -Inf", "not a number"} {
		if got := parseLabeledFloat(value); got != 0 {
			t.Fatalf("parseLabeledFloat(%q) = %v, want 0", value, got)
		}
	}
}
