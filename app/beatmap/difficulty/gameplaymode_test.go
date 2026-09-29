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

func TestSetModsFromReplay(t *testing.T) {
	tests := []struct {
		name        string
		replay      *rplpa.Replay
		wantMods    Modifier
		wantClassic ClassicSettings
	}{
		{
			name: "Stable legacy mods",
			replay: &rplpa.Replay{
				OsuVersion: LazerReplayVersion - 1,
				Mods:       uint32(Hidden),
			},
			wantMods:    Hidden | Classic,
			wantClassic: NewClassicSettings(),
		},
		{
			name: "Lazer Classic settings",
			replay: &rplpa.Replay{
				OsuVersion: LazerReplayVersion,
				ScoreInfo: &rplpa.ScoreInfo{Mods: []*rplpa.ModInfo{{
					Acronym: "CL",
					Settings: map[string]any{
						"classic_note_lock": false,
					},
				}}},
			},
			wantMods: Classic,
			wantClassic: func() ClassicSettings {
				settings := NewClassicSettings()
				settings.ClassicNoteLock = false
				return settings
			}(),
		},
		{
			name: "Stable explicit Classic settings",
			replay: &rplpa.Replay{
				OsuVersion: LazerReplayVersion - 1,
				ScoreInfo: &rplpa.ScoreInfo{Mods: []*rplpa.ModInfo{{
					Acronym: "CL",
					Settings: map[string]any{
						"classic_health": false,
					},
				}}},
			},
			wantMods: Classic,
			wantClassic: func() ClassicSettings {
				settings := NewClassicSettings()
				settings.ClassicHealth = false
				return settings
			}(),
		},
		{
			name: "Lazer without Classic",
			replay: &rplpa.Replay{
				OsuVersion: LazerReplayVersion,
				Mods:       uint32(HardRock),
			},
			wantMods: HardRock,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := NewDifficulty(5, 5, 5, 5)
			diff.SetModsFromReplay(test.replay)

			if diff.Mods != test.wantMods {
				t.Fatalf("SetModsFromReplay() mods = %v, want %v", diff.Mods, test.wantMods)
			}

			classic, hasClassic := GetModConfig[ClassicSettings](diff)
			if test.wantMods.Active(Classic) {
				if !hasClassic || classic != test.wantClassic {
					t.Fatalf("SetModsFromReplay() Classic settings = %#v, %t; want %#v, true", classic, hasClassic, test.wantClassic)
				}
			} else if hasClassic {
				t.Fatalf("SetModsFromReplay() unexpectedly created Classic settings: %#v", classic)
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

func TestClassicScoreMultiplierAppliesToStableReplay(t *testing.T) {
	for _, test := range []struct {
		name string
		mode GameplayMode
		mods Modifier
		want float64
	}{
		{"Stable NM replay", GameplayStable, Classic, 0.96},
		{"Stable HDDT replay", GameplayStable, Hidden | DoubleTime | Classic, 1.06 * 1.12 * 0.96},
		{"Lazer Classic", GameplayLazer, Classic, 0.985},
		{"Lazer HDDT Classic", GameplayLazer, Hidden | DoubleTime | Classic, 1.04 * 1.23 * 0.985},
	} {
		t.Run(test.name, func(t *testing.T) {
			diff := NewDifficulty(5, 5, 5, 5)
			diff.SetGameplayMode(test.mode)
			diff.SetMods(test.mods)
			if got := diff.GetScoreMultiplier(); math.Abs(got-test.want) > 1e-12 {
				t.Fatalf("GetScoreMultiplier() = %.8f, want %.8f", got, test.want)
			}
		})
	}

	stableReplay := &rplpa.Replay{OsuVersion: LazerReplayVersion - 1}
	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(GameplayStable)
	diff.SetModsFromReplay(stableReplay)
	if !diff.CheckModActive(Classic) || diff.GetScoreMultiplier() != 0.96 {
		t.Fatalf("Stable replay Classic multiplier = %.8f with mods %v, want 0.96 with Classic", diff.GetScoreMultiplier(), diff.Mods)
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

	if got := lazer.GetScoreMultiplier(); math.Abs(got-0.0985) > 1e-12 {
		t.Fatalf("Lazer Relax multiplier = %v, want 0.0985", got)
	}
}

func TestLazerClassicScoreMultiplierFollowsNoteLockSetting(t *testing.T) {
	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(GameplayLazer)
	diff.SetMods2([]rplpa.ModInfo{{Acronym: "CL", Settings: map[string]any{"classic_note_lock": false}}})
	if got := diff.GetScoreMultiplier(); math.Abs(got-0.96) > 1e-12 {
		t.Fatalf("Classic without note lock multiplier = %v, want 0.96", got)
	}
}

func TestLazerHiddenScoreMultiplierFollowsFadeSetting(t *testing.T) {
	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(GameplayLazer)
	diff.SetMods2([]rplpa.ModInfo{{Acronym: "HD", Settings: map[string]any{"only_fade_approach_circles": true}}})
	if got := diff.GetScoreMultiplier(); math.Abs(got-1.02) > 1e-12 {
		t.Fatalf("Hidden with approach-only fading multiplier = %v, want 1.02", got)
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

func TestAutopilotUsesStableWireIdentityAndUserFacingName(t *testing.T) {
	if got := ParseFromAcronym("AP"); got != Autopilot {
		t.Fatalf("ParseFromAcronym(AP) = %v, want Autopilot", got)
	}

	if got := Autopilot.String(); got != "AP" {
		t.Fatalf("Autopilot.String() = %q, want AP", got)
	}

	fullNames := Autopilot.StringFull()
	if len(fullNames) != 1 || fullNames[0] != "Autopilot" {
		t.Fatalf("Autopilot.StringFull() = %#v, want [Autopilot]", fullNames)
	}

	diff := NewDifficulty(5, 5, 5, 5)
	diff.SetMods(Autopilot)
	mods := diff.ExportMods2()
	if len(mods) != 1 || mods[0].Acronym != "AP" {
		t.Fatalf("ExportMods2() = %#v, want AP", mods)
	}
}

func TestModifierCompatibilityMatchesLazerForSupportedMods(t *testing.T) {
	tests := []struct {
		name string
		mods Modifier
		want bool
	}{
		{name: "easy and hard rock", mods: Easy | HardRock, want: false},
		{name: "easy and difficulty adjust", mods: Easy | DifficultyAdjust, want: false},
		{name: "hard rock and difficulty adjust", mods: HardRock | DifficultyAdjust, want: false},
		{name: "hard rock and mirror", mods: HardRock | Mirror, want: false},
		{name: "speed up and speed down", mods: DoubleTime | HalfTime, want: false},
		{name: "sudden death and perfect", mods: SuddenDeath | Perfect, want: false},
		{name: "no fail and sudden death", mods: NoFail | SuddenDeath, want: false},
		{name: "no fail and relax", mods: NoFail | Relax, want: true},
		{name: "no fail and autopilot", mods: NoFail | Autopilot, want: true},
		{name: "no fail and autoplay", mods: NoFail | Autoplay, want: true},
		{name: "relax and autoplay", mods: Relax | Autoplay, want: false},
		{name: "relax and autopilot", mods: Relax | Autopilot, want: false},
		{name: "relax and spun out", mods: Relax | SpunOut, want: true},
		{name: "autopilot and spun out", mods: Autopilot | SpunOut, want: false},
		{name: "autoplay and spun out", mods: Autoplay | SpunOut, want: false},
		{name: "cinema and no fail", mods: Cinema | NoFail, want: false},
		{name: "cinema and sudden death", mods: Cinema | SuddenDeath, want: false},
		{name: "cinema and autoplay", mods: Cinema | Autoplay, want: false},
		{name: "cinema and relax", mods: Cinema | Relax, want: false},
		{name: "cinema and autopilot", mods: Cinema | Autopilot, want: false},
		{name: "cinema and touch device", mods: Cinema | TouchDevice, want: false},
		{name: "traceable and hidden", mods: Traceable | Hidden, want: false},
		{name: "classic and score v2", mods: Classic | ScoreV2, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mods.Compatible(); got != tt.want {
				t.Fatalf("Modifier(%d).Compatible() = %t, want %t", tt.mods, got, tt.want)
			}
		})
	}
}

func TestParseModsAddsLazerEquivalentSpeedAndFailureModifiers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Modifier
	}{
		{name: "nightcore adds double time", input: "NC", want: Nightcore | DoubleTime},
		{name: "daycore adds half time", input: "DC", want: Daycore | HalfTime},
		{name: "perfect adds sudden death", input: "PF", want: Perfect | SuddenDeath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseMods(tt.input); got != tt.want {
				t.Fatalf("ParseMods(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
