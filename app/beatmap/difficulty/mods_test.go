package difficulty

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/wieku/rplpa"
)

func TestGetDiffMaskedModsIncludesHiddenForReadingDifficulty(t *testing.T) {
	mods := Hidden | HardRock | NoFail
	got := GetDiffMaskedMods(mods)

	if !got.Active(Hidden) || !got.Active(HardRock) {
		t.Fatalf("difficulty mask = %v, want HD and HR retained", got)
	}
	if got.Active(NoFail) {
		t.Fatalf("difficulty mask = %v, want NF excluded", got)
	}
}

func TestGetDiffMaskedModsNormalizesCompositeSpeedMods(t *testing.T) {
	tests := []struct {
		name  string
		input Modifier
		want  Modifier
	}{
		{name: "nightcore", input: Nightcore, want: DoubleTime},
		{name: "daycore", input: Daycore, want: HalfTime},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := GetDiffMaskedMods(test.input); got != test.want {
				t.Fatalf("difficulty mask = %v, want %v", got, test.want)
			}
		})
	}
}

func TestHiddenSettingsRoundTripAndAffectDifficultyCacheKey(t *testing.T) {
	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetMods2([]rplpa.ModInfo{{
		Acronym: "HD",
		Settings: map[string]any{
			"only_fade_approach_circles": true,
		},
	}})

	settings, ok := GetModConfig[HiddenSettings](diff)
	if !ok || !settings.OnlyFadeApproachCircles {
		t.Fatalf("imported Hidden settings = %#v, %t; want only-fade enabled", settings, ok)
	}
	if diff.HasHiddenObjectFading() {
		t.Fatal("Hidden object fading enabled with only-fade-approach-circles")
	}

	defaultHD := NewDifficulty(5, 5, 5, 5)
	defaultHD.SetMods(Hidden)
	if !defaultHD.HasHiddenObjectFading() {
		t.Fatal("default Hidden should fade object bodies")
	}
	if got, want := diff.GetModStringMasked(), defaultHD.GetModStringMasked(); got == want {
		t.Fatalf("Hidden cache keys collide: %q", got)
	}

	exported := diff.ExportMods2()
	if len(exported) != 1 || exported[0].Acronym != "HD" {
		t.Fatalf("exported mods = %#v, want one HD mod", exported)
	}
	if got, ok := exported[0].Settings["only_fade_approach_circles"].(bool); !ok || !got {
		t.Fatalf("exported Hidden setting = %#v, want true", exported[0].Settings)
	}
}

func TestKnownUnsupportedLazerPerformanceModsWarnAboutParity(t *testing.T) {
	previousWriter := log.Writer()
	var output bytes.Buffer
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	for _, acronym := range []string{"MG", "DF", "BL"} {
		output.Reset()
		if got := ParseFromAcronym(acronym); got != None {
			t.Fatalf("ParseFromAcronym(%q) = %v, want unsupported", acronym, got)
		}
		if message := output.String(); !strings.Contains(message, "star rating/performance may differ") {
			t.Fatalf("diagnostic for %q = %q, want parity warning", acronym, message)
		}
	}
}
