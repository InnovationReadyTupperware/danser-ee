package common

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/audio"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

const barsPerSegment = 4
const nightCoreDivisor = 2

type NightcoreProcessor struct {
	*BeatSynced

	hasFirstValue bool
	firstValue    int

	pProgress int
}

func NewNightcoreProcessor() *NightcoreProcessor {
	proc := &NightcoreProcessor{
		BeatSynced: NewBeatSynced(),
	}

	proc.Divisor = nightCoreDivisor
	proc.pProgress = -1

	return proc
}

func (bs *NightcoreProcessor) Update(time float64) {
	bs.BeatSynced.Update(time)

	segLength := bs.timingPoint.Signature * nightCoreDivisor * barsPerSegment

	if !bs.IsSynced {
		bs.hasFirstValue = false
		bs.pProgress = -1

		return
	}

	if !bs.hasFirstValue || bs.beatIndex < bs.firstValue {
		bs.hasFirstValue = true

		if bs.beatIndex < 0 {
			bs.firstValue = 0
		} else {
			bs.firstValue = (bs.beatIndex/segLength + 1) * segLength
		}
	}

	if bs.beatIndex >= bs.firstValue {
		firstBeat := bs.firstValue
		if bs.pProgress >= firstBeat {
			firstBeat = bs.pProgress + 1
		}

		for beatIndex := firstBeat; beatIndex <= bs.beatIndex; beatIndex++ {
			bs.playBeat(beatIndex%segLength, bs.timingPoint.Signature, bs.beatTime(beatIndex))
		}
	}

	bs.pProgress = bs.beatIndex
}

func (bs *NightcoreProcessor) beatTime(beatIndex int) float64 {
	beatLength := bs.timingPoint.GetBaseBeatLength() / bs.Divisor
	if beatLength <= 0 || math.IsNaN(beatLength) || math.IsInf(beatLength, 0) {
		return bs.lastTime
	}

	index := beatIndex
	if bs.timingPoint.OmitFirstBarLine {
		index++
	}

	return bs.timingPoint.Time + float64(index)*beatLength
}

func (bs *NightcoreProcessor) playBeat(beatIndex int, signature int, eventTime float64) { //nolint:gocyclo
	if !settings.Audio.PlayNightcoreSamples {
		return
	}

	if beatIndex == 0 {
		audio.PlayNamedSampleAt("nightcore-finish", eventTime, 1)
	}

	switch signature {
	case 3:
		switch beatIndex % 6 {
		case 0:
			audio.PlayNamedSampleAt("nightcore-kick", eventTime, 1)
		case 3:
			audio.PlayNamedSampleAt("nightcore-clap", eventTime, 1)
		default:
			audio.PlayNamedSampleAt("nightcore-hat", eventTime, 1)
		}
	case 4:
		switch beatIndex % 4 {
		case 0:
			audio.PlayNamedSampleAt("nightcore-kick", eventTime, 1)
		case 2:
			audio.PlayNamedSampleAt("nightcore-clap", eventTime, 1)
		default:
			audio.PlayNamedSampleAt("nightcore-hat", eventTime, 1)
		}
	}
}
