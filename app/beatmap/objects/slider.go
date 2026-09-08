package objects

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/go-gl/mathgl/mgl32"

	"github.com/innovationreadytupperware/danser-ee/app/audio"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/graphics/sliderrenderer"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/app/skin"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/sprite"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation/easing"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/curves"
	"github.com/innovationreadytupperware/danser-ee/framework/math/math32"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const (
	// These are parser safety ceilings. They are deliberately separate from
	// SliderWorkloadClass: a valid slider can be expensive without being
	// malformed, and workload classification must never shorten it.
	parserMaxPathLength         = 100_000_000
	parserMaxRepeats            = 10_000
	parserMaxCurveControlPoints = 4096
	// This is a last-resort derived-event safety budget. It is intentionally
	// far above ordinary maps and does not cap authored curve geometry or the
	// movement path used by cursor dance.
	maxDerivedSliderPoints = 1_000_000
)

type PathLine struct {
	Time1 int64
	Time2 int64
	Line  curves.Linear
}

type TickPoint struct {
	Time      float64
	Pos       vector.Vector2f
	fade      *animation.Glider
	scale     *animation.Glider
	IsReverse bool
	LastPoint bool
	EdgeIndex int
}

type Slider struct {
	*HitObject

	multiCurve  *curves.MultiCurve
	scorePath   []PathLine
	Timings     *Timings
	TPoint      TimingPoint
	pixelLength float64
	partLen     float64
	RepeatCount int

	sampleSets    []int
	additionSets  []int
	samples       []int
	sampleVolumes []float64
	baseSample    int
	audioLoops    audio.SliderLoopState

	Pos         vector.Vector2f
	TickPoints  []TickPoint
	TickReverse []TickPoint
	ScorePoints []TickPoint

	// TickPointsLazer holds the drawable Lazer tick dots. Lazer generates
	// ticks from the same fractional event timeline as ScorePointsLazer
	// (SliderEventGenerator.Generate) but repeat and tail points keep their
	// own endpoint visuals, so ticks are tracked separately like the Stable
	// TickPoints list.
	TickPointsLazer []TickPoint

	startCircle *Circle

	fade     *animation.Glider
	bodyFade *animation.Glider

	diff     *difficulty.Difficulty
	body     *sliderrenderer.Body
	lastTime float64

	ball     *sprite.Animation
	follower *sprite.Animation

	edges          []*Circle
	endCircles     []*Circle
	headEndCircles []*Circle
	tailEndCircles []*Circle

	isSliding bool

	EndTimeLazer     float64
	ScorePointsLazer []TickPoint
	spanDuration     float64

	updatedAtLeastOnce bool

	lastScorePoint int

	timingComputed     bool
	stablePathComputed bool

	workloadClassComputed bool
	workloadClass         SliderWorkloadClass
	movementFallback      bool
}

func NewSlider(data []string) *Slider {
	if len(data) < 8 {
		return nil
	}

	startX, errX := strconv.ParseFloat(data[0], 32)
	startY, errY := strconv.ParseFloat(data[1], 32)
	startTime, errTime := strconv.ParseFloat(data[2], 64)
	if errX != nil || errY != nil || errTime != nil ||
		math.IsNaN(startX) || math.IsInf(startX, 0) ||
		math.IsNaN(startY) || math.IsInf(startY, 0) ||
		math.IsNaN(startTime) || math.IsInf(startTime, 0) {
		return nil
	}

	slider := &Slider{
		HitObject: commonParse(data, 10),
	}

	slider.PositionDelegate = slider.PositionAt

	var err error
	slider.pixelLength, err = strconv.ParseFloat(data[7], 64)
	if err != nil || math.IsNaN(slider.pixelLength) || math.IsInf(slider.pixelLength, 0) || slider.pixelLength < 0 {
		return nil
	}

	slider.RepeatCount, err = strconv.Atoi(data[6])
	if err != nil {
		return nil
	}
	slider.RepeatCount = max(1, slider.RepeatCount)

	if slider.pixelLength > float64(parserMaxPathLength*10)/float64(slider.RepeatCount) {
		return nil
	}

	slider.pixelLength = min(slider.pixelLength, parserMaxPathLength)
	slider.RepeatCount = min(slider.RepeatCount, parserMaxRepeats)

	slider.multiCurve = slider.parseCurve(data[5])
	if slider.multiCurve == nil {
		return nil
	}

	if slider.pixelLength == 0 {
		derivedLength := float64(slider.multiCurve.GetLength())
		if !finiteSliderValue(derivedLength) || derivedLength < 0 || derivedLength > parserMaxPathLength {
			return nil
		}
		slider.pixelLength = derivedLength
	}

	if !finiteSliderValue(float64(slider.multiCurve.GetLength())) || !finiteSliderValue(slider.multiCurve.GetLengthLazer()) {
		return nil
	}
	for _, line := range slider.multiCurve.GetLines() {
		if !finiteSliderCoordinate(line.Point1.X) || !finiteSliderCoordinate(line.Point1.Y) ||
			!finiteSliderCoordinate(line.Point2.X) || !finiteSliderCoordinate(line.Point2.Y) ||
			!finiteSliderValue(float64(line.GetLength())) {
			return nil
		}
	}
	slider.updateWorkloadClass()

	slider.EndTime = slider.StartTime
	slider.EndTimeLazer = slider.StartTime
	slider.EndPosRaw = slider.multiCurve.PointAt(1.0)
	slider.Pos = slider.StartPosRaw

	slider.samples = make([]int, slider.RepeatCount+1)
	slider.sampleSets = make([]int, slider.RepeatCount+1)
	slider.additionSets = make([]int, slider.RepeatCount+1)
	slider.sampleVolumes = make([]float64, slider.RepeatCount+1)

	f, _ := strconv.ParseInt(data[4], 10, 64)
	slider.baseSample = int(f)

	for i := range slider.samples {
		slider.samples[i] = slider.baseSample
		slider.sampleSets[i] = slider.BasicHitSound.SampleSet
		slider.additionSets[i] = slider.BasicHitSound.AdditionSet
	}

	if len(data) > 8 {
		subData := strings.Split(data[8], "|")

		n := min(len(subData), len(slider.samples))

		for i := range n {
			sample, _ := strconv.Atoi(subData[i])
			slider.samples[i] = sample
		}
	}

	if len(data) > 9 {
		subData := strings.Split(data[9], "|")

		n := min(len(subData), len(slider.sampleSets))

		for i := range n {
			extras := strings.Split(subData[i], ":")

			if len(extras) == 0 {
				continue
			}

			sampleSet, _ := strconv.Atoi(extras[0])
			additionSet := 0
			if len(extras) > 1 {
				additionSet, _ = strconv.Atoi(extras[1])
			}

			slider.sampleSets[i] = sampleSet
			slider.additionSets[i] = additionSet

			if len(extras) > 3 {
				volume, err := strconv.ParseFloat(extras[3], 64)
				if err == nil {
					slider.sampleVolumes[i] = volume / 100
				}
			}
		}
	}

	slider.fade = animation.NewGlider(1)
	slider.bodyFade = animation.NewGlider(1)
	return slider
}

