package sliders

import (
	"math"
	"sort"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const (
	bodySampleSpacing = 4.0
	maxBodySamples    = 2048
	maxMotionSamples  = 8192
)

type motionSample struct {
	time     float64
	position vector.Vector2f
}

type motionPlan struct {
	slider             *objects.Slider
	diff               *difficulty.Difficulty
	body               []vector.Vector2f
	samples            []motionSample
	endTime            float64
	bodyRadius         float32
	trackRadius        float32
	followAuthoredPath bool
}

func newMotionPlan(slider *objects.Slider, diff *difficulty.Difficulty) *motionPlan {
	plan := &motionPlan{
		slider:  slider,
		diff:    diff,
		endTime: objects.GetEndTimeForDiff(slider, diff),
	}

	plan.bodyRadius = 32
	gameplayRadius := plan.bodyRadius
	if diff != nil {
		gameplayRadius = diff.GetRadius()
		plan.bodyRadius = float32(diff.CircleRadius)
		if diff.IsLazer() {
			plan.bodyRadius = float32(diff.CircleRadiusL)
		}
	}
	margin := max(float32(0.5), gameplayRadius*0.01)
	plan.trackRadius = gameplayRadius*2.4 - margin

	duration := plan.endTime - slider.GetStartTime()
	repeats := max(1, slider.RepeatCount)
	if duration <= 0 || !finite(duration) {
		plan.followAuthoredPath = true
		return plan
	}

	bodySteps := max(1, int(math.Ceil(float64(slider.GetLength())/bodySampleSpacing)))
	if bodySteps > maxBodySamples {
		plan.followAuthoredPath = true
		return plan
	}

	spanDuration := duration / float64(repeats)
	plan.body = make([]vector.Vector2f, bodySteps+1)
	for i := range bodySteps + 1 {
		time := slider.GetStartTime() + spanDuration*float64(i)/float64(bodySteps)
		plan.body[i] = plan.sliderPosition(time)
	}

	motionSteps := bodySteps * repeats
	if motionSteps > maxMotionSamples {
		plan.followAuthoredPath = true
		return plan
	}

	plan.samples = make([]motionSample, motionSteps+1)
	previous := plan.sliderPosition(slider.GetStartTime())
	for i := range motionSteps + 1 {
		time := slider.GetStartTime() + duration*float64(i)/float64(motionSteps)
		ball := plan.sliderPosition(time)
		if i > 0 {
			previous = plan.project(previous, ball)
		}
		plan.samples[i] = motionSample{time: time, position: previous}
	}

	return plan
}

func (plan *motionPlan) positionAt(time float64) vector.Vector2f {
	if plan.followAuthoredPath || len(plan.samples) < 2 {
		return plan.sliderPosition(time)
	}

	time = min(max(time, plan.slider.GetStartTime()), plan.endTime)
	index := sort.Search(len(plan.samples), func(i int) bool {
		return plan.samples[i].time >= time
	})
	if index == 0 {
		return plan.samples[0].position
	}
	if index == len(plan.samples) {
		return plan.samples[len(plan.samples)-1].position
	}

	previous := plan.samples[index-1]
	next := plan.samples[index]
	progress := float32((time - previous.time) / (next.time - previous.time))
	desired := previous.position.Lerp(next.position, progress)
	ball := plan.sliderPosition(time)

	if desired.DstSq(ball) <= plan.trackRadius*plan.trackRadius &&
		plan.distanceToBodySq(desired) <= plan.bodyRadius*plan.bodyRadius {
		return desired
	}

	return plan.project(desired, ball)
}

func (plan *motionPlan) sliderPosition(time float64) vector.Vector2f {
	if plan.diff == nil {
		return plan.slider.PositionAtForDiff(time, nil)
	}

	return plan.slider.GetStackedPositionAtModForDiff(time, plan.diff)
}

func (plan *motionPlan) project(desired, ball vector.Vector2f) vector.Vector2f {
	if len(plan.body) == 0 {
		return ball
	}

	best := ball
	bestDistance := desired.DstSq(ball)
	radiusSq := plan.trackRadius * plan.trackRadius

	for i := 1; i < len(plan.body); i++ {
		start := plan.body[i-1]
		delta := plan.body[i].Sub(start)
		lengthSq := delta.LenSq()
		if lengthSq == 0 {
			if start.DstSq(ball) <= radiusSq && desired.DstSq(start) < bestDistance {
				best = start
				bestDistance = desired.DstSq(start)
			}
			continue
		}

		lo, hi, ok := segmentInsideCircle(start, delta, ball, plan.trackRadius)
		if !ok {
			continue
		}

		projection := desired.Sub(start).Dot(delta) / lengthSq
		projection = min(max(projection, lo), hi)
		candidate := start.Add(delta.Scl(projection))
		distance := desired.DstSq(candidate)
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}

	return best
}

func (plan *motionPlan) distanceToBodySq(position vector.Vector2f) float32 {
	best := float32(math.MaxFloat32)
	for i := 1; i < len(plan.body); i++ {
		start := plan.body[i-1]
		delta := plan.body[i].Sub(start)
		lengthSq := delta.LenSq()
		if lengthSq == 0 {
			best = min(best, position.DstSq(start))
			continue
		}

		projection := position.Sub(start).Dot(delta) / lengthSq
		projection = min(max(projection, 0), 1)
		best = min(best, position.DstSq(start.Add(delta.Scl(projection))))
	}
	return best
}

func segmentInsideCircle(start, delta, center vector.Vector2f, radius float32) (float32, float32, bool) {
	relative := start.Sub(center)
	a := delta.Dot(delta)
	b := 2 * relative.Dot(delta)
	c := relative.Dot(relative) - radius*radius
	discriminant := b*b - 4*a*c
	if discriminant < 0 {
		return 0, 0, false
	}

	root := float32(math.Sqrt(float64(discriminant)))
	lo := max(float32(0), (-b-root)/(2*a))
	hi := min(float32(1), (-b+root)/(2*a))
	return lo, hi, lo <= hi
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
