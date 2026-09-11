package objects

import (
	"strconv"
	"strings"

	"github.com/innovationreadytupperware/danser-ee/app/audio"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const legacySampleControlPointLeniency = 5.0

type Renderable interface {
	Draw(time float64, color color2.Color, batch *batch.QuadBatch) bool
	DrawApproach(time float64, color color2.Color, batch *batch.QuadBatch)
}

func commonParse(data []string, extraIndex int) *HitObject {
	x, _ := strconv.ParseFloat(data[0], 32)
	y, _ := strconv.ParseFloat(data[1], 32)
	time, _ := strconv.ParseFloat(data[2], 64)
	objType, _ := strconv.ParseInt(data[3], 10, 64)

	startPos := vector.NewVec2f(float32(x), float32(y))

	hitObject := &HitObject{
		StartPosRaw:   startPos,
		EndPosRaw:     startPos,
		StartTime:     time,
		EndTime:       time,
		HitObjectID:   -1,
		NewCombo:      (objType & 4) == 4,
		ColorOffset:   (objType >> 4) & 7,
		StackIndexMap: make(map[int64]int64),
	}

	hitObject.BasicHitSound = parseExtras(data, extraIndex)

	return hitObject
}

func parseExtras(data []string, extraIndex int) (info audio.HitSoundInfo) {
	if extraIndex < len(data) && len(data[extraIndex]) > 0 {
		extras := strings.Split(data[extraIndex], ":")
		if len(extras) == 0 {
			return info
		}

		info.SampleSet, _ = strconv.Atoi(extras[0])
		if len(extras) > 1 {
			info.AdditionSet, _ = strconv.Atoi(extras[1])
		}

		if len(extras) > 2 {
			info.CustomIndex, _ = strconv.Atoi(extras[2])
		}

		if len(extras) > 3 {
			volume, _ := strconv.Atoi(extras[3])
			info.CustomVolume = float64(volume) / 100.0
		}

		if len(extras) > 4 {
			info.Filename = extras[4]
		}
	}

	return
}
