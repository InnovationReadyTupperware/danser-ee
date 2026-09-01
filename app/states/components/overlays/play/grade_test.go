package play

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
)

func TestGradeFallbackColor(t *testing.T) {
	tests := []struct {
		name  string
		grade osu.Grade
		want  color2.Color
	}{
		{name: "F", grade: osu.F, want: color2.NewI(lazerFGradeColor)},
		{name: "regular grade", grade: osu.A, want: color2.NewL(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GradeFallbackColor(tt.grade); got != tt.want {
				t.Fatalf("GradeFallbackColor(%v) = %#v, want %#v", tt.grade, got, tt.want)
			}
		})
	}
}
