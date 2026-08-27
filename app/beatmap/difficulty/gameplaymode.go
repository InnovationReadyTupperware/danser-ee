package difficulty

// LazerReplayVersion is the first osu! version that identifies a replay as
// having been recorded by osu!lazer rather than osu!stable.
const LazerReplayVersion = 30000000

// GameplayMode identifies the gameplay implementation used to judge an
// osu!standard play. It is deliberately separate from Modifier: osu!standard
// is the ruleset in both cases, while replay provenance determines whether
// danser must reproduce osu!stable or osu!lazer behaviour.
type GameplayMode uint8

const (
	// GameplayLazer is the default for newly created difficulties and for
	// gameplay without replay provenance.
	GameplayLazer GameplayMode = iota

	// GameplayStable reproduces osu!stable gameplay for a stable-versioned
	// replay. Classic settings supplied with a replay do not change this mode.
	GameplayStable
)

// GameplayModeFromReplayVersion maps the version stored in an osu! replay to
// the gameplay implementation that must judge it.
func GameplayModeFromReplayVersion(osuVersion int) GameplayMode {
	if osuVersion >= LazerReplayVersion {
		return GameplayLazer
	}

	return GameplayStable
}

// String returns the stable diagnostic name for the gameplay implementation.
func (mode GameplayMode) String() string {
	if mode == GameplayStable {
		return "Stable"
	}

	return "Lazer"
}

// IsLazer reports whether the mode uses the osu!lazer gameplay
// implementation.
func (mode GameplayMode) IsLazer() bool {
	return mode == GameplayLazer
}

// SetGameplayMode records the gameplay implementation for this difficulty.
// Unknown values are normalized to the Lazer default so malformed external
// input cannot silently select an unimplemented behavior mode.
func (diff *Difficulty) SetGameplayMode(mode GameplayMode) {
	if mode != GameplayStable {
		mode = GameplayLazer
	}

	diff.gameplayMode = mode
}

// GetGameplayMode returns the gameplay implementation associated with this
// difficulty.
func (diff *Difficulty) GetGameplayMode() GameplayMode {
	return diff.gameplayMode
}

// IsLazer reports whether this difficulty uses the osu!lazer gameplay
// implementation.
func (diff *Difficulty) IsLazer() bool {
	return diff.gameplayMode.IsLazer()
}
