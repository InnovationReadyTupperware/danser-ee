package launcher

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/ffmpeg"
	"github.com/wieku/danser-go/framework/math/math32"
	"github.com/wieku/rplpa"
	"golang.org/x/exp/constraints"
)

type floatParam param[float64]
type intParam param[int32]

type param[T constraints.Integer | constraints.Float] struct {
	ogValue T
	value   T
	changed bool
}

type knockoutReplay struct {
	path         string
	parsedReplay *rplpa.Replay
	included     bool
	mods         difficulty.Modifier
}

type builder struct {
	outputName string
	ssTime     float32

	speed floatParam
	pitch floatParam

	currentMap *beatmap.BeatMap

	replayPath    string
	currentReplay *rplpa.Replay

	offset intParam
	start  intParam
	end    intParam
	skip   bool

	mirrors int32
	tags    int32

	sourceDiff *difficulty.Difficulty
	baseDiff   *difficulty.Difficulty
	diff       *difficulty.Difficulty

	config string

	knockoutReplays []*knockoutReplay
}

func newBuilder() *builder {
	return &builder{
		speed: floatParam{
			ogValue: 1,
			value:   1,
		},
		pitch: floatParam{
			ogValue: 1,
			value:   1,
		},
		offset: intParam{
			ogValue: 0,
			value:   0,
		},
		mirrors:    1,
		tags:       1,
		config:     "default",
		sourceDiff: difficulty.NewDifficulty(1, 1, 1, 1),
		baseDiff:   difficulty.NewDifficulty(1, 1, 1, 1),
		diff:       difficulty.NewDifficulty(1, 1, 1, 1),
	}
}

func (b *builder) setMap(bMap *beatmap.BeatMap) {
	if bMap == nil {
		return
	}

	b.currentMap = bMap

	b.start = intParam{}

	mEnd := int32(math32.Ceil(float32(b.currentMap.Length) / 1000))

	b.end = intParam{
		ogValue: mEnd,
		value:   mEnd,
		changed: false,
	}

	b.offset.value = int32(bMap.LocalOffset)
	b.offset.changed = b.offset.value != 0

	b.sourceDiff.SetAR(bMap.Diff.GetBaseAR())
	b.sourceDiff.SetOD(bMap.Diff.GetBaseOD())
	b.sourceDiff.SetCS(bMap.Diff.GetBaseCS())
	b.sourceDiff.SetHP(bMap.Diff.GetBaseHP())
	b.sourceDiff.SetMods(difficulty.None)
	b.sourceDiff.SetGameplayMode(difficulty.GameplayLazer)

	b.diff.SetAR(bMap.Diff.GetBaseAR())
	b.diff.SetOD(bMap.Diff.GetBaseOD())
	b.diff.SetCS(bMap.Diff.GetBaseCS())
	b.diff.SetHP(bMap.Diff.GetBaseHP())

	b.baseDiff.SetAR(bMap.Diff.GetBaseAR())
	b.baseDiff.SetOD(bMap.Diff.GetBaseOD())
	b.baseDiff.SetCS(bMap.Diff.GetBaseCS())
	b.baseDiff.SetHP(bMap.Diff.GetBaseHP())
	b.baseDiff.SetMods(b.diff.Mods)
	b.baseDiff.SetGameplayMode(difficulty.GameplayLazer)
	b.diff.SetGameplayMode(difficulty.GameplayLazer)
}

func (b *builder) setReplay(replay *rplpa.Replay) {
	if replay == nil {
		return
	}

	b.currentReplay = replay
	gameplayMode := difficulty.GameplayModeFromReplayVersion(int(replay.OsuVersion))
	b.sourceDiff.SetGameplayMode(gameplayMode)
	b.baseDiff.SetGameplayMode(gameplayMode)
	b.diff.SetGameplayMode(gameplayMode)
	b.diff.RemoveMod(^difficulty.None)

	if replay.ScoreInfo != nil && replay.ScoreInfo.Mods != nil && len(replay.ScoreInfo.Mods) > 0 {
		modsNew := make([]rplpa.ModInfo, 0, len(replay.ScoreInfo.Mods))

		for _, mod := range replay.ScoreInfo.Mods {
			if mod != nil {
				modsNew = append(modsNew, *mod)
			}
		}

		if len(modsNew) > 0 {
			b.sourceDiff.SetMods2(modsNew)
			b.baseDiff.SetMods(b.sourceDiff.Mods)
			b.diff.SetMods2(modsNew)
		} else {
			b.sourceDiff.SetMods(difficulty.Modifier(replay.Mods))
			b.baseDiff.SetMods(difficulty.Modifier(replay.Mods))
			b.diff.SetMods(difficulty.Modifier(replay.Mods))
		}
	} else {
		b.sourceDiff.SetMods(difficulty.Modifier(replay.Mods))
		b.baseDiff.SetMods(difficulty.Modifier(replay.Mods))
		b.diff.SetMods(difficulty.Modifier(replay.Mods))
	}

}

