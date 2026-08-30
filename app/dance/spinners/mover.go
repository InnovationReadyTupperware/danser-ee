package spinners

import (
	"math"
	"strings"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
)

const (
	defaultSpinnerRadius = 100
	spinnerCenterX       = 256
	spinnerCenterY       = 192
)

// SpinnerProfile is the immutable configuration snapshot used by one
// generated spinner mover. Capturing the profile when a queue is initialized
// prevents a settings-panel edit or a malformed config entry from changing a
// mover halfway through a replay.
type SpinnerProfile struct {
	Mover         string
	CenterOffsetX float64
	CenterOffsetY float64
	Radius        float64
}

// DefaultSpinnerProfile returns the safe centered circle profile used when a
// cursor-dance config is absent, empty, or malformed.
func DefaultSpinnerProfile() SpinnerProfile {
	return SpinnerProfile{
		Mover:  "circle",
		Radius: defaultSpinnerRadius,
	}
}

// ResolveSpinnerProfile returns the profile selected for a cursor-dance
// cursor. Configuration is intentionally read once at initialization time;
// spinner movers are then independent of the mutable settings tree.
func ResolveSpinnerProfile(index int) SpinnerProfile {
	profile := DefaultSpinnerProfile()

	if settings.CursorDance == nil || len(settings.CursorDance.Spinners) == 0 {
		return profile
	}

	index %= len(settings.CursorDance.Spinners)
	if index < 0 {
		index += len(settings.CursorDance.Spinners)
	}

	configured := settings.CursorDance.Spinners[index]
	if configured == nil {
		return profile
	}

	profile.Mover = configured.Mover
	profile.CenterOffsetX = configured.CenterOffsetX
	profile.CenterOffsetY = configured.CenterOffsetY
	profile.Radius = configured.Radius

	return normalizeSpinnerProfile(profile)
}

func normalizeSpinnerProfile(profile SpinnerProfile) SpinnerProfile {
	profile.Mover = normalizeMoverName(profile.Mover)

	if math.IsNaN(profile.CenterOffsetX) || math.IsInf(profile.CenterOffsetX, 0) || math.Abs(profile.CenterOffsetX) > math.MaxFloat32 {
		profile.CenterOffsetX = 0
	}

	if math.IsNaN(profile.CenterOffsetY) || math.IsInf(profile.CenterOffsetY, 0) || math.Abs(profile.CenterOffsetY) > math.MaxFloat32 {
		profile.CenterOffsetY = 0
	}

	if profile.Radius < 1 || profile.Radius > math.MaxFloat32 || math.IsNaN(profile.Radius) || math.IsInf(profile.Radius, 0) {
		profile.Radius = defaultSpinnerRadius
	}

	return profile
}

func normalizeMoverName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	switch name {
	case "heart", "triangle", "square", "cube", "circle":
		return name
	default:
		return "circle"
	}
}

// SpinnerMover supplies a cursor path for a spinner in cursor-dance mode.
// Implementations embed BaseMover so timing, gameplay mode, and selected RPM
// are applied consistently across every shape.
type SpinnerMover interface {
	Initialize(startTime, speed, targetRPM float64, mode difficulty.GameplayMode)
	PositionAt(time float64) vector.Vector2f
	Radius() float32
}

// BaseMover stores the timing and rate shared by shape-specific spinner
// movers. Stable uses the historical phase calculation, while Lazer uses an
// exact angular phase derived from the selected RPM. The latter is important:
// a polygon or 3D shape may change its radius, but it must not accidentally
// change how many rotations the generated cursor makes.
type BaseMover struct {
	startTime      float64
	speed          float64
	targetRPM      float64
	rateMultiplier float64
	mode           difficulty.GameplayMode
	profile        SpinnerProfile
}

// Initialize prepares a mover for one spinner object. The profile belongs to
// the mover instance and is therefore safe to reuse for every object assigned
// to the same cursor-dance scheduler.
func (mover *BaseMover) Initialize(startTime, speed, targetRPM float64, mode difficulty.GameplayMode) {
	mover.profile = normalizeSpinnerProfile(mover.profile)
	mover.startTime = startTime

	if speed <= 0 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		speed = 1
	}
	mover.speed = speed

	if targetRPM <= 0 || math.IsNaN(targetRPM) || math.IsInf(targetRPM, 0) {
		targetRPM = difficulty.LegacySpinnerRPM
	}
	mover.targetRPM = targetRPM
	mover.rateMultiplier = targetRPM / difficulty.LegacySpinnerRPM

	if mode != difficulty.GameplayStable {
		mode = difficulty.GameplayLazer
	}
	mover.mode = mode
}