func (slider *Slider) parseCurve(curveData string) *curves.MultiCurve {
	if strings.Count(curveData, "|")+1 > parserMaxCurveControlPoints*2 {
		return nil
	}

	list := strings.Split(curveData, "|")

	var defs []curves.CurveDef

	cDef := curves.CurveDef{
		CurveType: curves.CType(-1),
		Points:    []vector.Vector2f{slider.StartPosRaw},
	}

	nextType := curves.CType(-1)
	controlPointCount := 1

	for i, j := 0, 0; i < len(list); i++ {
		split := strings.Split(list[i], ":")

		if len(split) == 1 {
			if tType := tryGetType(split[0]); tType > -1 {
				if cDef.CurveType == -1 {
					cDef.CurveType = tType
				} else {
					nextType = tType
				}
			}
		} else {
			if len(split) != 2 || len(cDef.Points) >= parserMaxCurveControlPoints || controlPointCount >= parserMaxCurveControlPoints {
				return nil
			}

			x, errX := strconv.ParseFloat(split[0], 64)
			y, errY := strconv.ParseFloat(split[1], 64)
			if errX != nil || errY != nil || math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
				return nil
			}

			fx, fy := float32(x), float32(y)
			if math.IsNaN(float64(fx)) || math.IsInf(float64(fx), 0) || math.IsNaN(float64(fy)) || math.IsInf(float64(fy), 0) {
				return nil
			}

			vec := vector.NewVec2f(fx, fy)

			if j > 0 || vec != slider.StartPosRaw { // skip the first point if it's the same as start position.
				cDef.Points = append(cDef.Points, vec)
			}
			controlPointCount++

			j++

			if nextType > -1 {
				defs = append(defs, cDef)

				cDef = curves.CurveDef{
					CurveType: nextType,
					Points:    []vector.Vector2f{vec},
				}

				nextType = -1
			}
		}
	}

	if len(cDef.Points) > 1 || len(defs) == 0 { // Lazer's multi-type slider has 1 point line
		if cDef.CurveType == -1 { // osu! uses catmull if there's no curve type
			cDef.CurveType = curves.CCatmull
		}

		defs = append(defs, cDef)
	}

	// validation
	for _, def := range defs {
		if def.CurveType == curves.CBezier {
			var controlDistance float64

			for i := 1; i < len(def.Points); i++ {
				controlDistance += float64(def.Points[i].Dst(def.Points[i-1]))
			}

			if controlDistance >= 2*parserMaxPathLength {
				return nil
			}
		}
	}

	return curves.NewMultiCurveT(defs, slider.pixelLength)
}

func tryGetType(str string) curves.CType {
	switch str {
	case "P":
		return curves.CCirArc
	case "L":
		return curves.CLine
	case "B":
		return curves.CBezier
	case "C":
		return curves.CCatmull
	default: // It's a point
		return -1
	}
}

func (slider *Slider) GetLength() float32 {
	return slider.multiCurve.GetLength()
}

func (slider *Slider) GetStartAngleMod(diff *difficulty.Difficulty) float32 {
	sampleTime := slider.StartTime + min(10.0, slider.partDurationForDiff(diff))
	return slider.GetStackedStartPositionMod(diff).AngleRV(slider.GetStackedPositionAtModForDiff(sampleTime, diff)) // temporary solution
}

func (slider *Slider) GetEndAngleMod(diff *difficulty.Difficulty) float32 {
	sampleTime := slider.endTimeForDiff(diff) - min(10.0, slider.partDurationForDiff(diff))
	return slider.GetStackedEndPositionModForDiff(diff).AngleRV(slider.GetStackedPositionAtModForDiff(sampleTime, diff)) // temporary solution
}

func (slider *Slider) GetPartLen() float32 {
	return float32(20.0) / float32(slider.Timings.GetSliderTimeP(slider.TPoint, slider.pixelLength)) * float32(slider.pixelLength)
}

func (slider *Slider) PositionAt(time float64) vector.Vector2f {
	if !slider.hasUsableStableTraversal() {
		return slider.StartPosRaw
	}
	if !finiteSliderValue(time) {
		time = slider.StartTime
	}

	index := sort.Search(len(slider.scorePath), func(i int) bool {
		return float64(slider.scorePath[i].Time2) >= time
	})

	pLine := slider.scorePath[mutils.Clamp(index, 0, len(slider.scorePath)-1)]

	clamped := mutils.Clamp(time, float64(pLine.Time1), float64(pLine.Time2))

	var pos vector.Vector2f
	if pLine.Time2 == pLine.Time1 {
		pos = pLine.Line.Point2
	} else {
		pos = pLine.Line.PointAt(float32(clamped-float64(pLine.Time1)) / float32(pLine.Time2-pLine.Time1))
	}

	return pos
}

func (slider *Slider) PositionAtLazer(time float64) vector.Vector2f {
	if slider.multiCurve == nil || slider.EndTimeLazer <= slider.StartTime ||
		math.IsNaN(slider.EndTimeLazer) || math.IsInf(slider.EndTimeLazer, 0) ||
		slider.spanDuration <= 0 || math.IsNaN(slider.spanDuration) || math.IsInf(slider.spanDuration, 0) {
		return slider.StartPosRaw
	}
	if math.IsNaN(time) || math.IsInf(time, 0) {
		time = slider.StartTime
	}

	t1 := mutils.Clamp(time, slider.StartTime, slider.EndTimeLazer)

	progress := (t1 - slider.StartTime) / slider.spanDuration

	progress = math.Mod(progress, 2)
	if progress >= 1 {
		progress = 2 - progress
	}

	return slider.multiCurve.PointAtLazer(progress)
}

// PositionAtForDiff selects the slider traversal that belongs to diff. The
// generated cursor and the ruleset must sample the same path when a map has
// different Stable and Lazer timing/path data.
func (slider *Slider) PositionAtForDiff(time float64, diff *difficulty.Difficulty) vector.Vector2f {
	if diff != nil && diff.IsLazer() {
		return slider.PositionAtLazer(time)
	}

	return slider.PositionAt(time)
}

func (slider *Slider) GetStackedPositionAtModLazer(time float64, diff *difficulty.Difficulty) vector.Vector2f {
	return ModifyPosition(slider.HitObject, slider.PositionAtLazer(time), diff)
}

// visualTickPoints selects the tick dots drawn for the participant's gameplay
// provenance: the fractional Lazer timeline for Lazer participants, the
// floored Stable timeline otherwise. Tick placement is identical with and
// without the Classic mod in the reference client. The workload policy
// governs movement expansion and detail audio, not tick dots.
func (slider *Slider) visualTickPoints() []TickPoint {
	if slider.diff != nil && slider.diff.IsLazer() {
		return slider.TickPointsLazer
	}

	return slider.TickPoints
}

// GetStackedPositionAtModForDiff returns the slider position for the
// participant's gameplay provenance after applying its stack and mirror
// modifiers.
func (slider *Slider) GetStackedPositionAtModForDiff(time float64, diff *difficulty.Difficulty) vector.Vector2f {
	return ModifyPosition(slider.HitObject, slider.PositionAtForDiff(time, diff), diff)
}

// GetStackedEndPositionModForDiff returns the endpoint from the traversal and
// timeline that belong to diff. The interface's stored endpoint is Stable's,
// which is not sufficient for repeated sliders under Lazer timing.
func (slider *Slider) GetStackedEndPositionModForDiff(diff *difficulty.Difficulty) vector.Vector2f {
	return ModifyPosition(slider.HitObject, slider.PositionAtForDiff(slider.endTimeForDiff(diff), diff), diff)
}

// GetStackedPositionAtModForDiff selects Lazer slider traversal without
// requiring callers that operate on generic hit objects to know about Slider.
func GetStackedPositionAtModForDiff(hitObject IHitObject, time float64, diff *difficulty.Difficulty) vector.Vector2f {
	if slider, ok := hitObject.(*Slider); ok {
		return slider.GetStackedPositionAtModForDiff(time, diff)
	}

	return hitObject.GetStackedPositionAtMod(time, diff)
}

// GetStackedEndPositionModForDiff selects a provenance-aware endpoint for a
// generic hit object.
func GetStackedEndPositionModForDiff(hitObject IHitObject, diff *difficulty.Difficulty) vector.Vector2f {
	if slider, ok := hitObject.(*Slider); ok {
		return slider.GetStackedEndPositionModForDiff(diff)
	}

	return hitObject.GetStackedEndPositionMod(diff)
}

// GetEndTimeForDiff returns the participant-specific end time for objects
// whose gameplay duration differs between Stable and Lazer.
func GetEndTimeForDiff(hitObject IHitObject, diff *difficulty.Difficulty) float64 {
	if slider, ok := hitObject.(*Slider); ok {
		return slider.endTimeForDiff(diff)
	}

	return hitObject.GetEndTime()
}

