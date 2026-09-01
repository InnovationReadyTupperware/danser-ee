package movers

import (
	"math"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/curves"
	"github.com/innovationreadytupperware/danser-ee/framework/math/math32"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const (
	streamEntryMin = 25
	streamEntryMax = 4000
	streamEscape   = 8000
)

type SplineMover struct {
	*basicMover

	curve     *curves.Spline
	objs      []objects.IHitObject
	lastTime  float64
	points    []vector.Vector2f
	timing    []float64
	timeDiff  []float32
	solver    curves.BSplineSolver
	beziers   []curves.Bezier
	curvePath []curves.Curve
}

func NewSplineMover() MultiPointMover {
	return &SplineMover{basicMover: &basicMover{}, lastTime: -math.MaxFloat64}
}

func (mover *SplineMover) Reset(diff *difficulty.Difficulty, id int) {
	mover.basicMover.Reset(diff, id)
	mover.lastTime = -math.MaxFloat64
	mover.curve = nil
	mover.objs = mover.objs[:0]
}

func (mover *SplineMover) SetObjects(objs []objects.IHitObject) int {
	config := settings.CursorDance.MoverSettings.Spline[mover.id%len(settings.CursorDance.MoverSettings.Spline)]

	mover.objs = mover.objs[:0]
	mover.points = mover.points[:0]
	mover.timing = mover.timing[:0]
	points := mover.points
	timing := mover.timing

	var angle float32
	var stream bool

	i := 0

	for ; i < len(objs); i++ {
		o := objs[i]

		if i == 0 {
			cEnd := objects.GetStackedEndPositionModForDiff(o, mover.diff)
			nStart := objs[i+1].GetStackedStartPositionMod(mover.diff)

			var wPoint vector.Vector2f

			switch s := o.(type) {
			case objects.ILongObject:
				wPoint = cEnd.Add(vector.NewVec2fRad(s.GetEndAngleMod(mover.diff), cEnd.Dst(nStart)*0.7))
			default:
				wPoint = cEnd.Lerp(nStart, 0.333)
			}

			points = append(points, cEnd, wPoint)
			timing = append(timing, max(o.GetStartTime(), mover.GetObjectsEndTime(o)))

			mover.startTime = max(o.GetStartTime(), mover.GetObjectsEndTime(o))

			continue
		}

		if _, ok := o.(objects.ILongObject); ok || i == len(objs)-1 {
			pEnd := objects.GetStackedEndPositionModForDiff(objs[i-1], mover.diff)
			cStart := o.GetStackedStartPositionMod(mover.diff)

			var wPoint vector.Vector2f

			switch s := o.(type) {
			case objects.ILongObject:
				wPoint = cStart.Add(vector.NewVec2fRad(s.GetStartAngleMod(mover.diff), cStart.Dst(pEnd)*0.7))
			default:
				wPoint = cStart.Lerp(pEnd, 0.333)
			}

			points = append(points, wPoint, cStart)
			timing = append(timing, o.GetStartTime())

			mover.endTime = o.GetStartTime()

			break
		} else if i > 1 && i < len(objs)-1 {
			pos1 := objs[i-1].GetStackedStartPositionMod(mover.diff)
			pos2 := o.GetStackedStartPositionMod(mover.diff)
			pos3 := objs[i+1].GetStackedStartPositionMod(mover.diff)

			minV := float32(streamEntryMin)
			maxV := float32(streamEntryMax)
			if stream {
				maxV = streamEscape
			}

			sq1 := pos1.DstSq(pos2)
			sq2 := pos2.DstSq(pos3)

			if sq1 > maxV && sq2 > maxV && config.RotationalForce {
				if stream {
					angle = 0
					stream = false
				} else {
					ang := int(math32.Abs(pos1.AngleRV(pos2) - pos1.AngleRV(pos3)))

					if ang == 0 {
						angle *= -1
					} else {
						angle = float32(ang) * 90 / 180 * math32.Pi
					}
				}
			} else if sq1 >= minV && sq1 <= maxV && sq2 >= minV && sq2 <= maxV && (config.StreamWobble || config.StreamHalfCircle) {
				if stream {
					angle *= -1

					if math32.Abs(angle) < 0.01 {
						pp1 := points[len(points)-1]

						shoeF := pp1.X*pos2.Y + pos2.X*pos3.Y + pos3.X*pp1.Y
						shoeS := pp1.Y*pos2.X + pos2.Y*pos3.X + pos3.Y*pp1.X

						sig := (shoeF - shoeS) > 0

						angle = math32.Pi / 2
						if sig {
							angle *= -1
						}
					}
				} else {
					stream = true
				}
			} else {
				stream = false
				angle = 0
			}

			if math32.Abs(angle) > 0.01 {
				mid := pos1.Mid(pos2)

				scale := float32(1.0)
				if stream && !config.StreamHalfCircle {
					scale = float32(config.WobbleScale)
				}

				if stream && config.StreamHalfCircle {
					sign := -1
					if angle < 0 {
						sign = 1
					}

					for t := -2; t <= 2; t++ {
						p4 := mid.Sub(pos1).Scl(scale).Rotate(angle + float32(sign*t)*math32.Pi/6).Add(mid)

						points = append(points, p4)
						timing = append(timing, (o.GetStartTime()-objs[i-1].GetStartTime())*(3+float64(t))/6+objs[i-1].GetStartTime())
					}
				} else {
					p4 := mid.Sub(pos1).Scl(scale).Rotate(angle).Add(mid)

					points = append(points, p4)
					timing = append(timing, (o.GetStartTime()-objs[i-1].GetStartTime())/2+objs[i-1].GetStartTime())
				}
			}
		}

		points = append(points, objects.GetStackedEndPositionModForDiff(o, mover.diff))
		timing = append(timing, o.GetStartTime())
		mover.objs = append(mover.objs, o)
	}

	// Keep the grown backing arrays on the mover. Without assigning the local
	// append results back, every rebuild would discard its buffers and repeat
	// the dense-path allocation spike.
	mover.points = points
	mover.timing = timing

	neededTimeDiff := max(0, len(timing)-1)
	if cap(mover.timeDiff) < neededTimeDiff {
		mover.timeDiff = make([]float32, neededTimeDiff)
	} else {
		mover.timeDiff = mover.timeDiff[:neededTimeDiff]
	}
	timeDiff := mover.timeDiff

	for j := range timeDiff {
		timeDiff[j] = float32(timing[j+1] - timing[j])
	}

	beziers := mover.solver.Solve(points, mover.beziers)
	mover.beziers = beziers
	if cap(mover.curvePath) < len(beziers) {
		mover.curvePath = make([]curves.Curve, len(beziers))
	} else {
		mover.curvePath = mover.curvePath[:len(beziers)]
	}

	for j := range beziers {
		b := &mover.beziers[j]
		if timeDiff[j] > 600 {
			scl := timeDiff[j] / 2

			b.Points[1] = b.Points[0].Add(b.Points[1].Sub(b.Points[0]).Nor().Scl(scl))
			b.Points[2] = b.Points[3].Add(b.Points[2].Sub(b.Points[3]).Nor().Scl(scl))
		}

		mover.curvePath[j] = &mover.beziers[j]
	}

	if len(mover.beziers) == 0 {
		mover.curve = nil
	} else if mover.curve == nil {
		mover.curve = curves.NewSplineW(mover.curvePath, timeDiff)
	} else {
		mover.curve.SetCurvesW(mover.curvePath, timeDiff)
	}

	return i + 1
}

func (mover *SplineMover) Update(time float64) vector.Vector2f {
	useMover := true
	var overridePos vector.Vector2f

	writeIndex := 0
	for _, g := range mover.objs {
		gStartTime := mover.GetObjectsStartTime(g)

		if gStartTime > time {
			mover.objs[writeIndex] = g
			writeIndex++
			continue
		}

		if mover.lastTime <= gStartTime {
			useMover = false
			overridePos = mover.GetObjectsStartPosition(g)
			continue
		}

		mover.objs[writeIndex] = g
		writeIndex++
	}
	clear(mover.objs[writeIndex:])
	mover.objs = mover.objs[:writeIndex]

	mover.lastTime = time

	if useMover && mover.curve != nil {
		t := movementProgress(time, mover.startTime, mover.endTime)
		return mover.curve.PointAt(t)
	}

	return overridePos
}
