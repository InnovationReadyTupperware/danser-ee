package settings

import "testing"

func TestDefaultCursorDanceUsesMaximumSpinnerRate(t *testing.T) {
	config := NewConfigFile()

	if config.CursorDance.SpinnerBehavior == nil {
		t.Fatal("CursorDance.SpinnerBehavior is nil")
	}

	if config.CursorDance.SpinnerBehavior.SpinAtLowestRPM {
		t.Fatal("SpinAtLowestRPM default = true, want false")
	}
}

func TestNormalizeCursorDanceRepairsMissingSpinnerConfiguration(t *testing.T) {
	previous := CursorDance
	t.Cleanup(func() {
		CursorDance = previous
	})

	CursorDance = nil
	NormalizeCursorDance()

	if CursorDance == nil || len(CursorDance.Spinners) != 1 || CursorDance.Spinners[0] == nil {
		t.Fatalf("normalized cursor dance has no safe spinner profile: %+v", CursorDance)
	}

	CursorDance.Spinners = nil
	NormalizeCursorDance()
	if len(CursorDance.Spinners) != 1 || CursorDance.Spinners[0] == nil {
		t.Fatalf("normalized empty spinner list = %+v, want one default profile", CursorDance.Spinners)
	}

	CursorDance.Spinners[0] = nil
	NormalizeCursorDance()
	if CursorDance.Spinners[0] == nil {
		t.Fatal("NormalizeCursorDance left a nil spinner entry")
	}
}

func TestNormalizeCursorDanceRepairsEmptyMoverSettings(t *testing.T) {
	previous := CursorDance
	t.Cleanup(func() {
		CursorDance = previous
	})

	config := NewConfigFile()
	config.CursorDance.MoverSettings = &moverSettings{
		Bezier:     []*bezier{nil},
		Flower:     nil,
		HalfCircle: []*circular{nil},
		Spline:     nil,
		Momentum:   []*momentum{nil},
		ExGon:      nil,
		Linear:     []*linear{nil},
		Pippi:      nil,
	}
	CursorDance = config.CursorDance

	NormalizeCursorDance()

	settings := CursorDance.MoverSettings
	if len(settings.Bezier) != 1 || settings.Bezier[0] == nil {
		t.Fatalf("normalized Bezier settings = %#v, want one non-nil entry", settings.Bezier)
	}
	if len(settings.Flower) != 1 || settings.Flower[0] == nil {
		t.Fatalf("normalized Flower settings = %#v, want one non-nil entry", settings.Flower)
	}
	if len(settings.HalfCircle) != 1 || settings.HalfCircle[0] == nil {
		t.Fatalf("normalized HalfCircle settings = %#v, want one non-nil entry", settings.HalfCircle)
	}
	if len(settings.Spline) != 1 || settings.Spline[0] == nil {
		t.Fatalf("normalized Spline settings = %#v, want one non-nil entry", settings.Spline)
	}
	if len(settings.Momentum) != 1 || settings.Momentum[0] == nil {
		t.Fatalf("normalized Momentum settings = %#v, want one non-nil entry", settings.Momentum)
	}
	if len(settings.ExGon) != 1 || settings.ExGon[0] == nil {
		t.Fatalf("normalized ExGon settings = %#v, want one non-nil entry", settings.ExGon)
	}
	if len(settings.Linear) != 1 || settings.Linear[0] == nil {
		t.Fatalf("normalized Linear settings = %#v, want one non-nil entry", settings.Linear)
	}
	if len(settings.Pippi) != 1 || settings.Pippi[0] == nil {
		t.Fatalf("normalized Pippi settings = %#v, want one non-nil entry", settings.Pippi)
	}
}