func (slider *Slider) GetAsDummyCircles() []IHitObject {
	return slider.GetAsDummyCirclesForDiff(slider.diff)
}

// GetAsDummyCirclesForDiff expands a slider using the score-point timeline
// associated with diff. Pathological and singular sliders intentionally keep
// their bounded one-point fallback.
func (slider *Slider) GetAsDummyCirclesForDiff(diff *difficulty.Difficulty) []IHitObject {
	circles := []IHitObject{slider.createDummyCircle(slider.GetStartTime(), true, false, diff)}

	if slider.IsPathological() || slider.IsSingular() {
		return circles
	}

	points := slider.ScorePoints
	isLazer := diff != nil && diff.IsLazer()
	if isLazer && len(slider.ScorePointsLazer) > 0 {
		points = slider.ScorePointsLazer
	}

	for i, p := range points {
		time := p.Time
		if i == len(points)-1 && settings.KNOCKOUT && !isLazer { // Lazer ends work differently so skip -36ms
			time = math.Floor(max(slider.StartTime+(slider.EndTime-slider.StartTime)/2, slider.EndTime-36))
		}

		circles = append(circles, slider.createDummyCircle(time, false, i == len(points)-1, diff))
	}

	return circles
}

func (slider *Slider) createDummyCircle(time float64, inheritStart, inheritEnd bool, diff *difficulty.Difficulty) *Circle {
	// Slider dance points inherit the source slider's stack map. Build that
	// representation directly instead of creating DummyCircleInherit's throwaway
	// map and immediately replacing it; dense Aspire sliders can otherwise make
	// thousands of avoidable map allocations during queue expansion.
	pos := slider.PositionAtForDiff(time, diff)
	circle := &Circle{HitObject: &HitObject{
		StartPosRaw:   pos,
		EndPosRaw:     pos,
		StartTime:     time,
		EndTime:       time,
		StackLeniency: slider.StackLeniency,
		StackIndexMap: slider.StackIndexMap,
		ComboSet:      slider.ComboSet,
	}}
	circle.SliderPoint = true
	circle.SliderPointStart = inheritStart
	circle.SliderPointEnd = inheritEnd
	circle.silent = true
	circle.textureName = "sliderstart"

	return circle
}

func (slider *Slider) SetTiming(timings *Timings, beatmapVersion int, diffCalcOnly bool) {
	slider.resetTimingData()
	slider.Timings = timings
	slider.timingComputed = timings != nil
	if timings == nil {
		slider.stablePathComputed = true
		slider.updateWorkloadClass()
		return
	}
	slider.TPoint = timings.GetPointAt(slider.StartTime)

	slider.calculateFollowPointsLazer(beatmapVersion)

	if diffCalcOnly { // We're not interested in stable-like path in difficulty calculator mode
		slider.updateWorkloadClass()
		return
	}

	slider.calculateFollowPointsStable(beatmapVersion)
	slider.updateWorkloadClass()
}

func (slider *Slider) resetTimingData() {
	slider.EndTime = slider.StartTime
	slider.EndTimeLazer = slider.StartTime
	slider.TPoint = TimingPoint{}
	slider.partLen = 0
	slider.spanDuration = 0
	slider.scorePath = slider.scorePath[:0]
	slider.TickPoints = slider.TickPoints[:0]
	slider.TickReverse = slider.TickReverse[:0]
	slider.ScorePoints = slider.ScorePoints[:0]
	slider.TickPointsLazer = slider.TickPointsLazer[:0]
	slider.ScorePointsLazer = slider.ScorePointsLazer[:0]
	slider.timingComputed = false
	slider.stablePathComputed = false
	slider.workloadClassComputed = false
	slider.movementFallback = false
}

func (slider *Slider) calculateFollowPointsLazer(beatmapVersion int) {
	const maxLzLength = 100000

	if slider.Timings == nil || slider.multiCurve == nil || slider.RepeatCount <= 0 {
		return
	}

	nanTimingPoint := math.IsNaN(slider.TPoint.beatLength)

	cLength := slider.multiCurve.GetLengthLazer()
	if !finiteSliderValue(cLength) || cLength <= 0 {
		return
	}

	velocity := 100 * slider.Timings.SliderMult / slider.TPoint.GetBeatLengthLazer()
	if !finiteSliderValue(velocity) || velocity <= 0 {
		return
	}

	scoringDistance := velocity * slider.TPoint.GetBaseBeatLength()

	tickDistanceMultiplier := 1.0
	if beatmapVersion < 8 {
		tickDistanceMultiplier = 1.0 / slider.TPoint.GetRatioLazer()
	}

	tickDistance := scoringDistance / slider.Timings.TickRate * tickDistanceMultiplier
	validTickDistance := finiteSliderValue(tickDistance) && tickDistance > 0

	endTime := slider.StartTime + float64(slider.RepeatCount)*cLength/velocity
	if !finiteSliderValue(endTime) || endTime <= slider.StartTime {
		return
	}

	slider.EndTimeLazer = endTime

	slider.spanDuration = (slider.EndTimeLazer - slider.StartTime) / float64(slider.RepeatCount)
	if !finiteSliderValue(slider.spanDuration) || slider.spanDuration <= 0 {
		slider.EndTimeLazer = slider.StartTime
		slider.spanDuration = 0
		return
	}

	length := min(maxLzLength, cLength)

	if validTickDistance {
		tickDistance = mutils.Clamp(tickDistance, 0, length)
		validTickDistance = tickDistance > 0
	}

	minDistanceFromEnd := velocity * 10

	// Lazer like score point calculations. Clean AF, but not unreliable enough for stable's replay processing. Would need more testing.
	edgeIndex := 1
	maxTickPoints := max(0, maxDerivedSliderPoints-slider.RepeatCount)
	generatedTickPoints := 0
	for span := 0; span < int(slider.RepeatCount); span++ {
		spanStartTime := slider.StartTime + float64(span)*slider.spanDuration
		reversed := span%2 == 1

		// Skip ticks if timingPoint has NaN beatLength
		for d := tickDistance; d <= length && !nanTimingPoint && validTickDistance && generatedTickPoints < maxTickPoints; d += tickDistance {
			if d >= length-minDistanceFromEnd {
				break
			}

			// Always generate ticks from the start of the path rather than the span to ensure that ticks in repeat spans are positioned identically to those in non-repeat spans
			timeProgress := d / length
			if reversed {
				timeProgress = 1 - timeProgress
			}

			tickTime := spanStartTime + timeProgress*slider.spanDuration
			slider.TickPointsLazer = append(slider.TickPointsLazer, TickPoint{
				Time:      tickTime,
				Pos:       slider.PositionAtLazer(tickTime),
				EdgeIndex: -1,
			})
			slider.ScorePointsLazer = append(slider.ScorePointsLazer, TickPoint{
				Time: tickTime,
			})
			generatedTickPoints++
		}

		slider.ScorePointsLazer = append(slider.ScorePointsLazer, TickPoint{
			Time:      spanStartTime + slider.spanDuration,
			IsReverse: span < int(slider.RepeatCount)-1,
			LastPoint: span == int(slider.RepeatCount)-1,
			EdgeIndex: edgeIndex,
		})
		edgeIndex++
	}

	slices.SortFunc(slider.ScorePointsLazer, func(a, b TickPoint) int { return cmp.Compare(a.Time, b.Time) })
}