func (b *builder) removeReplay() {
	b.currentReplay = nil
	b.sourceDiff.RemoveMod(^difficulty.None)
	b.sourceDiff.SetGameplayMode(difficulty.GameplayLazer)
	b.baseDiff.SetGameplayMode(difficulty.GameplayLazer)
	b.diff.SetGameplayMode(difficulty.GameplayLazer)
}

func (b *builder) numKnockoutReplays() (ret int) {
	for _, replay := range b.knockoutReplays {
		if replay.included {
			ret++
		}
	}

	return
}

func (b *builder) launchDisabled() bool {
	switch launcherConfig.CurrentMode {
	case Replay:
		return b.currentReplay == nil
	case Knockout:
		return b.currentMap == nil || b.numKnockoutReplays() == 0
	default:
		return b.currentMap == nil
	}
}

func (b *builder) getArguments() []string {
	args, err := b.getArgumentsChecked()
	if err != nil {
		return nil
	}

	return args
}

func (b *builder) getArgumentsChecked() (args []string, err error) {
	currentMode := launcherConfig.CurrentMode
	currentPMode := launcherConfig.CurrentPMode

	args = append(args, "-nodbcheck", "-noupdatecheck")

	if b.config != "default" {
		args = append(args, "-settings", b.config)
	}

	if currentMode == Replay {
		if b.currentMap == nil || b.currentReplay == nil || strings.TrimSpace(b.replayPath) == "" {
			return nil, fmt.Errorf("a replay and its beatmap must be selected")
		}

		args = append(args, "-replay", b.replayPath)

		if !b.sourceDiff.Equals(b.diff) {
			bt, marshalErr := json.Marshal(b.diff.ExportMods2())
			if marshalErr != nil {
				return nil, fmt.Errorf("encode replay mods: %w", marshalErr)
			}

			args = append(args, "-mods2", string(bt))
		}
	} else {
		if b.currentMap == nil {
			return nil, fmt.Errorf("a beatmap must be selected")
		}

		args = append(args, "-md5", b.currentMap.MD5)

		diffClone := b.diff.Clone()

		if currentMode == Play {
			args = append(args, "-play")
		} else if currentMode == Knockout {
			var list []string

			for _, r := range b.knockoutReplays {
				if r.included {
					list = append(list, r.path)
				}
			}

			data, marshalErr := json.Marshal(list)
			if marshalErr != nil {
				return nil, fmt.Errorf("encode knockout replays: %w", marshalErr)
			}
			args = append(args, "-knockout2", string(data))
		} else if currentMode == SoloKnockout {
			args = append(args, "-solo-knockout")
		} else if currentMode == DanserReplay {
			diffClone.AddMod(difficulty.Autoplay)
		}

		if diffClone.Mods != difficulty.None {
			bt, marshalErr := json.Marshal(diffClone.ExportMods2())
			if marshalErr != nil {
				return nil, fmt.Errorf("encode mods: %w", marshalErr)
			}

			args = append(args, "-mods2", string(bt))
		}
	}

	if currentMode != Play && currentPMode != Watch {
		oEmpty := true
		outputName := strings.TrimSpace(b.outputName)
		if currentPMode == Record && b.outputName != "" {
			if validationErr := ffmpeg.ValidateOutputName(b.outputName); validationErr != nil {
				return nil, fmt.Errorf("invalid recording output name: %w", validationErr)
			}
			outputName = b.outputName
		}

		if outputName != "" {
			oEmpty = false
			args = append(args, "-out", outputName)
		}

		if currentPMode == Record {
			args = append(args, "-preciseprogress")
		}

		if currentPMode == Screenshot {
			args = append(args, "-ss", strconv.FormatFloat(float64(b.ssTime), 'f', 3, 32))
		} else if oEmpty {
			args = append(args, "-record")
		}
	}

	if usesGeneratedCursors(currentMode) {
		if b.mirrors > 1 {
			args = append(args, "-cursors", strconv.Itoa(int(b.mirrors)))
		}

		if b.tags > 1 {
			args = append(args, "-tag", strconv.Itoa(int(b.tags)))
		}
	}

	if b.start.changed {
		args = append(args, "-start", strconv.FormatFloat(float64(b.start.value), 'f', 1, 32))
	}

	if b.end.changed {
		args = append(args, "-end", strconv.FormatFloat(float64(b.end.value), 'f', 1, 32))
	}

	if b.speed.changed {
		args = append(args, "-speed", strconv.FormatFloat(float64(b.speed.value), 'f', 2, 32))
	}

	if b.pitch.changed {
		args = append(args, "-pitch", strconv.FormatFloat(float64(b.pitch.value), 'f', 2, 32))
	}

	if b.skip {
		args = append(args, "-skip")
	}

	if b.offset.changed && currentPMode != Screenshot {
		args = append(args, "-offset", strconv.Itoa(int(b.offset.value)))
	}

	return
}
