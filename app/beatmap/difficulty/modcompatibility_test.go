package difficulty

import (
	"testing"

	"github.com/wieku/rplpa"
)

func TestModCompatibilityUsesProvenanceAndNormalizesReplayFlags(t *testing.T) {
	for _, test := range []struct {
		name string
		mode GameplayMode
		mods Modifier
		want Modifier
	}{
		{name: "nightcore composite", mods: Nightcore | DoubleTime},
		{name: "daycore composite", mods: Daycore | HalfTime},
		{name: "perfect composite", mods: Perfect | SuddenDeath},
		{name: "stable perfect composite", mode: GameplayStable, mods: Perfect | SuddenDeath},
		{name: "lazer relax no fail", mods: Relax | NoFail},
		{name: "lazer classic relax sudden death", mods: Classic | Relax | SuddenDeath},
		{name: "stable relax no fail", mode: GameplayStable, mods: Relax | NoFail, want: Relax | NoFail},
		{name: "stable autopilot perfect", mode: GameplayStable, mods: Autopilot | Perfect | SuddenDeath, want: Autopilot | Perfect},
		{name: "opposing speed mods", mods: Nightcore | DoubleTime | HalfTime, want: Nightcore | HalfTime},
		{name: "no fail perfect", mods: NoFail | Perfect | SuddenDeath, want: NoFail | Perfect},
		{name: "difficulty adjust easy", mods: DifficultyAdjust | Easy, want: DifficultyAdjust | Easy},
		{name: "difficulty adjust hard rock", mods: DifficultyAdjust | HardRock, want: DifficultyAdjust | HardRock},
		{name: "relax autopilot", mods: Relax | Autopilot, want: Relax | Autopilot},
		{name: "cinema alone", mods: Cinema},
		{name: "cinema relax", mods: Cinema | Relax, want: Cinema | Relax},
		{name: "lazer classic scorev2", mods: Classic | ScoreV2, want: Classic | ScoreV2},
		{name: "stable scorev2 with injected classic", mode: GameplayStable, mods: ScoreV2 | Classic},
		{name: "unsupported target", mods: Target, want: Target},
	} {
		t.Run(test.name, func(t *testing.T) {
			diff := NewDifficulty(5, 5, 5, 5)
			diff.SetGameplayMode(test.mode)
			diff.SetMods(test.mods)
			if got := diff.GetIncompatibleCombo(); got != test.want {
				t.Fatalf("incompatible mods = %s, want %s", got.String(), test.want.String())
			}
		})
	}
}

func TestModCompatibilityMatrixIsSymmetric(t *testing.T) {
	for _, mode := range []GameplayMode{GameplayStable, GameplayLazer} {
		for i := range len(modsString) {
			first := Modifier(1 << i)
			for j := range len(modsString) {
				second := Modifier(1 << j)
				if first.GetIncompatibleMods(mode).Active(second) != second.GetIncompatibleMods(mode).Active(first) {
					t.Fatalf("%s incompatibility is asymmetric for %s and %s", mode, first.String(), second.String())
				}
			}
		}
	}
}

func TestSuddenDeathSettingsRoundTripCloneAndRemoval(t *testing.T) {
	diff := NewDifficulty(5, 5, 5, 5)
	diff.AddMod(SuddenDeath)
	conf, ok := GetModConfig[SuddenDeathSettings](diff)
	if !ok || conf.FailOnSliderTail {
		t.Fatalf("default Sudden Death settings = %#v, %t", conf, ok)
	}
	diff.SetMods2([]rplpa.ModInfo{{Acronym: "SD", Settings: map[string]any{"fail_on_slider_tail": true}}})
	cloned := diff.Clone()
	exported := cloned.ExportMods2()
	if len(exported) != 1 || exported[0].Acronym != "SD" || exported[0].Settings["fail_on_slider_tail"] != true {
		t.Fatalf("exported Sudden Death = %#v", exported)
	}
	roundTrip := NewDifficulty(5, 5, 5, 5)
	roundTrip.SetMods2(exported)
	conf, ok = GetModConfig[SuddenDeathSettings](roundTrip)
	if !ok || !conf.FailOnSliderTail {
		t.Fatalf("round-trip settings = %#v, %t", conf, ok)
	}
	diff.RemoveMod(SuddenDeath)
	if _, ok = GetModConfig[SuddenDeathSettings](diff); ok {
		t.Fatal("removing SD retained its settings")
	}
	conf, ok = GetModConfig[SuddenDeathSettings](cloned)
	if !ok || !conf.FailOnSliderTail {
		t.Fatal("removing SD from the original changed the clone")
	}
	diff.AddMod(SuddenDeath)
	conf, _ = GetModConfig[SuddenDeathSettings](diff)
	if conf.FailOnSliderTail {
		t.Fatal("re-adding SD restored removed settings instead of defaults")
	}
}
