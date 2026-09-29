package play

import (
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/osuapi"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func TestScoreboardEntryUsesSelectedLazerDisplayScale(t *testing.T) {
	previous := settings.Gameplay.Score.DisplayMode
	t.Cleanup(func() { settings.Gameplay.Score.DisplayMode = previous })

	entry := &ScoreboardEntry{
		isLazer: true,
		score: osuapi.Score{
			TotalScore:        1186456,
			ClassicTotalScore: 128259993,
		},
	}

	settings.Gameplay.Score.DisplayMode = settings.ScoreDisplayStandardised
	if got := entry.getScore(); got != 1186456 {
		t.Fatalf("standardised scoreboard score = %d, want 1186456", got)
	}

	settings.Gameplay.Score.DisplayMode = settings.ScoreDisplayClassic
	if got := entry.getScore(); got != 128259993 {
		t.Fatalf("classic scoreboard score = %d, want 128259993", got)
	}

	entry.isLazer = false
	entry.score.Score = 165764484
	if got := entry.getScore(); got != 165764484 {
		t.Fatalf("Stable scoreboard score = %d, want original ScoreV1 total", got)
	}

	entry.isPlayer = true
	settings.Gameplay.Score.DisplayMode = settings.ScoreDisplayStandardised
	if got := entry.getScore(); got != 1186456 {
		t.Fatalf("recorded Stable player standardised score = %d, want 1186456", got)
	}
	settings.Gameplay.Score.DisplayMode = settings.ScoreDisplayClassic
	if got := entry.getScore(); got != 128259993 {
		t.Fatalf("recorded Stable player Classic score = %d, want 128259993", got)
	}
}