func (slider *Slider) calculateFollowPointsStable(beatmapVersion int) {
	slider.stablePathComputed = true

	if slider.Timings == nil || slider.multiCurve == nil {
		return
	}

	nanTimingPoint := math.IsNaN(slider.TPoint.beatLength)

	lines := slider.multiCurve.GetLines()

	startTime := slider.StartTime

	velocity := slider.Timings.GetVelocity(slider.TPoint)
	if !finiteSliderValue(velocity) || velocity <= 0 {
		return
	}

	cLength := float64(slider.multiCurve.GetLength())
	if !finiteSliderValue(cLength) || cLength <= 0 {
		return
	}

	minDistanceFromEnd := velocity * 0.01

	tickDistance := slider.Timings.GetTickDistance(slider.TPoint)
	validTickDistance := finiteSliderValue(tickDistance) && tickDistance > 0
	if beatmapVersion < 8 {
		tickDistance = slider.Timings.GetScoringDistance()
		validTickDistance = finiteSliderValue(tickDistance) && tickDistance > 0
	}

	if validTickDistance && slider.multiCurve.GetLength() > 0 && tickDistance > slider.pixelLength {
		tickDistance = slider.pixelLength
		validTickDistance = tickDistance > 0
	}

	// Sanity limit to 32768 ticks per repeat
	if validTickDistance && cLength/tickDistance > 32768 {
		tickDistance = cLength / 32768
	}

	scoringLengthTotal := 0.0
	scoringDistance := 0.0
	maxTickPoints := max(0, maxDerivedSliderPoints-slider.RepeatCount)
	generatedTickPoints := 0

	// Stable-like score point processing, ugly AF.
	for i := range slider.RepeatCount {
		distanceToEnd := float64(slider.multiCurve.GetLength())
		skipTick := nanTimingPoint // NaN SV acts like 1.0x SV, but doesn't spawn slider ticks

		reverse := (i % 2) == 1

		start := 0
		end := len(lines)
		direction := 1

		if reverse {
			start = len(lines) - 1
			end = -1
			direction = -1
		}

		for j := start; j != end; j += direction {
			line := lines[j]

			p1, p2 := line.Point1, line.Point2

			if reverse {
				p1, p2 = p2, p1
			}

			distance := float32(line.GetCustomLength())

			progress := 1000.0 * float64(distance) / velocity
			if !finiteSliderValue(progress) {
				return
			}

			slider.scorePath = append(slider.scorePath, PathLine{Time1: int64(startTime), Time2: int64(startTime + progress), Line: curves.NewLinear(p1, p2)})

			startTime += progress
			slider.EndTime = math.Floor(startTime)

			scoringDistance += float64(distance)

			for scoringDistance >= tickDistance && !skipTick && validTickDistance && generatedTickPoints < maxTickPoints {
				scoringLengthTotal += tickDistance
				scoringDistance -= tickDistance
				distanceToEnd -= tickDistance

				skipTick = distanceToEnd <= minDistanceFromEnd
				if skipTick {
					break
				}

				scoreTime := slider.StartTime + math.Floor(float64(float32(scoringLengthTotal))/velocity*1000)

				var fade, scale *animation.Glider
				if !slider.IsPathological() {
					fade = animation.NewGlider(0.0)
					scale = animation.NewGlider(0.0)
				}
				point := TickPoint{scoreTime, slider.GetPositionAt(scoreTime), fade, scale, false, false, -1}
				slider.TickPoints = append(slider.TickPoints, point)
				slider.ScorePoints = append(slider.ScorePoints, point)
				generatedTickPoints++
			}
		}

		if !validTickDistance || generatedTickPoints >= maxTickPoints {
			scoreTime := slider.StartTime + math.Floor(float64(i+1)*cLength/velocity*1000)
			if i == slider.RepeatCount-1 {
				scoreTime = slider.EndTime
			}

			point := TickPoint{scoreTime, slider.GetPositionAt(scoreTime), nil, nil, true, (i + 1) == slider.RepeatCount, i + 1}
			slider.TickReverse = append(slider.TickReverse, point)
			slider.ScorePoints = append(slider.ScorePoints, point)
			continue
		}

		scoringLengthTotal += scoringDistance

		scoreTime := slider.StartTime + math.Floor((float64(float32(scoringLengthTotal))/velocity)*1000)

		// Ensure last tick is not later than end time. Ruleset calculates the last tick regardless of this value
		if i == slider.RepeatCount-1 {
			scoreTime = slider.EndTime
		}

		point := TickPoint{scoreTime, slider.GetPositionAt(scoreTime), nil, nil, true, (i + 1) == slider.RepeatCount, i + 1}

		slider.TickReverse = append(slider.TickReverse, point)
		slider.ScorePoints = append(slider.ScorePoints, point)

		if skipTick {
			scoringDistance = 0
		} else {
			scoringLengthTotal -= tickDistance - scoringDistance
			scoringDistance = tickDistance - scoringDistance
		}
	}

	slider.partLen = (slider.EndTime - slider.StartTime) / float64(slider.RepeatCount)

	slider.EndPosRaw = slider.GetPositionAt(slider.EndTime)

	//if len(slider.scorePath) == 0 || slider.StartTime == slider.EndTime {
	//	log.Println("Warning: slider", slider.HitObjectID, "at ", slider.StartTime, "is broken.")
	//}

	slices.SortFunc(slider.TickPoints, func(a, b TickPoint) int { return cmp.Compare(a.Time, b.Time) })
	slices.SortFunc(slider.ScorePoints, func(a, b TickPoint) int { return cmp.Compare(a.Time, b.Time) })
}

func copySliderHOData(target, base *HitObject) {
	target.ComboNumber = base.ComboNumber
	target.ComboSet = base.ComboSet
	target.ComboSetHax = base.ComboSetHax
	target.HitObjectID = base.HitObjectID
	target.StackLeniency = base.StackLeniency
	target.StackIndexMap = base.StackIndexMap
}

// visualEndTime is the timeline used by the renderer. Lazer keeps the
// fractional slider duration while the Stable-compatible HitObject duration
// is floored for replay and judgement code.
func (slider *Slider) visualEndTime() float64 {
	if slider.EndTimeLazer > slider.StartTime && !math.IsNaN(slider.EndTimeLazer) && !math.IsInf(slider.EndTimeLazer, 0) {
		return slider.EndTimeLazer
	}

	return slider.EndTime
}

func (slider *Slider) endTimeForDiff(diff *difficulty.Difficulty) float64 {
	if diff != nil && diff.IsLazer() {
		return slider.visualEndTime()
	}

	return slider.EndTime
}

func (slider *Slider) partDurationForDiff(diff *difficulty.Difficulty) float64 {
	if diff != nil && diff.IsLazer() {
		return slider.visualSpanDuration()
	}

	return float64(slider.partLen)
}

// visualSpanDuration returns the duration of one Lazer span and falls back to
// the Stable value for manually constructed sliders and malformed maps.
func (slider *Slider) visualSpanDuration() float64 {
	if slider.spanDuration > 0 && !math.IsNaN(slider.spanDuration) && !math.IsInf(slider.spanDuration, 0) {
		return slider.spanDuration
	}

	if slider.RepeatCount > 0 {
		duration := (slider.visualEndTime() - slider.StartTime) / float64(slider.RepeatCount)
		if duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) {
			return duration
		}
	}

	return 0
}

func (slider *Slider) positionAtEndpoint(time float64) vector.Vector2f {
	if slider.diff != nil && slider.diff.IsLazer() {
		return slider.PositionAtLazer(time)
	}

	return slider.PositionAt(time)
}

