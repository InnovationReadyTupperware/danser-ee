package osu

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestClassicDisplayScoreMatchesJustability(t *testing.T) {
	const standardisedScore = 1186456
	const objectCount = 1821
	const wantClassic = 128259993

	if got := ClassicDisplayScore(standardisedScore, objectCount); got != wantClassic {
		t.Fatalf("ClassicDisplayScore(%d, %d) = %d, want %d", standardisedScore, objectCount, got, wantClassic)
	}
}

func TestRecordedJustabilityScoreMigration(t *testing.T) {
	replay := RecordedStableScore{
		TotalScore: 165764484,
		MaxCombo:   2685,
		Count300:   1800,
		Count100:   17,
		Count50:    3,
		CountMiss:  1,
	}
	attrs := legacyMigrationAttributes{
		accuracyScore:   593060,
		comboScore:      158603640,
		bonusScore:      34600,
		bonusScoreRatio: 1720.0 / 34600,
		maxCombo:        2868,
	}
	const legacyMods = 1.06 * 1.12
	const modernMods = 1.04 * 1.23 * 0.985
	if got := migrateLegacyScoreV1(replay, attrs, legacyMods, modernMods); got != 1186456 {
		t.Fatalf("migrated Stable score = %d, want 1186456", got)
	}
}

func TestDisplayScoreKeepsGameplayScoreSeparate(t *testing.T) {
	cursor := &graphics.Cursor{}
	set := &OsuRuleSet{
		beatMap: &beatmap.BeatMap{HitObjects: make([]objects.IHitObject, 1821)},
		cursors: map[*graphics.Cursor]*subSet{
			cursor: {
				score:          &Score{Score: 1186456},
				scoreProcessor: &scoreV3Processor{score: 1186456},
			},
		},
	}

	if got := set.GetDisplayScore(cursor, settings.ScoreDisplayStandardised); got != 1186456 {
		t.Fatalf("standardised display = %d, want 1186456", got)
	}
	if got := set.GetDisplayScore(cursor, settings.ScoreDisplayClassic); got != 128259993 {
		t.Fatalf("classic display = %d, want 128259993", got)
	}
	if got := set.GetScore(cursor).Score; got != 1186456 {
		t.Fatalf("gameplay score changed to %d, want 1186456", got)
	}

	set.cursors[cursor].scoreProcessor = &scoreV1Processor{score: 165764484}
	set.cursors[cursor].score.Score = 165764484
	if got := set.GetDisplayScore(cursor, settings.ScoreDisplayStandardised); got != 165764484 {
		t.Fatalf("Stable display = %d, want original ScoreV1 total", got)
	}
}

func TestRecordedScoreAvailableWhenResultsBegin(t *testing.T) {
	cursor := &graphics.Cursor{}
	set := &OsuRuleSet{cursors: map[*graphics.Cursor]*subSet{
		cursor: {
			score: &Score{Score: 165607689},
			recordedScore: &recordedScoreDisplay{
				standardised: 1186456,
				classic:      128259993,
			},
		},
	}}
	if got := set.GetDisplayScore(cursor, settings.ScoreDisplayStandardised); got != 165607689 {
		t.Fatalf("playback display = %d, want 165607689", got)
	}
	set.ShowRecordedStableScore(cursor)
	if got := set.GetDisplayScore(cursor, settings.ScoreDisplayStandardised); got != 1186456 {
		t.Fatalf("standardised result = %d, want 1186456", got)
	}
	if got := set.GetDisplayScore(cursor, settings.ScoreDisplayClassic); got != 128259993 {
		t.Fatalf("Classic result = %d, want 128259993", got)
	}
	if got := set.GetScore(cursor).Score; got != 165607689 {
		t.Fatalf("gameplay score = %d, want 165607689", got)
	}
}
