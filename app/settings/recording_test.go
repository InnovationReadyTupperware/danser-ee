package settings

import (
	"slices"
	"testing"
)

func TestNormalizeRecordingRepairsNullableSubsections(t *testing.T) {
	config := NewConfigFile()
	config.Recording.MotionBlur = nil
	config.Recording.X264Settings = nil
	config.Recording.AACSettings = nil

	config.normalizeRecording()

	if config.Recording.MotionBlur == nil || config.Recording.X264Settings == nil || config.Recording.AACSettings == nil {
		t.Fatal("normalizeRecording left a nullable recording subsection")
	}
}

func TestParseCustomOptions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "empty"},
		{name: "plain", input: "-preset slow", want: []string{"-preset", "slow"}},
		{name: "quoted", input: `-metadata "title=Danser Render"`, want: []string{"-metadata", "title=Danser Render"}},
		{name: "single quoted", input: `-vf 'scale=1280:720'`, want: []string{"-vf", "scale=1280:720"}},
		{name: "escaped whitespace", input: `-metadata title=Danser\ Render`, want: []string{"-metadata", "title=Danser Render"}},
		{name: "Windows path", input: `-attach C:\fonts\danser.ttf`, want: []string{"-attach", `C:\fonts\danser.ttf`}},
		{name: "empty argument", input: `-metadata ""`, want: []string{"-metadata", ""}},
		{name: "unmatched quote", input: `-metadata "title=x`, wantErr: true},
		{name: "dangling escape", input: `-metadata title=x\`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCustomOptions(nil, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCustomOptions(%q) error = %v, wantErr %t", tt.input, err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("parseCustomOptions(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}
