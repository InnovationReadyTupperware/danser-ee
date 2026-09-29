package osu

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/objects"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/framework/assets"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/math/math87"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
	"github.com/wieku/rplpa"
)

// FixturesEnv names a directory holding replay conformance fixtures. Each
// fixture is a pair of files that share a base name:
//
//	<name>.osu    a difficulty read from a .osu or .osz
//	<name>.osr    the osu!stable replay recorded against it
//
// The map files and replays are third-party osu! content and are deliberately
// not committed, so this test skips unless the variable points at a populated
// directory. scripts/replay-conformance.ps1 builds and runs the test binary from
// the repository root, which is also required for the bundled assets and
// SDL3.dll to resolve, because env.LibDir is the executable's directory.
const FixturesEnv = "DANSER_REPLAY_FIXTURES"

// framesPerStep bounds how much replay one main-loop iteration consumes so
// queued main-thread work (skin texture uploads) drains between chunks the
// same way it does during real playback.
const framesPerStep = 2000

// replayFixture is one replay plus the beatmap it was recorded on.
type replayFixture struct {
	name   string
	osu    string
	replay string
}

type simulationResult struct {
	score   Score
	events  []judgementEvent
	stream  []scoreEvent
	lastMap *beatmap.BeatMap
	diff    *difficulty.Difficulty
	cursor  *graphics.Cursor
	ruleset *OsuRuleSet
}

// scoreEvent is one entry of the score stream exactly as the score processor
// received it, including nested slider point results.
type scoreEvent struct {
	hit   HitResult
	max   HitResult
	combo ComboResult
	part  sliderJudgementPart
	obj   HitObject
}

// recordingScoreProcessor delegates to the real processor while recording
// every result, so the diagnostic observes the same stream the score is
// computed from rather than the filtered hit listener.
type tracingScoreProcessor struct {
	inner scoreProcessor
	trace func(JudgementResult)
}

func (p *tracingScoreProcessor) Init(bMap *beatmap.BeatMap, player *difficultyPlayer) {
	p.inner.Init(bMap, player)
}

func (p *tracingScoreProcessor) AddResult(result JudgementResult) {
	if p.trace != nil {
		p.trace(result)
	}
	p.inner.AddResult(result)
}

func (p *tracingScoreProcessor) ModifyResult(result HitResult, src HitObject) HitResult {
	return p.inner.ModifyResult(result, src)
}

func (p *tracingScoreProcessor) GetScore() int64      { return p.inner.GetScore() }
func (p *tracingScoreProcessor) GetCombo() int64      { return p.inner.GetCombo() }
func (p *tracingScoreProcessor) GetAccuracy() float64 { return p.inner.GetAccuracy() }

// judgementEvent is one base-hit judgement observed during the simulation.
type judgementEvent struct {
	time      int64
	hit       HitResult
	max       HitResult
	combo     ComboResult
	part      sliderJudgementPart
	num       int64
	kind      string
	startTime int64
	endTime   int64
	points    []sliderEvent
	slider    *Slider
}

var (
	glOnce sync.Once
	glErr  error
)

// ensureSimulationEnvironment prepares the pieces a judgement run needs: the
// bundled assets, a hidden offscreen OpenGL context, and danser's main-thread
// dispatcher. Judgement arms object visuals, which uploads skin textures and
// fonts through queued main-thread work, so a real context is required even
// though nothing is ever presented. Hidden mirrors danser's record mode.
func ensureSimulationEnvironment(t *testing.T) {
	t.Helper()

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		_, sdlErr := os.Stat(filepath.Join(dir, "SDL3.dll"))
		_, skinErr := os.Stat(filepath.Join(dir, "assets", "default-skin", "skin.ini"))
		if sdlErr != nil || skinErr != nil {
			t.Skip("simulation assets not next to the test binary; run scripts/replay-conformance.ps1")
		}
	}

	glOnce.Do(func() {
		if glErr = gcontext.Initialize(true); glErr != nil {
			return
		}

		if glErr = gcontext.SDLCreateWindow(1920, 1080, "danser-conformance", gcontext.OptionalProps{Hidden: true}); glErr != nil {
			return
		}

		glErr = gcontext.GLInit(false)
	})

	if glErr != nil {
		t.Skipf("no offscreen OpenGL context available: %v", glErr)
	}
}