// updateEndpointPositions follows the current body only until an endpoint is
// judged. Final tails always use their path endpoint, and successful repeats
// remain at the position captured by setSliderPosition's latch.
func (slider *Slider) updateEndpointPositions(time float64) {
	if slider.diff == nil || slider.multiCurve == nil {
		return
	}

	bodyRange := slider.visualBodyRange(time)
	headPos := slider.multiCurve.PointAt(float32(bodyRange.head))
	tailPos := slider.multiCurve.PointAt(float32(bodyRange.tail))
	headAngle := slider.multiCurve.GetStartAngleAt(float32(bodyRange.head)) + math.Pi
	tailAngle := slider.multiCurve.GetEndAngleAt(float32(bodyRange.tail)) + math.Pi

	mS, mOk := difficulty.GetModConfig[difficulty.MirrorSettings](slider.diff)

	vFlip := slider.diff.CheckModActive(difficulty.HardRock) != (mOk && (mS.FlipMode+1)&2 == 2)
	hFlip := mOk && (mS.FlipMode+1)&1 == 1

	if vFlip {
		headAngle = -headAngle
		tailAngle = -tailAngle
	}

	if hFlip {
		headAngle = mutils.Signum(headAngle)*math.Pi - headAngle
		tailAngle = mutils.Signum(tailAngle)*math.Pi - tailAngle
	}

	for _, circle := range slider.headEndCircles {
		if circle.SliderPointEnd {
			circle.setSliderPosition(slider.positionAtEndpoint(circle.StartTime), 0)
		} else {
			circle.setSliderPosition(headPos, float64(headAngle))
		}
		circle.Update(time)
	}

	for _, circle := range slider.tailEndCircles {
		if circle.SliderPointEnd {
			circle.setSliderPosition(slider.positionAtEndpoint(circle.StartTime), 0)
		} else {
			circle.setSliderPosition(tailPos, float64(tailAngle))
		}
		circle.Update(time)
	}
}

func (slider *Slider) visualBodyRange(time float64) sliderBodyRange {
	if slider.diff == nil {
		return sliderBodyRange{}
	}

	return sliderBodyRangeAt(
		time,
		slider.StartTime,
		slider.visualEndTime(),
		slider.diff.Preempt,
		slider.visualSpanDuration(),
		slider.RepeatCount,
		sliderSnakeSettings{
			in:                 settings.Objects.Sliders.Snaking.In,
			out:                settings.Objects.Sliders.Snaking.Out,
			durationMultiplier: settings.Objects.Sliders.Snaking.DurationMultiplier,
			fadeMultiplier:     settings.Objects.Sliders.Snaking.FadeMultiplier,
		},
	)
}

func (slider *Slider) SetDifficulty(diff *difficulty.Difficulty) {
	slider.diff = diff
	slider.edges = slider.edges[:0]
	slider.endCircles = slider.endCircles[:0]
	slider.headEndCircles = slider.headEndCircles[:0]
	slider.tailEndCircles = slider.tailEndCircles[:0]
	slider.lastScorePoint = 0
	slider.lastTime = math.Inf(-1)
	slider.updatedAtLeastOnce = false
	slider.isSliding = false
	slider.Pos = slider.StartPosRaw

	visualEndTime := slider.visualEndTime()

	slider.fade = animation.NewGlider(0)
	slider.fade.AddEvent(slider.StartTime-diff.Preempt, slider.StartTime-(diff.Preempt-diff.TimeFadeIn), 1)

	slider.bodyFade = animation.NewGlider(0)
	slider.bodyFade.AddEvent(slider.StartTime-diff.Preempt, slider.StartTime-(diff.Preempt-diff.TimeFadeIn), 1)

	if diff.HasHiddenObjectFading() {
		slider.bodyFade.AddEventEase(slider.StartTime-diff.Preempt+diff.TimeFadeIn, visualEndTime, 0, easing.OutQuad)
	}

	slider.fade.AddEvent(visualEndTime, visualEndTime+difficulty.HitFadeOut, 0)

	if !diff.HasHiddenObjectFading() {
		fadeEnd := visualEndTime + difficulty.HitFadeOut
		if settings.Objects.Sliders.Snaking.Out {
			if settings.Objects.Sliders.Snaking.OutFadeInstant {
				fadeEnd = visualEndTime
			} else {
				// Lazer gives the slider body a short fade after a snaking-out
				// tail so the body does not disappear as a hard cut.
				fadeEnd = visualEndTime + 40
			}
		}

		slider.bodyFade.AddEvent(visualEndTime, fadeEnd, 0)
	}

	slider.startCircle = DummyCircle(slider.StartPosRaw, slider.StartTime)
	copySliderHOData(slider.startCircle.HitObject, slider.HitObject)
	slider.startCircle.SetDifficulty(diff)

	slider.edges = append(slider.edges, slider.startCircle)

	sixty := 1000.0 / 60
	velocity := slider.Timings.GetVelocity(slider.TPoint)
	frameDelay := sixty
	if velocity > 0 && !math.IsNaN(velocity) && !math.IsInf(velocity, 0) {
		frameDelay = max(150/velocity*sixty, sixty)
	}

	slider.ball = sprite.NewAnimation(skin.GetFrames("sliderb", false), frameDelay, true, 0.0, vector.NewVec2d(0, 0), vector.Centre)

	if len(slider.scorePath) > 0 {
		angle := slider.scorePath[0].Line.GetStartAngle()
		slider.ball.SetVFlip(angle > -math32.Pi/2 && angle < math32.Pi/2)
	}

	followerFrames := skin.GetFrames("sliderfollowcircle", true)

	slider.follower = sprite.NewAnimation(followerFrames, skin.GetInfo().GetFrameTime(max(1, len(followerFrames))), true, 0.0, vector.NewVec2d(0, 0), vector.Centre)
	slider.follower.SetAlpha(0.0)

	// Pathological and singular sliders use the normal hit-note presentation
	// for slider-dance expansion and slider-detail audio. Keep the authored
	// slider data for bounded direct cursor tracking, gameplay, and body
	// rendering, but avoid allocating endpoint objects that this presentation
	// policy will not animate.
	if !slider.IsPathological() && !slider.IsSingular() {
		spanDuration := slider.visualSpanDuration()
		for i := 1; i <= slider.RepeatCount; i++ {
			circleTime := slider.StartTime + spanDuration*float64(i)

			appearTime := slider.StartTime - slider.diff.Preempt
			bounceStartTime := slider.StartTime - min(slider.diff.Preempt, 15000.0)

			if i > 1 {
				appearTime = circleTime - spanDuration*2
				bounceStartTime = appearTime
			}

			circle := NewSliderEndCircle(vector.NewVec2f(0, 0), appearTime, bounceStartTime, circleTime, i == 1, i == slider.RepeatCount)
			copySliderHOData(circle.HitObject, slider.HitObject)
			circle.SetTiming(slider.Timings, 14, false)
			circle.SetDifficulty(diff)

			slider.endCircles = append(slider.endCircles, circle)
			slider.edges = append(slider.edges, circle)

			if i%2 == 0 {
				slider.headEndCircles = append(slider.headEndCircles, circle)
			} else {
				slider.tailEndCircles = append(slider.tailEndCircles, circle)
			}
		}
	}

	for i, p := range slider.TickPoints {
		p.Pos = slider.GetStackedPositionAtMod(p.Time, slider.diff)

		slider.TickPoints[i] = p
	}

	for i, p := range slider.TickPointsLazer {
		p.Pos = slider.GetStackedPositionAtModLazer(p.Time, slider.diff)

		slider.TickPointsLazer[i] = p
	}

	for i, p := range slider.TickReverse {
		p.Pos = slider.GetStackedPositionAtMod(p.Time, slider.diff)

		slider.TickReverse[i] = p
	}

	mS, mOk := difficulty.GetModConfig[difficulty.MirrorSettings](slider.diff)

	vFlip := slider.diff.CheckModActive(difficulty.HardRock) != (mOk && (mS.FlipMode+1)&2 == 2)
	hFlip := mOk && (mS.FlipMode+1)&1 == 1

	slider.body = sliderrenderer.NewBody(slider.multiCurve, vFlip, hFlip, float32(slider.diff.CircleRadius))
}

