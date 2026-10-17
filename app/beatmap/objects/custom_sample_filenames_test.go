package objects

import (
	"reflect"
	"testing"
)

func TestCustomSampleFilenamesIncludeCircleSpinnerAndSliderEdges(t *testing.T) {
	circle := NewCircle([]string{
		"256", "192", "1000", "1", "0", "0:0:0:0:Samples/circle.wav",
	})
	spinner := NewSpinner([]string{
		"256", "192", "1500", "8", "0", "2000", "0:0:0:0:Samples/spinner.ogg",
	})
	slider := NewSlider([]string{
		"256", "192", "2500", "2", "1",
		"L|356:192", "2", "100", "0|0|0",
		"0:0:0:0:Samples/head.wav|0:0:0:0:Samples/repeat.wav|0:0:0:0:Samples/tail.mp3",
		"0:0:0:0:ignored-slider.wav",
	})
	if slider == nil {
		t.Fatal("slider was rejected")
	}

	for _, test := range []struct {
		name string
		obj  IHitObject
		want []string
	}{
		{"circle", circle, []string{"Samples/circle.wav"}},
		{"spinner", spinner, []string{"Samples/spinner.ogg"}},
		{"slider", slider, []string{"Samples/head.wav", "Samples/repeat.wav", "Samples/tail.mp3"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider, ok := test.obj.(interface{ GetCustomSampleFilenames() []string })
			if !ok {
				t.Fatal("object does not expose its authored sample filenames")
			}
			if got := provider.GetCustomSampleFilenames(); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("GetCustomSampleFilenames() = %q, want %q", got, test.want)
			}
		})
	}
}