// loadFixtures reads the fixture directory named by FixturesEnv. An unset or
// unreadable directory is a skip rather than a failure: these fixtures are not
// part of the repository.
func loadFixtures(t *testing.T) []replayFixture {
	t.Helper()

	root := strings.TrimSpace(os.Getenv(FixturesEnv))
	if root == "" {
		t.Skipf("%s is not set; replay conformance fixtures are not committed", FixturesEnv)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skip("fixture directory is unreadable")
	}

	// A fixture may be supplied either as a bare .osu or inside the .osz the
	// player downloaded. Extract the .osz members next to the replays first so
	// both shapes can be paired by base name.
	mapsByName := map[string]string{}
	replays := map[string]string{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		path := filepath.Join(root, e.Name())
		base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))

		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".osu":
			mapsByName[base] = path
		case ".osz":
			mapsByName = mergeOsuMembers(t, path, mapsByName)
		case ".osr":
			replays[base] = path
		}
	}

	// Replays and maps rarely share a file name. A replay identifies its
	// difficulty by the MD5 of the .osu it was recorded on, so pair on that.
	type candidate struct {
		base  string
		osu   string
		md5   string
		order int
	}

	var maps []candidate

	order := 0

	for base, path := range mapsByName {
		maps = append(maps, candidate{base, path, fileMD5(t, path), order})
		order++
	}

	sort.Slice(maps, func(i, j int) bool { return maps[i].order < maps[j].order })

	var out []replayFixture

	for _, replayPath := range replays {
		raw, err := os.ReadFile(replayPath)
		if err != nil {
			t.Log("skip unreadable replay")
			continue
		}

		rp, err := rplpa.ParseReplay(raw)
		if err != nil {
			t.Log("skip invalid replay")
			continue
		}

		for _, m := range maps {
			if strings.EqualFold(m.md5, rp.BeatmapMD5) {
				out = append(out, replayFixture{m.md5[:12], m.osu, replayPath})
				break
			}
		}
	}

	if len(out) == 0 {
		t.Skip("no replay matched a supplied .osu/.osz by beatmap MD5")
	}

	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func fileMD5(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func mergeOsuMembers(t *testing.T, oszPath string, into map[string]string) map[string]string {
	t.Helper()

	zr, err := zip.OpenReader(oszPath)
	if err != nil {
		return into
	}
	defer zr.Close()

	dest := t.TempDir()

	for _, zf := range zr.File {
		if !strings.HasSuffix(strings.ToLower(zf.Name), ".osu") {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			continue
		}

		data, err := io.ReadAll(rc)
		rc.Close()

		if err != nil {
			continue
		}

		name := filepath.Base(zf.Name)
		target := filepath.Join(dest, name)
		if err := os.WriteFile(target, data, 0o644); err != nil {
			continue
		}

		into[strings.TrimSuffix(name, filepath.Ext(name))] = target
	}

	return into
}

// simulateReplay drives a Stable replay through the ruleset using the recorded
// input frames and returns the resulting score plus every base-hit judgement.
func simulateReplay(t *testing.T, fx replayFixture) simulationResult {
	return simulateReplayMode(t, fx, false)
}

func simulateReplayMode(t *testing.T, fx replayFixture, forceLazer bool) simulationResult {
	t.Helper()

	raw, err := os.ReadFile(fx.replay)
	if err != nil {
		t.Fatal("read replay failed")
	}

	rp, err := rplpa.ParseReplay(raw)
	if err != nil {
		t.Fatal("parse replay failed")
	}

	var (
		result   simulationResult
		finished bool
		frame    int
		lastTime int64
		cur      float64
		cursor   *graphics.Cursor
		ruleset  *OsuRuleSet
	)

	goroutines.RunMain(func() {
		goroutines.RunMainLoopWithHooks(
			func() bool { return !finished },

			func() {
				bMap, err := loadFixtureMap(fx.osu)
				if err != nil {
					t.Error("load fixture map failed")
					finished = true
					return
				}

				diff := bMap.Diff.Clone()
				diff.SetModsFromReplay(rp)
				diff.SetGameplayMode(difficulty.GameplayModeFromReplayVersion(int(rp.OsuVersion)))
				if forceLazer {
					diff.SetGameplayMode(difficulty.GameplayLazer)
				}

				bMap.Diff = diff
				bMap.Reset()

				// graphics.NewCursor builds a framebuffer against a sized
				// context; the ruleset only reads RawPosition and the discrete
				// button samples, so a zero-value cursor is sufficient.
				cursor = &graphics.Cursor{IsReplay: true}

				ruleset = NewOsuRuleset(bMap, []*graphics.Cursor{cursor}, []*difficulty.Difficulty{diff})
				ruleset.SetFailSuppressed(cursor, true)

				// Wrap the score processor so the captured stream is the one
				// the processor actually consumes. The hit listener is not that
				// stream: SendResult filters Ignore/PositionalMiss and applies
				// Perfect/SuddenDeath before calling it, and nested slider point
				// results never reach it in the same shape.
				if sp := ruleset.cursors[cursor].scoreProcessor; sp != nil {
					ruleset.cursors[cursor].scoreProcessor = &tracingScoreProcessor{
						inner: sp,
						trace: func(r JudgementResult) {
							result.stream = append(result.stream, scoreEvent{
								hit:   r.HitResult,
								max:   r.MaxResult,
								combo: r.ComboResult,
								part:  r.sliderPart,
								obj:   r.object,
							})
						},
					}
				}

				ruleset.SetListener(func(_ *graphics.Cursor, res JudgementResult, _ Score) {
					// A Stable slider's object-level judgement is synthesised
					// in UpdatePostFor from the proportion of its points that
					// were hit, so the summary result is the one to attribute.
					if res.HitResult&BaseHitsM == 0 && res.sliderPart != sliderPartSummary {
						return
					}

					ev := judgementEvent{
						time:  res.Time,
						hit:   res.HitResult,
						max:   res.MaxResult,
						combo: res.ComboResult,
						part:  res.sliderPart,
						num:   res.Number,
					}

					if sl, ok := res.object.(*Slider); ok && sl.hitSlider != nil {
						ev.kind = "slider"
						ev.slider = sl
						ev.points = buildStableSliderEvents(sl.hitSlider)
						ev.startTime = int64(sl.hitSlider.GetStartTime())
						ev.endTime = int64(sl.hitSlider.GetEndTime())
					} else if c, ok := res.object.(*Circle); ok {
						ev.kind = "circle"
						ev.startTime = int64(c.hitCircle.GetStartTime())
					} else {
						ev.kind = "other"
					}

					result.events = append(result.events, ev)
				})

				result.lastMap = bMap
				result.diff = diff
				result.cursor = cursor
				result.ruleset = ruleset
			},

			func() {
				if finished {
					return
				}

				end := min(len(rp.ReplayData), frame+framesPerStep)

				for ; frame < end; frame++ {
					fr := rp.ReplayData[frame]

					cur += fr.Time
					now := int64(cur)

					cursor.RawPosition = vector.NewVec2d(fr.MouseX, fr.MouseY).Copy32()
					cursor.LastFrameTime = lastTime
					cursor.CurrentFrameTime = now
					cursor.IsInputFrame = true
					cursor.IsReplayFrame = true
					cursor.LeftButton = fr.KeyPressed.LeftClick
					cursor.RightButton = fr.KeyPressed.RightClick
					cursor.LeftKey = fr.KeyPressed.LeftClick && fr.KeyPressed.Key1
					cursor.RightKey = fr.KeyPressed.RightClick && fr.KeyPressed.Key2
					cursor.LeftMouse = fr.KeyPressed.LeftClick && !fr.KeyPressed.Key1
					cursor.RightMouse = fr.KeyPressed.RightClick && !fr.KeyPressed.Key2
					cursor.SmokeKey = fr.KeyPressed.Smoke

					traceTargetSliderFrame(t, result, now)
					if now != lastTime {
						ruleset.UpdateClickFor(cursor, now)
						ruleset.UpdateNormalFor(cursor, now, false)
						ruleset.UpdatePostFor(cursor, now, false)
						ruleset.Update(now)
						lastTime = now
					}
				}

				if frame >= len(rp.ReplayData) {
					result.score = ruleset.GetScore(cursor)
					finished = true
				}
			},

			nil,
		)
	})

	return result
}

func traceTargetSliderFrame(t *testing.T, result simulationResult, now int64) {
	if result.ruleset == nil || len(result.lastMap.HitObjects) != 1821 ||
		!(now >= 214710 && now <= 214770 || now >= 433545 && now <= 433610) {
		return
	}

	player := result.ruleset.cursors[result.cursor].player
	for _, object := range result.ruleset.processed {
		slider, ok := object.(*Slider)
		if !ok || slider.GetNumber() != 764 && slider.GetNumber() != 1721 {
			continue
		}
		state := slider.state[player]
		position := objects.ModifyPosition(slider.hitSlider.HitObject, slider.hitSlider.GetPositionAt(float64(now)), player.diff)
		radius := player.diff.GetRadius()
		if state.sliding {
			radius = math87.Mul87(radius, 2.4)
		}
		t.Logf("tail frame #%d t=%d held=%v raw=%v slider=%v distanceSq=%.3f radiusSq=%.3f sliding=%v slideStart=%d downButton=%d gameDown=%v judged=%v",
			slider.GetNumber(), now, result.cursor.LeftButton || result.cursor.RightButton,
			result.cursor.RawPosition, position, result.cursor.RawPosition.DstSq87(position), math87.Mul87(radius, radius),
			state.sliding, state.slideStart, state.downButton, player.gameDownState, state.points[len(state.points)-1].judged)
	}
}

func TestJustabilityLazerDisplay(t *testing.T) {
	fixtures := loadFixtures(t)
	var justability *replayFixture
	for i := range fixtures {
		if strings.Contains(strings.ToLower(filepath.Base(fixtures[i].osu)), "justability") {
			justability = &fixtures[i]
			break
		}
	}
	if justability == nil {
		t.Skip("Justability fixture is not available")
	}

	runtime.LockOSThread()
	env.Init("danser")
	assets.Init(true)
	ensureSimulationEnvironment(t)

	got := simulateReplayMode(t, *justability, true)
	data, err := os.ReadFile(justability.replay)
	if err != nil {
		t.Fatal("read replay failed")
	}
	replay, err := rplpa.ParseReplay(data)
	if err != nil {
		t.Fatal("parse replay failed")
	}
	recorded := RecordedStableScore{
		TotalScore: int64(replay.Score),
		MaxCombo:   int(replay.MaxCombo),
		Count300:   int(replay.Count300),
		Count100:   int(replay.Count100),
		Count50:    int(replay.Count50),
		CountMiss:  int(replay.CountMiss),
	}
	legacyMultiplier := got.ruleset.GetFinalDiffAttribs(got.cursor).LegacyScoreBaseMultiplier
	attrs := simulateLegacyMigrationAttributes(got.lastMap, legacyMultiplier)
	migratedScore := standardisedStableScore(recorded, got.lastMap, got.diff, legacyMultiplier)
	t.Logf("migration attributes: %+v multiplier=%v migrated=%d", attrs, legacyMultiplier, migratedScore)
	if migratedScore != 1186456 {
		t.Errorf("recorded score migration = %d, want 1186456", migratedScore)
	}
	stable := simulateReplayMode(t, *justability, false)
	stable.ruleset.SetRecordedStableScore(stable.cursor, recorded)
	if before := stable.ruleset.GetDisplayScore(stable.cursor, settings.ScoreDisplayStandardised); before != stable.score.Score {
		t.Errorf("score before replay end = %d, want playback score %d", before, stable.score.Score)
	}
	stable.ruleset.PlayerStopped(stable.cursor, 0)
	if standardised := stable.ruleset.GetDisplayScore(stable.cursor, settings.ScoreDisplayStandardised); standardised != 1186456 {
		t.Errorf("final standardised score = %d, want 1186456", standardised)
	}
	if classic := stable.ruleset.GetDisplayScore(stable.cursor, settings.ScoreDisplayClassic); classic != 128259993 {
		t.Errorf("final classic score = %d, want 128259993", classic)
	}
	if result := stable.ruleset.GetPresentationScore(stable.cursor); result.Count300 != 1800 || result.Count100 != 17 || result.Combo != 2685 {
		t.Errorf("final replay statistics = %d/%d combo %d, want 1800/17 combo 2685", result.Count300, result.Count100, result.Combo)
	}
	t.Logf("Lazer playback: standardised=%d classic=%d combo=%d 300=%d 100=%d 50=%d miss=%d",
		got.score.Score, ClassicDisplayScore(got.score.Score, len(got.lastMap.HitObjects)),
		got.score.Combo, got.score.Count300, got.score.Count100, got.score.Count50, got.score.CountMiss)
	const websiteStandardised = 1186456
	const websiteClassic = 128259993
	t.Logf("website: standardised=%d classic=%d", websiteStandardised, websiteClassic)
}

func loadFixtureMap(osuPath string) (*beatmap.BeatMap, error) {
	// ParseObjects reopens the file from SongsDir, so the difficulty has to be
	// visible there.
	dir := osuPath[:strings.LastIndex(osuPath, string(os.PathSeparator))]
	settings.General.OsuSongsDir = dir

	f, err := os.Open(osuPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bMap, err := beatmap.ParseBeatMapFileWithError(f)
	if err != nil {
		return nil, err
	}

	beatmap.ParseTimingPointsAndPauses(bMap)
	beatmap.ParseObjects(bMap, false, false)

	return bMap, nil
}

// headerAccuracy is the accuracy osu!stable records for these counts. Stable
// accuracy is a strictly determined function of the base-hit counts, so it
// doubles as a fingerprint for whether the counts agree.
func headerAccuracy(c300, c100, c50, cMiss uint16) float64 {
	n := float64(c300 + c100 + c50 + cMiss)
	if n == 0 {
		return 1
	}
	return (300*float64(c300) + 100*float64(c100) + 50*float64(c50)) / (300 * n)
}

// TestReplayHeaderConformance compares replay playback against the score and
// judgements recorded during the original osu!stable play. Playback may differ
// from the original play because replay input is sampled at discrete times.
func TestReplayHeaderConformance(t *testing.T) {
	fixtures := loadFixtures(t)

	// The GL context belongs to the thread that creates it.
	runtime.LockOSThread()

	env.Init("danser")
	assets.Init(true)

	ensureSimulationEnvironment(t)

	// The simulation is bound to the thread that owns the OpenGL context, so
	// fixtures are walked inline rather than through t.Run, which would move
	// each case onto a new goroutine and thread.
	for _, fx := range fixtures {
		raw, err := os.ReadFile(fx.replay)
		if err != nil {
			t.Fatal("read replay failed")
		}

		rp, err := rplpa.ParseReplay(raw)
		if err != nil {
			t.Fatal("parse replay failed")
		}

		got := simulateReplay(t, fx)
		wantAcc := headerAccuracy(rp.Count300, rp.Count100, rp.Count50, rp.CountMiss)

		t.Logf("=== %s ===", fx.name)
		t.Logf("osuVer=%d mods=%#x objects=%d", rp.OsuVersion, rp.Mods, len(got.lastMap.HitObjects))
		t.Logf("danser: score=%d acc=%.4f%% combo=%d 300=%d 100=%d 50=%d miss=%d",
			got.score.Score, got.score.Accuracy*100, got.score.Combo,
			got.score.Count300, got.score.Count100, got.score.Count50, got.score.CountMiss)
		t.Logf("header: score=%d acc=%.4f%% combo=%d 300=%d 100=%d 50=%d miss=%d",
			rp.Score, wantAcc*100, rp.MaxCombo,
			rp.Count300, rp.Count100, rp.Count50, rp.CountMiss)

		if got.score.Count300 != uint(rp.Count300) ||
			got.score.Count100 != uint(rp.Count100) ||
			got.score.Count50 != uint(rp.Count50) ||
			got.score.CountMiss != uint(rp.CountMiss) {
			t.Errorf("%s: judgement counts differ from the replay header:\n"+
				"  300 danser=%d header=%d\n"+
				"  100 danser=%d header=%d\n"+
				"  50  danser=%d header=%d\n"+
				"  miss danser=%d header=%d",
				fx.name,
				got.score.Count300, rp.Count300,
				got.score.Count100, rp.Count100,
				got.score.Count50, rp.Count50,
				got.score.CountMiss, rp.CountMiss)

			reportDowngrades(t, got, rp)
		}

		if got.score.Combo != uint(rp.MaxCombo) {
			t.Errorf("%s: max combo = %d, want %d", fx.name, got.score.Combo, rp.MaxCombo)
		}
	}
}

// reportDowngrades attributes the object-level judgement divergences.
//
// A Stable slider's score is collapsed from the proportion of its points that
// were hit (see Slider.UpdatePostFor), so losing a single tick, repeat or tail
// turns a 300 into a 100 while leaving miss and 50 counts untouched. For every
// downgraded slider this reports the tail time and the player's last held frame
// so a dropped point can be tied to a release.
func reportDowngrades(t *testing.T, got simulationResult, rp *rplpa.Replay) {
	t.Helper()

	w300 := got.lastMap.Diff.Hit300
	w100 := got.lastMap.Diff.Hit100
	w50 := got.lastMap.Diff.Hit50

	frames := absoluteFrames(rp)
	lastHeld := func(t int64) (int64, bool) {
		at, best := int64(0), int64(0)
		found := false
		for _, f := range frames {
			if f.t > t {
				break
			}
			at = f.t
			if f.held {
				best, found = f.t, true
			}
		}
		_ = at
		return best, found
	}

	extra := int(got.score.Count100) - int(rp.Count100)
	t.Logf("stable hit windows: 300<%d 100<%d 50<%d ; header implies %d extra 100s", w300, w100, w50, extra)

	for _, e := range got.events {
		base := e.hit & BaseHitsM
		if e.max&BaseHitsM != Hit300 || base == Hit300 {
			continue
		}

		switch e.kind {
		case "circle":
			delta := e.time - e.startTime
			verdict := "consistent with the windows"
			if absInt64(delta) < w300 {
				verdict = "IMPOSSIBLE: inside the 300 window"
			}
			t.Logf("  circle #%d judged %v at delta=%+dms -> %s", e.num, base, delta, verdict)

		case "slider":
			tail := int64(0)
			if n := len(e.points); n > 0 {
				tail = int64(e.points[n-1].time)
			}
			held, heldFound := lastHeld(tail)
			headDelta := e.time - e.startTime
			t.Logf("  slider #%d judged %v (part=%d) points=%d start=%d end=%d tail=%d headDelta=%+d lastHeld=%d(found=%v) release-tail=%+d",
				e.num, base, e.part, len(e.points), e.startTime, e.endTime, tail,
				headDelta, held, heldFound, held-tail)

			// Per-point state, read from the live judgement rather than
			// inferred, so the dropped point is identified directly.
			dumpSliderPoints(t, got, e)
		}
	}

	reportMissedPoints(t, got, frames)
	reportScoreStream(t, got)
	reportLegacyScoreReplay(t, got, rp.Score)
}

// reportScoreStream breaks the v1 score accumulation down by result type using
// every result received by the score processor, including nested slider points.
func reportScoreStream(t *testing.T, got simulationResult) {
	t.Helper()

	type bucket struct {
		count int64
		base  int64
	}

	buckets := map[HitResult]*bucket{}

	for _, e := range got.stream {
		v := e.hit
		b, ok := buckets[v]
		if !ok {
			b = &bucket{}
			buckets[v] = b
		}
		b.count++
		b.base += v.ScoreValue()
	}

	total := int64(0)
	for _, b := range buckets {
		total += b.base
	}

	t.Logf("score stream: %d results, base sum=%d, final score=%d (bonus=%d)",
		len(got.stream), total, got.score.Score, got.score.Score-total)

	for v, b := range buckets {
		if b.count == 0 {
			continue
		}
		t.Logf("   result=%-6d base=%-4d count=%-5d sum=%d", int(v), int(v.ScoreValue()), b.count, b.base)
	}
}

var pointKindNames = map[sliderPointKind]string{
	sliderPointTick:   "tick",
	sliderPointRepeat: "repeat",
	sliderPointTail:   "tail",
}

// reportMissedPoints counts points dropped while the replay button was held.
// A held button alone does not imply a hit; the cursor must also be within the
// slider's follow radius when the point is judged.
func reportMissedPoints(t *testing.T, got simulationResult, frames []struct {
	t    int64
	held bool
}) {
	t.Helper()

	// A point becomes due on the first frame at or after its own time, and it is
	// resolved on that frame, so held-ness has to be sampled there. Sampling the
	// preceding frame instead reports a drop for any point the player released
	// just before, which is not the same thing.
	heldAt := func(time int64) bool {
		for _, f := range frames {
			if f.t >= time {
				return f.held
			}
		}
		return false
	}

	var (
		sliders                                             int
		droppedWhileHeld, tickDrops, repeatDrops, tailDrops int
		samples                                             []string
	)

	for _, e := range got.events {
		if e.kind != "slider" || e.slider == nil || got.ruleset == nil || got.cursor == nil {
			continue
		}

		player := got.ruleset.GetPlayer(got.cursor)
		if player == nil {
			continue
		}

		state := e.slider.state[player]
		if state == nil {
			continue
		}

		sliders++

		for i := range state.points {
			point := &state.points[i]
			if point.judged && point.hitResult.IsHit() {
				continue
			}
			if !heldAt(int64(point.time)) {
				continue
			}

			droppedWhileHeld++

			switch point.kind {
			case sliderPointTail:
				tailDrops++
			case sliderPointRepeat:
				repeatDrops++
			default:
				tickDrops++
			}

			if len(samples) < 20 {
				samples = append(samples, fmt.Sprintf("slider #%d dropped a %s at %.0f (end %d)",
					e.num, pointKindNames[point.kind], point.time, e.endTime))
			}
		}
	}

	t.Logf("sliders inspected=%d", sliders)
	t.Logf("points dropped while the button was still held at that point's own time: %d (tick/repeat=%d tail=%d)",
		droppedWhileHeld, tickDrops, tailDrops)

	for _, s := range samples {
		t.Logf("    %s", s)
	}
}

// dumpSliderPoints reports each generated point of a Stable slider and how it
// was actually resolved. The summary result only records how many points were
// scored, so this is what identifies which point was lost.
func dumpSliderPoints(t *testing.T, got simulationResult, e judgementEvent) {
	t.Helper()

	if e.slider == nil || got.ruleset == nil || got.cursor == nil {
		return
	}

	player := got.ruleset.GetPlayer(got.cursor)
	if player == nil {
		return
	}

	state := e.slider.state[player]
	if state == nil {
		return
	}

	headHit := state.startResult.IsHit()
	scored, missed := 0, 0

	for i := range state.points {
		point := &state.points[i]
		if point.judged && point.hitResult.IsHit() {
			scored++
		} else {
			missed++
		}
	}

	t.Logf("     headHit=%v isHit=%v points=%d scored=%d missed=%d startResult=%v",
		headHit, state.isHit, len(state.points), scored, missed, state.startResult)

	for i := range state.points {
		point := &state.points[i]
		t.Logf("     point[%d] kind=%d time=%.0f judged=%v result=%v",
			i, point.kind, point.time, point.judged, point.hitResult&BaseHitsM)
	}
}

// absoluteFrames accumulates the replay's frame time deltas into absolute
// times, the way the replay controller does during playback.
func absoluteFrames(rp *rplpa.Replay) []struct {
	t    int64
	held bool
} {
	out := make([]struct {
		t    int64
		held bool
	}, 0, len(rp.ReplayData))

	cur := 0.0

	for _, f := range rp.ReplayData {
		cur += f.Time
		out = append(out, struct {
			t    int64
			held bool
		}{int64(cur), f.KeyPressed != nil && (f.KeyPressed.LeftClick || f.KeyPressed.RightClick)})
	}

	return out
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// reportLegacyScoreReplay checks that the captured processor stream reproduces
// the live score and compares it with the score recorded in the replay header.
func reportLegacyScoreReplay(t *testing.T, got simulationResult, header int32) {
	t.Helper()

	if len(got.stream) == 0 {
		t.Logf("no score stream captured")
		return
	}

	processor := got.ruleset.cursors[got.cursor].scoreProcessor.(*tracingScoreProcessor).inner.(*scoreV1Processor)
	mult := processor.scoreMultiplier
	modMult := processor.modMultiplier
	var priorFloatDivision, stable, combo, nested, bonusEvents int64
	for _, e := range got.stream {
		if e.obj != nil && (e.obj.GetNumber() == 764 || e.obj.GetNumber() == 1721) && len(got.lastMap.HitObjects) == 1821 {
			t.Logf("target score event: object=%d part=%d hit=%v max=%v combo=%d runningCombo=%d", e.obj.GetNumber(), e.part, e.hit, e.max, e.combo, combo)
		}
		if e.hit != SliderMiss && e.hit != Miss {
			base := e.hit.ScoreValue()
			priorFloatDivision += base
			stable += base
			if e.hit&RawHits == 0 {
				bonusCombo := max(combo-1, 0)
				priorFloatDivision += int64(float64(base) * float64(bonusCombo) * mult * modMult / 25)
				stable += int64(float64(bonusCombo) * float64(base/25) * mult * modMult)
				bonusEvents++
			} else if e.part != sliderPartNone {
				nested++
			}
		}
		if e.combo == Reset || e.hit == Miss {
			combo = 0
		} else if e.combo == Increase {
			combo++
		}
	}
	t.Logf("legacy v1 replay: prior-float-division=%d stable-division=%d actual=%d header=%d residual=%d",
		priorFloatDivision, stable, got.score.Score, header, int64(header)-stable)
	t.Logf("legacy v1 replay: final combo=%d nestedHits=%d bonusEvents=%d multiplier=%.4f modMultiplier=%.4f",
		combo, nested, bonusEvents, mult, modMult)
	attributes := got.ruleset.GetFinalDiffAttribs(got.cursor)
	maxAccuracyScore := int64(len(got.lastMap.HitObjects)) * 300
	for _, object := range got.lastMap.HitObjects {
		if slider, ok := object.(*objects.Slider); ok {
			maxAccuracyScore += 30
			for _, event := range buildStableSliderEvents(slider) {
				maxAccuracyScore += event.maxResult.ScoreValue()
			}
		}
	}
	t.Logf("legacy migration attributes: accuracyScore=%d comboScore=%d maxCombo=%d maxNestedPerObject=%.2f",
		maxAccuracyScore, attributes.MaximumLegacyComboScore, attributes.MaxCombo, attributes.NestedScorePerObject)
	if len(got.lastMap.HitObjects) == 1821 {
		reportCandidateSliderPairs(t, got.stream, mult, modMult, int64(header), 2685)
		corrected := &scoreV1Processor{modMultiplier: modMult, scoreMultiplier: mult}
		for _, event := range got.stream {
			result := JudgementResult{HitResult: event.hit, MaxResult: event.max, ComboResult: event.combo}
			if event.obj != nil && (event.obj.GetNumber() == 764 || event.obj.GetNumber() == 1721) {
				if event.part == sliderPartTail && result.HitResult == SliderMiss {
					result.HitResult = result.MaxResult
					result.ComboResult = Increase
				}
				if event.part == sliderPartSummary && result.HitResult == Hit100 {
					result.HitResult = Hit300
				}
			}
			corrected.AddResult(result)
		}
		t.Logf("counterfactual repaired tails/summaries: score=%d header=%d residual=%d", corrected.GetScore(), header, int64(header)-corrected.GetScore())
	}
}

func reportCandidateSliderPairs(t *testing.T, stream []scoreEvent, multiplier, modMultiplier float64, headerScore, headerCombo int64) {
	t.Helper()
	candidates := make([]int64, 0)
	for _, event := range stream {
		if event.obj != nil && event.part == sliderPartSummary && event.hit == Hit100 {
			candidates = append(candidates, event.obj.GetNumber())
		}
	}
	type candidateResult struct {
		first, second   int64
		score, maxCombo int64
	}
	results := make([]candidateResult, 0)
	for i, first := range candidates {
		for _, second := range candidates[i+1:] {
			processor := &scoreV1Processor{modMultiplier: modMultiplier, scoreMultiplier: multiplier}
			var maxCombo int64
			for _, event := range stream {
				judgement := JudgementResult{HitResult: event.hit, MaxResult: event.max, ComboResult: event.combo}
				if event.obj != nil && (event.obj.GetNumber() == first || event.obj.GetNumber() == second) {
					if event.part >= sliderPartTick && event.part <= sliderPartTail && judgement.HitResult == SliderMiss {
						judgement.HitResult = judgement.MaxResult
						judgement.ComboResult = Increase
					}
					if event.part == sliderPartSummary && judgement.HitResult == Hit100 {
						judgement.HitResult = Hit300
					}
				}
				processor.AddResult(judgement)
				maxCombo = max(maxCombo, processor.combo)
			}
			results = append(results, candidateResult{first, second, processor.score, maxCombo})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		left := absInt64(headerScore-results[i].score) + absInt64(headerCombo-results[i].maxCombo)*1000000
		right := absInt64(headerScore-results[j].score) + absInt64(headerCombo-results[j].maxCombo)*1000000
		return left < right
	})
	for _, result := range results[:min(8, len(results))] {
		t.Logf("candidate repaired sliders #%d/%d: score=%d residual=%d maxCombo=%d", result.first, result.second,
			result.score, headerScore-result.score, result.maxCombo)
	}
}
