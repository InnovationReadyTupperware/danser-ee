package api

type PerfScore struct {
	// Score is the historical score input used by older calculators. Genuine
	// Stable ScoreV1 totals are stored as signed 32-bit values in .osr files, so
	// they fit this field even on 32-bit hosts. Current calculators use
	// LegacyTotalScore when legacy-score provenance matters.
	Score      int
	Accuracy   float64
	MaxCombo   int
	CountGreat int
	CountOk    int
	CountMeh   int
	CountMiss  int
	// SliderBreaks and SliderEnd are retained for historical calculators.
	// Current calculators use the exact slider statistics below instead.
	SliderBreaks int
	SliderEnd    int

	// SliderTickMisses counts missed large ticks and reverse arrows for
	// non-classic scores. SliderTailHits counts successful slider tails.
	SliderTickMisses int
	SliderTailHits   int

	// LegacyTotalScore is present only when the input comes from a genuine
	// Stable ScoreV1 play. Lazer Classic scores must not populate this field.
	LegacyTotalScore *int64
}
