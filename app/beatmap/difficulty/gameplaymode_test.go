package difficulty

import (
	"math"
	"testing"

	"github.com/wieku/rplpa"
)

func TestGameplayModeFromReplayVersion(t *testing.T) {
	tests := []struct {
		name       string
		osuVersion int
		want       GameplayMode
	}{
		{name: "stable before lazer boundary", osuVersion: LazerReplayVersion - 1, want: GameplayStable},
		{name: "stable old replay", osuVersion: 20190506, want: GameplayStable},
		{name: "lazer boundary", osuVersion: LazerReplayVersion, want: GameplayLazer},
		{name: "new lazer replay", osuVersion: LazerReplayVersion + 1, want: GameplayLazer},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := GameplayModeFromReplayVersion(test.osuVersion); got != test.want {
				t.Fatalf("GameplayModeFromReplayVersion(%d) = %v, want %v", test.osuVersion, got, test.want)
			}
		})
	}
}

func TestClassicDoesNotChangeGameplayMode(t *testing.T) {
	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(GameplayStable)
	diff.SetMods(Classic)

	if got := diff.GetGameplayMode(); got != GameplayStable {
		t.Fatalf("Classic changed gameplay mode to %v, want Stable", got)
	}

	if diff.IsLazer() {
		t.Fatal("Stable gameplay became Lazer after enabling Classic")
	}

	classicSettings, ok := GetModConfig[ClassicSettings](diff)
	if !ok || !classicSettings.NoSliderHeadAccuracy {
		t.Fatal("Classic settings were not initialized with their stable-compatible defaults")
	}

	diff.SetGameplayMode(GameplayLazer)
	if !diff.IsLazer() {
		t.Fatal("Lazer gameplay mode was not retained after enabling Classic")
	}
}

func TestLegacyLazerModifierIsIgnoredAndNotExported(t *testing.T) {
	if got := ParseFromAcronym("LZ"); got != None {
		t.Fatalf("ParseFromAcronym(LZ) = %v, want None", got)
	}

	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetMods(reservedLazerModifier | Classic)

	if got := diff.Mods; got != Classic {
		t.Fatalf("SetMods retained the reserved Lazer bit: %v", got)
	}

	mods := diff.ExportMods2()
	if len(mods) != 1 || mods[0].Acronym != "CL" {
		t.Fatalf("ExportMods2() = %#v, want only CL", mods)
	}
}

func TestRelaxScoreMultiplierFollowsGameplayMode(t *testing.T) {
	stable := NewDifficulty(5, 5, 5, 5)
	stable.SetGameplayMode(GameplayStable)
	stable.SetMods(Relax | Classic)

	if got := stable.GetScoreMultiplier(); got != 0 {
		t.Fatalf("Stable Relax multiplier = %v, want 0", got)
	}

	lazer := NewDifficulty(5, 5, 5, 5)
	lazer.SetGameplayMode(GameplayLazer)
	lazer.SetMods(Relax | Classic)

	if got := lazer.GetScoreMultiplier(); math.Abs(got-0.096) > 1e-12 {
		t.Fatalf("Lazer Relax multiplier = %v, want 0.096", got)
	}
}

func TestSetMods2KeepsClassicSettingsSeparateFromMode(t *testing.T) {
	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(GameplayStable)
	diff.SetMods2([]rplpa.ModInfo{{
		Acronym: "CL",
		Settings: map[string]any{
			"classic_note_lock": false,
		},
	}})

	if got := diff.GetGameplayMode(); got != GameplayStable {
		t.Fatalf("SetMods2 changed gameplay mode to %v, want Stable", got)
	}

	classicSettings, ok := GetModConfig[ClassicSettings](diff)
	if !ok || classicSettings.ClassicNoteLock {
		t.Fatalf("SetMods2 did not preserve the supplied Classic setting: %#v", classicSettings)
	}
}
