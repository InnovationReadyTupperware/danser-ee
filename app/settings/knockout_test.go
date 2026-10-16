package settings

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestDefaultKnockoutCursorBlendingIsAdaptive(t *testing.T) {
	config := NewConfigFile()
	if got := config.Knockout.CursorBlendingMode; got != KnockoutCursorBlendingAdaptive {
		t.Fatalf("default knockout cursor blending = %d, want adaptive", got)
	}
	data, err := json.Marshal(config.Knockout)
	if err != nil {
		t.Fatalf("marshal default knockout settings: %v", err)
	}
	if !strings.Contains(string(data), `"CursorBlendingMode":0`) {
		t.Fatalf("default knockout settings do not serialize Adaptive as 0: %s", data)
	}
}

func TestKnockoutCursorBlendingModeRoundTrip(t *testing.T) {
	for _, test := range []struct {
		mode  KnockoutCursorBlendingMode
		value int
	}{
		{KnockoutCursorBlendingAdaptive, 0},
		{KnockoutCursorBlendingAlways, 1},
		{KnockoutCursorBlendingDisabled, 2},
	} {
		mode := test.mode
		if int(mode) != test.value {
			t.Fatalf("mode %v has serialized value %d, want %d", mode, int(mode), test.value)
		}
		config := NewConfigFile()
		config.Knockout.CursorBlendingMode = mode
		data, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("marshal mode %d: %v", mode, err)
		}
		if !strings.Contains(string(data), `"CursorBlendingMode":`+strconv.Itoa(test.value)) {
			t.Fatalf("serialized mode %d did not use value %d: %s", mode, test.value, data)
		}

		loaded := NewConfigFile()
		if err := json.Unmarshal(data, loaded); err != nil {
			t.Fatalf("unmarshal mode %d: %v", mode, err)
		}
		if loaded.Knockout.CursorBlendingMode != mode {
			t.Fatalf("restored mode = %d, want %d", loaded.Knockout.CursorBlendingMode, mode)
		}
	}
}

func TestKnockoutCursorBlendingModes(t *testing.T) {
	tests := []struct {
		name      string
		mode      KnockoutCursorBlendingMode
		instances int
		want      bool
	}{
		{"adaptive empty", KnockoutCursorBlendingAdaptive, 0, false},
		{"adaptive solo", KnockoutCursorBlendingAdaptive, 1, false},
		{"adaptive pair", KnockoutCursorBlendingAdaptive, 2, true},
		{"adaptive mirrored solo", KnockoutCursorBlendingAdaptive, 4, true},
		{"adaptive full knockout", KnockoutCursorBlendingAdaptive, 50, true},
		{"always empty", KnockoutCursorBlendingAlways, 0, true},
		{"always solo", KnockoutCursorBlendingAlways, 1, true},
		{"always mirrored solo", KnockoutCursorBlendingAlways, 4, true},
		{"disabled empty", KnockoutCursorBlendingDisabled, 0, false},
		{"disabled solo", KnockoutCursorBlendingDisabled, 1, false},
		{"disabled full knockout", KnockoutCursorBlendingDisabled, 50, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.mode.UseAdditiveBlending(test.instances); got != test.want {
				t.Fatalf("UseAdditiveBlending(%d) = %t, want %t", test.instances, got, test.want)
			}
		})
	}
}