func (slider *Slider) Update(time float64) bool {
	if !slider.updatedAtLeastOnce {
		slider.initScorePointAnimations()
		slider.updatedAtLeastOnce = true
	}

	visualEndTime := slider.visualEndTime()

	if automaticSliderEdgesEnabled() {
		if slider.lastTime < slider.StartTime && time >= slider.StartTime {
			slider.HitEdge(0, time, true)
			slider.InitSlide(slider.StartTime)
		}

		if slider.lastTime < visualEndTime && time >= visualEndTime {
			slider.HitEdge(slider.RepeatCount, time, true)
		}
	}

	// Pathological sliders suppress generated slider-detail presentation even
	// when direct cursor tracking retains their authored path. The head hit
	// remains handled by HitEdge while gameplay still owns the nested judgement
	// data.
	if slider.isSliding && !slider.IsPathological() {
		points := slider.ScorePoints
		if slider.diff != nil && slider.diff.IsLazer() && len(slider.ScorePointsLazer) > 0 {
			points = slider.ScorePointsLazer
		}

		for i := slider.lastScorePoint; i < len(points)-1; i++ {
			p := points[i]

			if time < p.Time {
				break
			} else if slider.lastTime < p.Time {
				if p.IsReverse {
					slider.HitEdge(p.EdgeIndex, time, true)
				} else {
					slider.PlayTickAt(p.Time)
				}
			}

			slider.lastScorePoint = i + 1
		}
	}

	if slider.startCircle != nil {
		slider.startCircle.Update(time)
	}

	if slider.ball != nil {
		slider.ball.Update(time)
	}

	if slider.follower != nil {
		slider.follower.Update(time)
	}

	slider.fade.Update(time)
	slider.bodyFade.Update(time)

	slider.updateEndpointPositions(time)

	for _, p := range slider.visualTickPoints() {
		if p.fade != nil {
			p.fade.Update(time)
		}
		if p.scale != nil {
			p.scale.Update(time)
		}
	}

	// The slider ball follows the full Lazer path. Snaking changes the body
	// range, not the ball's gameplay position, so the ball does not jump when
	// the final span retracts.
	pos := slider.GetStackedPositionAtModLazer(time, slider.diff)
	if slider.ball != nil {
		slider.ball.SetPosition(pos.Copy64())
	}

	if time-slider.lastTime > 0 && time >= slider.StartTime {
		angle := pos.AngleRV(slider.Pos)

		reversed := false
		if spanDuration := slider.visualSpanDuration(); spanDuration > 0 {
			span := int(math.Floor((time - slider.StartTime) / spanDuration))
			span = max(0, min(span, slider.RepeatCount-1))
			reversed = span%2 == 1
		}

		if reversed {
			angle -= math32.Pi
		}

		if slider.ball != nil {
			slider.ball.SetHFlip(skin.GetInfo().SliderBallFlip && reversed)
			slider.ball.SetRotation(float64(angle))
		}
	}

	if slider.isSliding && time >= slider.StartTime && time <= visualEndTime {
		slider.PlaySlideSamples(time)
	}

	if slider.lastTime <= visualEndTime && time > visualEndTime && slider.isSliding {
		slider.StopSlide()
	}

	slider.Pos = pos

	slider.lastTime = time

	return true
}

// automaticObjectPresentationEnabled reports whether beatmap objects own
// presentation events that would otherwise be submitted by a one-player
// ruleset. Multi-participant playback has one shared object, so it remains
// object-owned; one-player ruleset paths must not submit the same event twice.
func automaticObjectPresentationEnabled() bool {
	return settings.PLAYERS > 1 || (!settings.PLAY && !settings.KNOCKOUT)
}

// automaticSliderEdgesEnabled reports whether the beatmap object owns shared
// slider edge presentation.
func automaticSliderEdgesEnabled() bool {
	return automaticObjectPresentationEnabled()
}

func (slider *Slider) ArmStart(clicked bool, time float64) {
	slider.startCircle.Arm(clicked, time)

	if slider.ball != nil {
		// The ball is hidden while the body is snaking out. Once the head is
		// judged, Lazer reveals it at the actual slider start.
		slider.ball.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, slider.StartTime, slider.StartTime, 1, 1))
	}
}

// initScorePointAnimations prepares the independent score-point visuals. The
// slider body itself is not initialized here because its snaking range is a
// function of the current frame and must remain seek-safe. The reference
// client draws tick dots on every valid slider regardless of path length.
func (slider *Slider) initScorePointAnimations() {
	slSnInS := slider.StartTime - slider.diff.Preempt
	slSnInE := slider.StartTime - slider.diff.Preempt*2/3*(1.0-clampSnakeMultiplier(settings.Objects.Sliders.Snaking.FadeMultiplier)) +
		slider.visualSpanDuration()*clampSnakeMultiplier(settings.Objects.Sliders.Snaking.DurationMultiplier)

	if slider.ball != nil && settings.Objects.Sliders.Snaking.Out {
		slider.ball.SetAlpha(0)
	}

	isLazer := slider.diff != nil && slider.diff.IsLazer()
	points := slider.visualTickPoints()

	for i, p := range points {
		var startTime, endTime float64

		partDuration := slider.partDurationForDiff(slider.diff)
		if partDuration <= 0 {
			partDuration = slider.visualSpanDuration()
		}
		repeatProgress := 0.0
		if partDuration > 0 {
			repeatProgress = (p.Time - slider.StartTime) / partDuration
		}

		if repeatProgress < 1.0 {
			normalStart := (p.Time-slider.StartTime)/2 + slider.StartTime - slider.diff.Preempt*2/3

			startTime = max(repeatProgress*(slSnInE-slSnInS)+slSnInS, normalStart)

			endTime = min(startTime+150, p.Time-36)
		} else {
			rStart := slider.StartTime + partDuration*math.Floor(repeatProgress)

			endTime = rStart + (p.Time-rStart)/2
			startTime = endTime - 200
		}

		if endTime < startTime {
			endTime = startTime
		}

		// Dots that arrive without gliders allocate them here, so every drawn
		// dot owns its fade and scale transforms.
		if p.scale == nil {
			p.scale = animation.NewGlider(0.0)
		}
		if p.fade == nil {
			p.fade = animation.NewGlider(0.0)
		}

		p.scale.AddEventS(startTime, endTime, 0.5, 1.2)
		p.scale.AddEventSEase(endTime, endTime+150, 1.2, 1.0, easing.OutQuad)
		p.fade.AddEventS(startTime, endTime, 0.0, 1.0)

		if slider.diff.HasHiddenObjectFading() {
			p.fade.AddEventS(max(endTime, p.Time-1000), p.Time, 1.0, 0.0)
		} else {
			p.fade.AddEventS(p.Time, p.Time, 1.0, 0.0)
		}

		if isLazer {
			p.Pos = slider.GetStackedPositionAtModLazer(p.Time, slider.diff)
		} else {
			p.Pos = slider.GetStackedPositionAtMod(p.Time, slider.diff)
		}

		points[i] = p
	}
}

// InitSlide starts the Lazer-style follow-circle press animation. Release is
// intentionally not animated: Lazer keeps the follow circle available until
// the next nested judgement reports a hit, end, or break.
func (slider *Slider) InitSlide(time float64) {
	if time > slider.visualEndTime() || slider.follower == nil {
		return
	}

	slider.follower.ClearTransformations()

	fadeInEnd := min(time+180, slider.visualEndTime())

	slider.follower.AddTransformUnordered(animation.NewSingleTransform(animation.Fade, easing.Linear, time, min(time+60, slider.visualEndTime()), 0, 1))
	slider.follower.AddTransformUnordered(animation.NewSingleTransform(animation.Scale, easing.OutQuad, time, fadeInEnd, 0.5, 1))

	slider.follower.SortTransformations()

	slider.isSliding = true
}

// StopSlide stops slider-loop samples after tracking is released. Lazer does
// not play a break animation merely because the cursor leaves the follow
// radius; the nested event that is actually missed owns that animation.
func (slider *Slider) StopSlide() {
	slider.isSliding = false
	slider.StopSlideSamples()
}

// AnimateSliderTick applies the legacy follow-circle tick pulse. The pulse is
// emitted by judged tick and repeat events, not pre-scheduled for every point,
// so misses cannot produce a false hit animation.
func (slider *Slider) AnimateSliderTick(time float64) {
	if slider.follower == nil || !settings.Objects.Sliders.HitAnimations || slider.follower.GetScale().X < 0.999 {
		return
	}

	currentScale := slider.follower.GetScale().X
	slider.follower.AddTransform(animation.NewSingleTransform(animation.Scale, easing.Linear, time, time, currentScale, 1.1))
	slider.follower.AddTransform(animation.NewSingleTransform(animation.Scale, easing.Linear, time, time+200, 1.1, currentScale))
}

