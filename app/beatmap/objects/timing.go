package objects

import (
	"math"
	"sort"

	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

const (
	defaultTimingBeatLength = 60000.0 / 60.0
	minimumTimingBeatLength = 6
	maximumTimingBeatLength = 60000
)

type TimingPoint struct {
	Time float64

	beatLengthBase      float64
	beatLengthBaseLazer float64
	beatLength          float64

	SampleSet    int
	SampleIndex  int
	SampleVolume float64

	Signature int

	Inherited bool

	Kiai             bool
	OmitFirstBarLine bool
}

func (t TimingPoint) GetRatio() float64 {
	if t.beatLength >= 0 || math.IsNaN(t.beatLength) {
		return 1.0
	}

	return float64(float32(mutils.Clamp(-t.beatLength, 10, 1000)) / 100)
}

func (t TimingPoint) GetRatioLazer() float64 {
	if t.beatLength >= 0 || math.IsNaN(t.beatLength) {
		return 1.0
	}

	return 100 / (-t.beatLength)
}

func (t TimingPoint) GetBaseBeatLength() float64 {
	return t.beatLengthBase
}

func (t TimingPoint) GetBaseBPM() float64 {
	return 60000 / t.beatLengthBase
}

func (t TimingPoint) GetBeatLength() float64 {
	return t.beatLengthBase * t.GetRatio()
}

func (t TimingPoint) GetBeatLengthLazer() float64 {
	sliderVelocityAsBeatLength := -100 / t.GetRatioLazer()

	bpmMultiplier := 1.0
	if sliderVelocityAsBeatLength < 0 {
		bpmMultiplier = float64(mutils.Clamp(float32(-sliderVelocityAsBeatLength), 10, 1000) / 100)
	}

	return t.beatLengthBaseLazer * bpmMultiplier
}

type Timings struct {
	SliderMult float64
	TickRate   float64

	defaultTimingPoint TimingPoint

	points         []TimingPoint
	originalPoints []TimingPoint

	Current TimingPoint

	BaseSet int
}

func NewTimings() *Timings {
	return &Timings{
		defaultTimingPoint: TimingPoint{
			Time:                0,
			beatLengthBase:      defaultTimingBeatLength,
			beatLengthBaseLazer: defaultTimingBeatLength,
			beatLength:          defaultTimingBeatLength,
			SampleSet:           0,
			SampleIndex:         1,
			SampleVolume:        1,
			Signature:           4,
			Inherited:           false,
			Kiai:                false,
			OmitFirstBarLine:    false,
		},
		BaseSet: 1,
	}
}

func (tim *Timings) AddPoint(time, beatLength float64, sampleSet, sampleIndex int, sampleVolume float64, signature int, inherited, kiai, omitFirstBarLine bool) {
	baseBeatLength := beatLength
	// osu!lazer stores timing-point beat lengths in a bindable constrained to
	// 6..60000 ms. Keep the raw value in beatLength because its sign still
	// encodes the inherited slider-velocity multiplier, but normalize the base
	// value before inherited points copy it. This prevents denormal or
	// zero-like values in malformed maps from turning a long slider into a
	// zero-duration object.
	if !inherited && !math.IsNaN(beatLength) {
		baseBeatLength = max(float64(minimumTimingBeatLength), min(float64(maximumTimingBeatLength), beatLength))
	}

	point := TimingPoint{
		Time:                time,
		beatLengthBase:      baseBeatLength,
		beatLengthBaseLazer: baseBeatLength,
		beatLength:          beatLength,
		SampleSet:           sampleSet,
		SampleIndex:         sampleIndex,
		SampleVolume:        sampleVolume,
		Signature:           signature,
		Inherited:           inherited,
		Kiai:                kiai,
		OmitFirstBarLine:    omitFirstBarLine,
	}

	tim.points = append(tim.points, point)
}

func (tim *Timings) FinalizePoints() {
	sort.SliceStable(tim.points, func(i, j int) bool {
		return tim.points[i].Time < tim.points[j].Time
	})

	for _, point := range tim.points {
		if !point.Inherited {
			tim.originalPoints = append(tim.originalPoints, point)
		}
	}

	for i, point := range tim.points {
		if point.Inherited {
			if i > 0 {
				point.beatLengthBase = tim.points[i-1].beatLengthBase
			} else {
				// An inherited point without an earlier base point uses the
				// default timing point. This is both the stable fallback and
				// the timing-control-point default used by Lazer.
				point.beatLengthBase = tim.defaultTimingPoint.beatLengthBase
			}

			point.beatLengthBaseLazer = tim.GetOriginalPointAt(point.Time).beatLengthBaseLazer

			tim.points[i] = point
		}
	}
}

func (tim *Timings) Update(time float64) {
	tim.Current = tim.GetPointAt(time)
}

func (tim *Timings) GetDefault() TimingPoint {
	return tim.defaultTimingPoint
}

func (tim *Timings) GetPointAt(time float64) TimingPoint {
	if tim == nil {
		return TimingPoint{}
	}

	tLen := len(tim.points)
	if tLen == 0 {
		// A malformed or partially loaded beatmap can reach a spinner's audio
		// callback before timing points are available. The parser normally
		// rejects this state, but the default point is still the correct safe
		// fallback for callers that operate on an in-memory beatmap.
		return tim.defaultTimingPoint
	}

	index := sort.Search(tLen, func(i int) bool {
		return time < tim.points[i].Time
	})

	return tim.points[max(0, index-1)]
}

func (tim *Timings) GetOriginalPointAt(time float64) TimingPoint {
	if tim == nil {
		return TimingPoint{}
	}

	tLen := len(tim.originalPoints)

	if tLen == 0 {
		return tim.defaultTimingPoint
	}

	index := sort.Search(tLen, func(i int) bool {
		return time < tim.originalPoints[i].Time
	})

	return tim.originalPoints[max(0, index-1)]
}

func (tim *Timings) GetScoringDistance() float64 {
	return (100 * tim.SliderMult) / tim.TickRate
}

func (tim *Timings) GetSliderTimeP(point TimingPoint, pixelLength float64) float64 {
	return float64(float32(1000.0*pixelLength) / float32(100.0*tim.SliderMult*(1000.0/point.GetBeatLength())))
}

func (tim *Timings) GetVelocity(point TimingPoint) float64 {
	velocity := tim.GetScoringDistance() * tim.TickRate

	beatLength := point.GetBeatLength()

	if beatLength >= 0 {
		velocity *= 1000.0 / beatLength
	}

	return velocity
}

func (tim *Timings) GetTickDistance(point TimingPoint) float64 {
	return tim.GetScoringDistance() / point.GetRatio()
}

func (tim *Timings) HasPoints() bool {
	return len(tim.points) > 0
}

func (tim *Timings) Clear() {
	tim.originalPoints = tim.originalPoints[:0]
	tim.points = tim.points[:0]

	tim.Current = tim.defaultTimingPoint
}

func (tim *Timings) Reset() {
	if tim == nil {
		return
	}

	if len(tim.points) == 0 {
		tim.Current = tim.defaultTimingPoint
		return
	}

	tim.Current = tim.points[0]
}