// phaseAt returns the historical phase-unit value used by Stable-compatible
// mover formulas. Negative or non-finite pre-start times are clamped so a
// scheduler polling before the object starts cannot generate NaN positions or
// a backwards jump.
func (mover *BaseMover) phaseAt(time float64) float32 {
	elapsed := time - mover.startTime
	if elapsed <= 0 || math.IsNaN(elapsed) || math.IsInf(elapsed, 0) {
		return 0
	}

	timeScale := min(1.0, mover.speed)
	if timeScale <= 0 || math.IsNaN(timeScale) || math.IsInf(timeScale, 0) {
		timeScale = 1
	}

	phase := elapsed / timeScale * mover.rateMultiplier
	if math.IsNaN(phase) || math.IsInf(phase, 0) {
		return 0
	}

	result := float32(phase)
	if math.IsNaN(float64(result)) || math.IsInf(float64(result), 0) {
		return 0
	}

	return result
}

func (mover *BaseMover) effectiveElapsedAt(time float64) float64 {
	elapsed := time - mover.startTime
	if elapsed <= 0 || math.IsNaN(elapsed) || math.IsInf(elapsed, 0) {
		return 0
	}

	timeScale := min(1.0, mover.speed)
	if timeScale <= 0 || math.IsNaN(timeScale) || math.IsInf(timeScale, 0) {
		timeScale = 1
	}

	effectiveElapsed := elapsed / timeScale
	if math.IsNaN(effectiveElapsed) || math.IsInf(effectiveElapsed, 0) {
		return 0
	}

	return effectiveElapsed
}

func (mover *BaseMover) angleAt(time float64) float64 {
	if mover.mode.IsLazer() {
		angle := mover.effectiveElapsedAt(time) * mover.targetRPM / 60000 * 2 * math.Pi
		if math.IsNaN(angle) || math.IsInf(angle, 0) {
			return 0
		}

		return math.Mod(angle, 2*math.Pi)
	}

	angle := float64(difficulty.LegacySpinnerRotationsPerMillisecond * mover.phaseAt(time) * 2 * math.Pi)
	if math.IsNaN(angle) || math.IsInf(angle, 0) {
		return 0
	}

	return angle
}

func (mover *BaseMover) profileCenter() vector.Vector2f {
	return vector.NewVec2f(
		spinnerCenterX+float32(mover.profile.CenterOffsetX),
		spinnerCenterY+float32(mover.profile.CenterOffsetY),
	)
}

func (mover *BaseMover) polarPosition(angle, radius float64) vector.Vector2f {
	if radius <= 0 || math.IsNaN(radius) || math.IsInf(radius, 0) {
		radius = mover.profile.Radius
	}

	position := vector.NewVec2fRad(float32(angle), float32(radius))
	return position.Add(mover.profileCenter())
}

// Radius returns the configured path radius in pixels.
func (mover *BaseMover) Radius() float32 {
	return float32(mover.profile.Radius)
}

func newMover(profile SpinnerProfile) SpinnerMover {
	profile = normalizeSpinnerProfile(profile)

	switch profile.Mover {
	case "heart":
		return newHeartMover(profile)
	case "triangle":
		return newTriangleMover(profile)
	case "square":
		return newSquareMover(profile)
	case "cube":
		return newCubeMover(profile)
	default:
		return newCircleMover(profile)
	}
}

// GetMoverByName creates a mover using the first configured profile while
// selecting the shape by name. New code that has access to a cursor index
// should use ResolveSpinnerProfile and GetMoverCtor so the intended profile
// is captured explicitly.
func GetMoverByName(name string) SpinnerMover {
	profile := ResolveSpinnerProfile(0)
	profile.Mover = name
	return newMover(profile)
}

// GetMoverCtor returns a constructor that captures profile by value. Each
// scheduler receives a fresh mover without rereading global settings.
func GetMoverCtor(profile SpinnerProfile) func() SpinnerMover {
	profile = normalizeSpinnerProfile(profile)
	return func() SpinnerMover {
		return newMover(profile)
	}
}

// GetMoverCtorByName preserves the simple constructor used by non-dance input
// controllers. Those controllers historically used the first configured
// spinner's radius and offsets while selecting the shape by name, so retain
// that profile compatibility without making the mover read settings later.
func GetMoverCtorByName(name string) func() SpinnerMover {
	profile := ResolveSpinnerProfile(0)
	profile.Mover = name
	return GetMoverCtor(profile)
}