// AnimateSliderPoint updates a repeat or tail circle and emits the matching
// follow-circle event. It is separate from PlayEdgeSample because Classic
// sample policy may request audio without replaying a visual animation.
func (slider *Slider) AnimateSliderPoint(index int, time float64, isHit bool) {
	if index <= 0 || index >= len(slider.edges) {
		return
	}

	slider.updateEndpointPositions(time)
	edges := slider.edges[index]
	edges.ArmSliderPoint(isHit, time, slider.visualSpanDuration())
	edges.Update(time)
	if !isHit {
		return
	}

	if index == slider.RepeatCount {
		slider.AnimateSliderEnd(time)
	} else {
		slider.AnimateSliderTick(time)
	}
}

// AnimateSliderEnd completes the follow-circle animation after a successful
// tail judgement. The body itself has an independent Lazer-compatible fade.
func (slider *Slider) AnimateSliderEnd(time float64) {
	if slider.follower == nil {
		slider.StopSlide()
		return
	}

	alpha := slider.follower.GetAlpha()
	if !settings.Objects.Sliders.HitAnimations {
		slider.follower.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, time, time+60, alpha, 0))
	} else {
		currentScale := slider.follower.GetScale().X
		slider.follower.AddTransform(animation.NewSingleTransform(animation.Scale, easing.OutQuad, time, time+200, currentScale, 0.8))
		slider.follower.AddTransform(animation.NewSingleTransform(animation.Fade, easing.InQuad, time, time+200, alpha, 0))
	}

	slider.StopSlide()
}

// AnimateSliderBreak plays the follow-circle break animation for a missed
// nested slider event. A release without a missed event deliberately does not
// call this method.
func (slider *Slider) AnimateSliderBreak(time float64) {
	if slider.follower == nil {
		slider.StopSlide()
		return
	}

	alpha := slider.follower.GetAlpha()
	slider.follower.ClearTransformations()
	if !settings.Objects.Sliders.HitAnimations {
		slider.follower.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, time, time+60, alpha, 0))
	} else {
		currentScale := max(1.0, slider.follower.GetScale().X)
		slider.follower.AddTransform(animation.NewSingleTransform(animation.Scale, easing.Linear, time, time+100, currentScale, 2))
		slider.follower.AddTransform(animation.NewSingleTransform(animation.Fade, easing.Linear, time, time+100, alpha, 0))
	}

	slider.StopSlide()
}

func (slider *Slider) PlaySlideSamples(time float64) {
	if slider.audioSubmissionDisabled || slider.IsPathological() || slider.IsSingular() {
		return
	}

	point := slider.Timings.GetPointAt(time)

	sampleSet := slider.BasicHitSound.SampleSet
	if sampleSet == 0 {
		sampleSet = point.SampleSet
	}

	audio.PlaySliderLoopsAt(&slider.audioLoops, time, sampleSet, slider.BasicHitSound.AdditionSet, slider.baseSample,
		point.SampleIndex, point.SampleVolume, slider.HitObjectID, slider.Pos.X64())
}

func (slider *Slider) StopSlideSamples() {
	// Cleanup must remain unconditional. The submission flag prevents new
	// sounds after a seek/skip, but an already-running loop still owns a
	// native channel and must be stopped when the slider leaves the screen.
	audio.StopSliderLoops(&slider.audioLoops)
}

func (slider *Slider) PlayEdgeSample(index int) {
	if slider.audioSubmissionDisabled {
		return
	}
	if index < 0 || index >= len(slider.sampleSets) || index >= len(slider.additionSets) || index >= len(slider.samples) {
		return
	}

	sampleSet := slider.sampleSets[index]
	if sampleSet == 0 && index == 0 {
		sampleSet = slider.BasicHitSound.SampleSet
	}

	eventTime := slider.StartTime + math.Floor(float64(index)*slider.partLen)
	edgeTime := eventTime + 5
	edgePosition := slider.GetStackedPositionAtMod(eventTime, slider.diff)
	if slider.diff != nil && slider.diff.IsLazer() {
		eventTime = slider.StartTime + float64(index)*slider.visualSpanDuration()
		edgeTime = eventTime + 5
		edgePosition = slider.GetStackedPositionAtModLazer(edgeTime-5, slider.diff)
	}

	customVolume := slider.sampleVolumes[index]
	if customVolume <= 0 && index == 0 {
		customVolume = slider.BasicHitSound.CustomVolume
	}

	slider.playSampleT(eventTime, sampleSet, slider.additionSets[index], slider.samples[index], slider.Timings.GetPointAt(edgeTime), customVolume, edgePosition)
}

func (slider *Slider) HitEdge(index int, time float64, isHit bool) {
	if index == 0 {
		slider.ArmStart(isHit, time)
	} else if index > 0 {
		slider.AnimateSliderPoint(index, time, isHit)
	}

	if isHit && (index == 0 || (!slider.IsPathological() && (index == slider.RepeatCount || !slider.IsSingular()))) {
		slider.PlayEdgeSample(index)
	}
}

func (slider *Slider) PlayTickAt(eventTime float64) {
	if slider.audioSubmissionDisabled || slider.IsPathological() {
		return
	}

	point := slider.Timings.GetPointAt(eventTime)
	position := slider.GetStackedPositionAtMod(eventTime, slider.diff)
	if slider.diff != nil && slider.diff.IsLazer() {
		position = slider.GetStackedPositionAtModLazer(eventTime, slider.diff)
	}

	audio.PlaySliderTickAt(eventTime, point.SampleSet, point.SampleIndex, point.SampleVolume, slider.HitObjectID, position.X64())
}

func (slider *Slider) playSampleT(eventTime float64, sampleSet, additionSet, sample int, point TimingPoint, customVolume float64, pos vector.Vector2f) {
	if sampleSet == 0 {
		sampleSet = point.SampleSet
	}

	if additionSet == 0 {
		additionSet = sampleSet
	}

	audio.PlaySampleAt(eventTime, sampleSet, additionSet, sample, point.SampleIndex, point.SampleVolume, customVolume, slider.HitObjectID, pos.X64())
}

func (slider *Slider) GetPosition() vector.Vector2f {
	return slider.Pos
}

func (slider *Slider) DrawBodyBase(time float64, projection mgl32.Mat4) {
	if slider.body == nil || slider.diff == nil {
		return
	}

	rangeAtTime := slider.visualBodyRange(time)
	// A collapsed range represents a fully retracted body. Avoid submitting
	// a cap-only draw for zero-duration sliders and the final frame of a
	// snaking-out slider.
	if rangeAtTime.head == rangeAtTime.tail {
		return
	}
	slider.body.DrawBase(rangeAtTime.head, rangeAtTime.tail, projection)
}

func (slider *Slider) DrawBody(time float64, circleColor, bodyColor, innerBorder, outerBorder color2.Color, projection mgl32.Mat4, scale float32) {
	// The beatmap can finalize the slider in the same update that advances past
	// its body proxy. Treat a missing render resource as an already-faded body
	// so a late frame or seek cannot crash the draw path.
	if slider.body == nil || slider.bodyFade == nil || slider.diff == nil {
		return
	}
	if rangeAtTime := slider.visualBodyRange(time); rangeAtTime.head == rangeAtTime.tail {
		return
	}

	colorAlpha := slider.bodyFade.GetValue() * float64(bodyColor.A)

	bodyOpacityInner := mutils.Clamp(float32(settings.Objects.Colors.Sliders.Body.InnerAlpha), 0.0, 1.0)
	bodyOpacityOuter := mutils.Clamp(float32(settings.Objects.Colors.Sliders.Body.OuterAlpha), 0.0, 1.0)

	borderInner := color2.NewRGBA(innerBorder.R, innerBorder.G, innerBorder.B, float32(colorAlpha))
	borderOuter := color2.NewRGBA(outerBorder.R, outerBorder.G, outerBorder.B, float32(colorAlpha))
	bodyInner := color2.NewL(0)
	bodyOuter := color2.NewL(0)

	if slider.diff.CheckModActive(difficulty.Traceable) && slider.HitObjectID != 0 {
		borderInner = skin.GetObjectColor(int(slider.ComboSet), int(slider.ComboSetHax), circleColor)
		borderOuter = borderInner
		bodyOpacityInner = 0
		bodyOpacityOuter = 0
	} else if settings.Skin.UseColorsFromSkin {
		borderOuter = skin.GetColor(skin.SliderBorder)
		borderInner = borderOuter

		borderOuter.A = float32(colorAlpha)
		borderInner.A = float32(colorAlpha)

		var baseTrack color2.Color

		if c, ok := skin.TryGetColor(skin.SliderTrackOverride); ok {
			baseTrack = c
		} else {
			baseTrack = skin.GetObjectColor(int(slider.ComboSet), int(slider.ComboSetHax), baseTrack)
		}

		bodyOuter = baseTrack.Shade2(-0.1)
		bodyInner = baseTrack.Shade2(0.5)
	} else {
		if settings.Objects.Colors.Sliders.Border.UseHitCircleColor {
			borderInner = skin.GetObjectColor(int(slider.ComboSet), int(slider.ComboSetHax), borderInner)
			borderOuter = skin.GetObjectColor(int(slider.ComboSet), int(slider.ComboSetHax), borderOuter)
		}

		if settings.Objects.Colors.Sliders.Body.UseHitCircleColor {
			bodyColor = skin.GetObjectColor(int(slider.ComboSet), int(slider.ComboSetHax), bodyColor)
		}

		if settings.Objects.Colors.Sliders.Border.EnableCustomGradientOffset {
			borderOuter = borderInner.Shift(float32(settings.Objects.Colors.Sliders.Border.CustomGradientOffset), 0, 0)
		}

		bodyInner = bodyColor.Shade2(float32(settings.Objects.Colors.Sliders.Body.InnerOffset))
		bodyOuter = bodyColor.Shade2(float32(settings.Objects.Colors.Sliders.Body.OuterOffset))
	}

	borderInner.A = float32(colorAlpha)
	borderOuter.A = float32(colorAlpha)
	bodyInner.A = float32(colorAlpha) * bodyOpacityInner
	bodyOuter.A = float32(colorAlpha) * bodyOpacityOuter

	stackIndex := slider.GetStackIndexMod(slider.diff)
	stackOffset := -float32(stackIndex) * float32(slider.diff.CircleRadius) / 10

	slider.body.DrawNormal(projection, vector.NewVec2f(stackOffset, stackOffset), scale, bodyInner, bodyOuter, borderInner, borderOuter)
}

func (slider *Slider) Draw(time float64, color color2.Color, batch *batch.QuadBatch) bool {
	if len(slider.scorePath) == 0 {
		return true
	}

	visualEndTime := slider.visualEndTime()

	alpha := slider.fade.GetValue() * float64(color.A)

	if settings.DIVIDES >= settings.Objects.Colors.MandalaTexturesTrigger {
		alpha *= settings.Objects.Colors.MandalaTexturesAlpha
	}

	batch.SetColor(float64(color.R), float64(color.G), float64(color.B), alpha)

	if settings.DIVIDES < settings.Objects.Colors.MandalaTexturesTrigger {
		if time < visualEndTime {
			if settings.Objects.Sliders.DrawScorePoints {
				shifted := color.Shift(float32(settings.Objects.Colors.Sliders.ScorePointColorOffset), 0, 0)

				scorePoint := skin.GetTexture("sliderscorepoint")

				for _, p := range slider.visualTickPoints() {
					if scorePoint == nil || p.fade == nil || p.scale == nil {
						continue
					}

					al := p.fade.GetValue()

					if al > 0.001 {
						batch.SetTranslation(p.Pos.Copy64())
						batch.SetSubScale(p.scale.GetValue(), p.scale.GetValue())

						if settings.Objects.Colors.Sliders.WhiteScorePoints || settings.Skin.UseColorsFromSkin {
							batch.SetColor(1, 1, 1, alpha*al)
						} else {
							batch.SetColor(float64(shifted.R), float64(shifted.G), float64(shifted.B), alpha*al)
						}

						batch.DrawTexture(*scorePoint)
					}
				}
			}
		}

		batch.SetSubScale(1, 1)

		for i := len(slider.endCircles) - 1; i >= 0; i-- {
			slider.endCircles[i].Draw(time, color, batch)
		}
	}

	batch.SetColor(1, 1, 1, 1)
	slider.startCircle.Draw(time, color, batch)

	if time >= slider.StartTime && time <= visualEndTime {
		slider.drawBall(time, batch, color, alpha, settings.Objects.Sliders.ForceSliderBallTexture || settings.DIVIDES < settings.Objects.Colors.MandalaTexturesTrigger)
	}

	if settings.DIVIDES < settings.Objects.Colors.MandalaTexturesTrigger && settings.Objects.Sliders.DrawSliderFollowCircle && slider.follower != nil {
		batch.SetTranslation(slider.Pos.Copy64())
		batch.SetColor(1, 1, 1, alpha)
		slider.follower.Draw(time, batch)
	}

	batch.SetSubScale(1, 1)
	batch.SetTranslation(vector.NewVec2d(0, 0))

	return time >= visualEndTime && slider.fade.GetValue() <= 0.001
}

func (slider *Slider) Finalize() {
	slider.StopSlideSamples()
	if slider.body != nil {
		slider.body.Dispose()
		slider.body = nil
	}
}

func (slider *Slider) drawBall(time float64, batch *batch.QuadBatch, color color2.Color, alpha float64, useBallTexture bool) {
	batch.SetTranslation(slider.ball.GetPosition())

	source := skin.GetSourceFromTexture(slider.ball.Texture)

	if useBallTexture && skin.GetTextureSource("sliderb-nd", source) != nil {
		batch.SetColor(0.1, 0.1, 0.1, alpha*slider.ball.GetAlpha())
		batch.DrawTexture(*skin.GetTexture("sliderb-nd"))
	}

	if settings.Skin.UseColorsFromSkin {
		color := color2.NewL(1)

		if skin.GetInfo().SliderBallTint {
			color = skin.GetObjectColor(int(slider.ComboSet), int(slider.ComboSetHax), color)
		} else if c, ok := skin.TryGetColor(skin.SliderBall); ok {
			color = c
		}

		batch.SetColor(float64(color.R), float64(color.G), float64(color.B), alpha)
	} else if settings.Objects.Colors.Sliders.SliderBallTint {
		color = skin.GetObjectColor(int(slider.ComboSet), int(slider.ComboSetHax), color)
		batch.SetColor(float64(color.R), float64(color.G), float64(color.B), alpha)
	} else {
		batch.SetColor(1, 1, 1, alpha)
	}

	if useBallTexture {
		batch.SetTranslation(vector.NewVec2d(0, 0))
		slider.ball.Draw(time, batch)
		batch.SetTranslation(slider.ball.GetPosition())
	} else {
		batch.DrawTexture(*skin.GetTexture("hitcircle-full"))
	}

	if useBallTexture && skin.GetTextureSource("sliderb-spec", source) != nil {
		batch.SetColor(1, 1, 1, alpha*slider.ball.GetAlpha())
		batch.SetAdditive(true)
		batch.DrawTexture(*skin.GetTexture("sliderb-spec"))
		batch.SetAdditive(false)
	}
}

func (slider *Slider) DrawApproach(time float64, color color2.Color, batch *batch.QuadBatch) {
	if len(slider.scorePath) == 0 {
		return
	}

	slider.startCircle.DrawApproach(time, color, batch)
}

func (slider *Slider) GetType() Type {
	return SLIDER
}
