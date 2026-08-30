package launcher

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/wieku/rplpa"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	"github.com/innovationreadytupperware/danser-ee/app/database"
	"github.com/innovationreadytupperware/danser-ee/app/graphics"
	"github.com/innovationreadytupperware/danser-ee/app/graphics/gui/drawables"
	"github.com/innovationreadytupperware/danser-ee/app/osuapi"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/app/states/components/common"
	appUtils "github.com/innovationreadytupperware/danser-ee/app/utils"
	"github.com/innovationreadytupperware/danser-ee/build"
	"github.com/innovationreadytupperware/danser-ee/framework/assets"
	"github.com/innovationreadytupperware/danser-ee/framework/bass"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/files"
	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/viewport"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"
	"github.com/innovationreadytupperware/danser-ee/framework/math/animation/easing"
	color2 "github.com/innovationreadytupperware/danser-ee/framework/math/color"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
	"github.com/innovationreadytupperware/danser-ee/framework/platform"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
	"github.com/innovationreadytupperware/danser-ee/framework/qpc"
	"github.com/innovationreadytupperware/danser-ee/framework/util"
)

type Mode int

const (
	CursorDance Mode = iota
	DanserReplay
	Replay
	Knockout
	Play
	// SoloKnockout is kept after Play so existing launcher.json files retain
	// their numeric Play selection when this mode is introduced.
	SoloKnockout
)

func (m Mode) String() string {
	switch m {
	case CursorDance:
		return "Cursor dance / mandala / tag"
	case DanserReplay:
		return "Cursor dance with UI"
	case Replay:
		return "Watch a replay"
	case Knockout:
		return "Watch knockout"
	case Play:
		return "Play osu!standard"
	case SoloKnockout:
		return "Watch solo knockout"
	}

	return ""
}

var modes = []Mode{CursorDance, DanserReplay, Replay, Knockout, SoloKnockout, Play}

func usesGeneratedCursors(mode Mode) bool {
	return mode == CursorDance || mode == SoloKnockout
}

type PMode int

const (
	Watch PMode = iota
	Record
	Screenshot
)

func (m PMode) String() string {
	switch m {
	case Watch:
		return "Watch"
	case Record:
		return "Record"
	case Screenshot:
		return "Screenshot"
	}

	return ""
}

var pModes = []PMode{Watch, Record, Screenshot}

type ConfigMode int

const (
	Rename ConfigMode = iota
	Clone
	New
)

// catalogProgressState is the complete worker-to-renderer progress snapshot.
// It is stored as one value in atomic.Value so the launcher never observes a
// partially updated stage, count, or completion flag while drawing a frame.
type catalogProgressState struct {
	generation uint64
	stage      database.ImportStage
	processed  int
	target     int
	active     bool
	prominent  bool
}

// catalogProgressVisibilityThreshold hides tiny refreshes from the normal
// launcher flow while still making a large reconciliation observable.
const catalogProgressVisibilityThreshold = 128

type launcher struct {
	runtimeDone   <-chan struct{}
	runtimeCancel context.CancelFunc
	runtimeEvents chan launcherEvent
	shutdownOnce  sync.Once
	backgroundWG  sync.WaitGroup

	bg *common.Background

	batch *batch.QuadBatch
	coin  *common.DanserCoin

	bld *builder

	catalog *database.CatalogSnapshot

	configList    []string
	currentConfig *settings.Config

	catalogProgress           atomic.Value
	catalogGeneration         atomic.Uint64
	catalogSnapshotReady      atomic.Bool
	catalogSnapshotGeneration atomic.Uint64
	beatmapEventPending       atomic.Bool
	catalogCoordinator        *catalogCoordinator

	newCloneOpened bool

	configManiMode ConfigMode
	configPrevName string

	newCloneName       string
	refreshRate        float32
	configEditOpened   bool
	danserRunning      bool
	danserStartPending bool
	startTimer         *time.Timer
	recordProgress     float32
	recordStatus       string
	recordStatusSpeed  string
	recordStatusETA    string
	showProgressBar    bool

	triangleSpeed     *animation.Glider
	encodeInProgress  bool
	encodeStart       time.Time
	processSupervisor *processSupervisor
	closeAfterDanser  bool
	popupStack        []iPopup

	selectWindow *songSelectPopup

	prevMap *beatmap.BeatMap

	configSearch string

	knockoutManager *knockoutManagerPopup

	currentEditor     *settingsEditor
	beatmapDirUpdated bool
	showBeatmapAlert  float64
	watcher           *directoryWatcher

	winter        bool
	christmas     bool
	recordSnowPos vector.Vector2f

	snow *drawables.Snow

	timeMenu *timePopup

	cHold           map[string]*bool
	configScrolling bool
}

func StartLauncher() {
	defer func() {
		var err any
		var stackTrace []string

		if err = recover(); err != nil {
			stackTrace = goroutines.GetStackTrace(4)
		}

		closeHandler(err, stackTrace)
	}()

	goroutines.SetCrashHandler(closeHandler)

	cTime := time.Now()
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())

	launcher := &launcher{
		runtimeDone:   runtimeCtx.Done(),
		runtimeCancel: runtimeCancel,
		runtimeEvents: make(chan launcherEvent, launcherEventQueueCapacity),
		bld:           newBuilder(),
		catalog:       database.NewCatalogSnapshot(nil),
		popupStack:    make([]iPopup, 0),
		winter:        (cTime.Month() == 12 && cTime.Day() >= 6) || (cTime.Month() < 2),
		christmas:     cTime.Month() == 12 && cTime.Day() >= 6,
		cHold:         make(map[string]*bool),
	}

	platform.StartLogging("launcher")

	loadLauncherConfig()

	settings.CreateDefault()

	settings.Playfield.Background.Triangles.Enabled = true
	settings.Playfield.Background.Triangles.DrawOverBlur = true
	settings.Playfield.Background.Blur.Enabled = false
	settings.Playfield.Background.Parallax.Enabled = true
	settings.Playfield.Background.Parallax.Amount = 0.02

	assets.Init(build.Stream == "Dev")

	goroutines.RunMain(func() {
		defer func() {
			if err := recover(); err != nil {
				stackTrace := goroutines.GetStackTrace(4)
				closeHandler(err, stackTrace)
			}
		}()

		goroutines.CallMain(func() {
			launcher.startContext(runtimeCtx)
		})

		for !gcontext.ShouldClose() {
			goroutines.CallMain(func() {
				launcher.drainEvents()

				if !gcontext.IsMinimized() {
					if !gcontext.IsFocused() {
						gcontext.SetSwapInterval(2)
					} else {
						gcontext.SetSwapInterval(1)
					}
				} else {
					gcontext.SetSwapInterval(int(launcher.refreshRate / 10))
				}
				launcher.Draw()
				gcontext.SwapBuffers()
				gcontext.HandleEvents()
			})
		}
	})

	launcher.shutdown()
}

func closeHandler(err any, stackTrace []string) {
	if err != nil {
		log.Println("panic:", err)

		for _, s := range stackTrace {
			log.Println(s)
		}

		showMessage(mError, "Launcher crashed with message:\n %s", err)

		os.Exit(1)
	}

	log.Println("Exiting normally.")
}

func (l *launcher) startContext(ctx context.Context) {
	if ctx == nil {
		panic("launcher runtime context is nil")
	}

	if err := gcontext.Initialize(false); err != nil {
		panic(err)
	}

	l.refreshRate = gcontext.GetPrimaryRefreshRate()

	l.tryCreateDefaultConfig()
	l.createConfigList()
	settings.LoadCredentials()

	l.bld.config = *launcherConfig.Profile

	c, err := l.loadConfig(l.bld.config)
	if err != nil {
		showMessage(mError, "Failed to read \"%s\" profile.\nReverting to \"default\".\nError: %s", l.bld.config, err)

		l.bld.config = "default"
		*launcherConfig.Profile = l.bld.config
		saveLauncherConfig()

		c, err = l.loadConfig(l.bld.config)
		if err != nil {
			panic(err)
		}
	}

	settings.General.OsuSongsDir = c.General.OsuSongsDir

	l.currentConfig = c

	settings.Graphics.Fullscreen = false
	settings.Graphics.WindowWidth = 800
	settings.Graphics.WindowHeight = 534

	iconName := "dansercoin*"

	if l.christmas {
		iconName += "-s"
	}

	if err := gcontext.SDLCreateWindow(800, 534, "danser-go "+build.VERSION+" launcher", gcontext.OptionalProps{
		IconName:       iconName,
		ScaleToMonitor: true,
		BuiltinMSAA:    true,
	}); err != nil {
		panic("Failed to create SDL window: " + err.Error())
	}

	log.Println("SDL initialized!")

	err = gcontext.GLInit(false)
	if err != nil {
		panic("Failed to initialize OpenGL: " + err.Error())
	}

	gcontext.SetSwapInterval(1)

	SetupImgui()

	graphics.LoadTextures()

	if l.winter {
		graphics.LoadWinterTextures()
	}

	bass.Init(false)

	l.triangleSpeed = animation.NewGlider(1)
	l.triangleSpeed.SetEasing(easing.OutQuad)

	l.batch = batch.NewQuadBatch()

	l.bg = common.NewBackground(false)

	settings.Playfield.Background.Triangles.Enabled = true

	if l.winter {
		imgui.PushStyleColorVec4(imgui.ColBorder, vec4(0.76, 0.9, 1, 1))

		settings.Playfield.Background.Triangles.Enabled = false

		l.snow = drawables.NewSnow()
	}

	if l.christmas {
		l.coin = common.NewDanserCoinSanta()
	} else {
		l.coin = common.NewDanserCoin()
	}

	l.coin.DrawVisualiser(true)

	l.processSupervisor = newProcessSupervisor(l)
	l.setupWatcher()

	gcontext.RegisterListener(func(event gcontext.DropEvent) {
		l.handleDrop(event)
	})

	gcontext.RegisterListener(func(_ gcontext.CloseEvent) {
		l.handleCloseRequest()
	})

	// The cached snapshot is published by the coordinator before the full
	// reconciliation completes. Startup paths that need a map wait for the
	// first completed reconciliation, while ordinary launcher browsing remains
	// available immediately.
	l.catalogCoordinator = newCatalogCoordinator(l)
	l.catalogCoordinator.start(ctx)
	l.reloadMaps(l.processStartupSelection)

	l.startBackgroundTasks(ctx)

	gcontext.RegisterListener(func(event gcontext.KeyEvent) {
		if l.currentEditor != nil {
			l.currentEditor.updateKey(event)
		}
	})
}

// startBackgroundTasks contains startup work that may perform network or
// native-library I/O. The worker reports only immutable results; dialogs and
// any other launcher-facing behavior remain on the main thread.
func (l *launcher) startBackgroundTasks(ctx context.Context) {
	if ctx == nil {
		return
	}

	checkUpdates := launcherConfig.CheckForUpdates

	l.backgroundWG.Add(1)
	go func() {
		defer l.backgroundWG.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("Launcher: Background startup task panicked: %v", recovered)
			}
		}()

		settings.DefaultsFactory.EncoderOptions()

		if checkUpdates {
			status, url, err := appUtils.CheckForUpdateContext(ctx)
			if !l.postEventContext(ctx, launcherEvent{
				kind: launcherStartupTaskEvent,
				task: func() { showUpdateResult(status, url, err, false) },
			}) {
				return
			}
		}

		if refreshErr := osuapi.TryRefreshTokenContext(ctx); refreshErr != nil && ctx.Err() == nil {
			l.postEventContext(ctx, launcherEvent{
				kind: launcherStartupTaskEvent,
				task: func() {
					showMessage(mError, "Failed to refresh token!\nPlease go to Settings->Credentials and click Authorize.\nError: %s", refreshErr)
				},
			})
		}
	}()
}

func (l *launcher) loadBeatmaps(after func()) {
	if l.catalogCoordinator == nil {
		return
	}

	l.catalogCoordinator.request(after)
}

func (l *launcher) catalogImportListener() database.ImportListener {
	return l.catalogImportListenerFor(l.catalogGeneration.Load())
}

func (l *launcher) catalogImportListenerFor(generation uint64) database.ImportListener {
	prominent := false

	return func(stage database.ImportStage, processed, target int) {
		if generation != 0 && generation != l.catalogGeneration.Load() {
			return
		}

		// Store every active stage so song select can explain why its catalog is
		// changing. The main launcher keeps small refreshes unobtrusive through
		// the separate prominent flag.
		switch stage {
		case database.Discovery:
			if processed >= catalogProgressVisibilityThreshold {
				prominent = true
			}
			l.catalogProgress.Store(catalogProgressState{
				generation: generation,
				stage:      stage,
				processed:  processed,
				active:     true,
				prominent:  prominent,
			})
		case database.Comparison:
			l.catalogProgress.Store(catalogProgressState{
				generation: generation,
				stage:      database.Comparison,
				active:     true,
				prominent:  prominent,
			})
		case database.Import, database.Cleanup:
			if target >= catalogProgressVisibilityThreshold {
				prominent = true
			}
			l.catalogProgress.Store(catalogProgressState{
				generation: generation,
				stage:      stage,
				processed:  processed,
				target:     target,
				active:     true,
				prominent:  prominent,
			})
		}
	}
}

// clearCatalogProgress publishes the idle state after reconciliation. Keeping
// this as a named transition makes it harder for a future early return to
// leave stale worker status in the launcher UI.
func (l *launcher) clearCatalogProgress() {
	l.catalogProgress.Store(catalogProgressState{})
}

func (l *launcher) clearCatalogProgressFor(generation uint64) {
	value := l.catalogProgress.Load()
	if value == nil {
		return
	}

	progress, ok := value.(catalogProgressState)
	if ok && (progress.generation == 0 || progress.generation == generation) {
		l.clearCatalogProgress()
	}
}

func (l *launcher) catalogStarRatingListener() func(processed, target int, message string) {
	return l.catalogStarRatingListenerFor(l.catalogGeneration.Load())
}

func (l *launcher) catalogStarRatingListenerFor(generation uint64) func(processed, target int, message string) {
	return func(processed, target int, _ string) {
		if generation != 0 && generation != l.catalogGeneration.Load() {
			return
		}

		l.catalogProgress.Store(catalogProgressState{
			generation: generation,
			stage:      database.StarRating,
			processed:  processed,
			target:     target,
			active:     true,
			prominent:  target >= catalogProgressVisibilityThreshold,
		})
	}
}

func (l *launcher) materializeCatalogEntry(entry *database.BeatmapEntry) (*beatmap.BeatMap, error) {
	bMap, err := database.LoadRuntimeBeatMap(entry)
	if err != nil {
		// A cached row can outlive its source file on an external or actively
		// edited Songs drive. Keep the cached selector usable, but ask the
		// background coordinator to reconcile the stale row instead of mutating
		// the database from this UI callback.
		l.reloadMaps(nil)
		return nil, err
	}
	if bMap == nil {
		l.reloadMaps(nil)
		return nil, errors.New("catalog returned an empty beatmap")
	}

	if entry != nil && (entry.LastModified != bMap.LastModified || entry.FileSize != bMap.FileSize || !strings.EqualFold(entry.MD5, bMap.MD5)) {
		// Runtime selection verifies the file lazily. A changed file is still
		// playable now, while the asynchronous reconciliation refreshes search
		// metadata and the durable fingerprint for the next access.
		l.reloadMaps(nil)
	}

	return bMap, nil
}

func (l *launcher) findCatalogEntryByMD5(md5 string) *database.BeatmapEntry {
	if l.catalog == nil || md5 == "" {
		return nil
	}

	var found *database.BeatmapEntry
	l.catalog.ForEach(func(entry *database.BeatmapEntry) bool {
		if strings.EqualFold(entry.MD5, md5) {
			found = entry
			return false
		}
		return true
	})

	return found
}

func (l *launcher) loadLatestReplay() {
	if l.currentConfig == nil {
		return
	}

	replaysDir := l.currentConfig.General.GetReplaysDir()

	type lastModPath struct {
		tStamp time.Time
		name   string
	}

	var list []lastModPath

	entries, err := os.ReadDir(replaysDir)
	if err != nil {
		return
	}

	for _, d := range entries {
		if !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".osr") {
			if info, err1 := d.Info(); err1 == nil {
				list = append(list, lastModPath{
					tStamp: info.ModTime(),
					name:   d.Name(),
				})
			}
		}
	}

	if list == nil {
		return
	}

	slices.SortFunc(list, func(a, b lastModPath) int {
		return -a.tStamp.Compare(b.tStamp)
	})

	// Load the newest that can be used
	for _, replayPath := range list {
		r, err := l.loadReplay(filepath.Join(replaysDir, replayPath.name))
		if err == nil {
			l.trySelectReplay(r)
			break
		}
	}
}

func (l *launcher) Draw() {
	w, h := gcontext.GetFramebufferSize() //l.win.GetFramebufferSize()
	viewport.Push(w, h)

	if l.bg.HasBackground() {
		gl.ClearColor(0, 0, 0, 1.0)
	} else {
		gl.ClearColor(0.1, 0.1, 0.1, 1.0)
	}

	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.Enable(gl.SCISSOR_TEST)

	w, h = int(settings.Graphics.WindowWidth), int(settings.Graphics.WindowHeight)

	settings.Graphics.Fullscreen = false
	settings.Graphics.WindowWidth = int64(w)
	settings.Graphics.WindowHeight = int64(h)

	if l.currentConfig != nil {
		settings.Audio.GeneralVolume = l.currentConfig.Audio.GeneralVolume
		settings.Audio.MusicVolume = l.currentConfig.Audio.MusicVolume
		settings.Gameplay.AlwaysSkipIntro = l.currentConfig.Gameplay.AlwaysSkipIntro
	}

	t := qpc.GetMilliTimeF()

	l.triangleSpeed.Update(t)

	settings.Playfield.Background.Triangles.Speed = l.triangleSpeed.GetValue()

	pX := (float64(imgui.MousePos().X) * 2 / settings.Graphics.GetWidthF()) - 1
	pY := (float64(imgui.MousePos().Y) * 2 / settings.Graphics.GetHeightF()) - 1

	l.bg.Update(t, -pX, pY)

	l.batch.SetCamera(mgl32.Ortho(-float32(w)/2, float32(w)/2, float32(h)/2, -float32(h)/2, -1, 1))

	bgA := 1.0
	if l.bg.HasBackground() {
		bgA = 0.33
	}

	l.bg.Draw(t, l.batch, 0, bgA, l.batch.Projection)

	l.batch.SetColor(1, 1, 1, 1)
	l.batch.ResetTransform()
	l.batch.SetCamera(mgl32.Ortho(0, float32(w), float32(h), 0, -1, 1))

	l.batch.Begin()

	if l.winter {
		l.snow.Update(t)
		l.snow.Draw(t, l.batch)
	}

	if l.catalog != nil {
		if l.winter {
			bSnow := *graphics.Snow[0]

			if l.showProgressBar {
				bSnow = *graphics.Snow[5]
			}

			l.batch.DrawStObject(vector.NewVec2d(0, settings.Graphics.GetHeightF()), vector.BottomLeft, vector.NewVec2d(1, 1), false, false, 0, color2.NewL(1), false, bSnow)

			//record button
			if launcherConfig.CurrentMode != Play {
				l.batch.DrawStObject(l.recordSnowPos.Copy64().AddS(0, 2), vector.BottomCentre, vector.NewVec2d(1, 1), false, false, 0, color2.NewL(1), false, *graphics.Snow[2])
			}

			//danse button
			l.batch.DrawStObject(vector.NewVec2d(624, 448), vector.BottomCentre, vector.NewVec2d(1, 1), false, false, 0, color2.NewL(1), false, *graphics.Snow[1])

			l.batch.DrawStObject(vector.NewVec2d(115, 240), vector.BottomCentre, vector.NewVec2d(1, 1), false, false, 0, color2.NewL(1), false, *graphics.Snow[4])

			if launcherConfig.CurrentMode != Replay {
				l.batch.DrawStObject(vector.NewVec2d(314, 240), vector.BottomCentre, vector.NewVec2d(1, 1), false, false, 0, color2.NewL(1), false, *graphics.Snow[3])
			}

		}

		if l.bld.currentMap != nil && l.prevMap != l.bld.currentMap {
			l.bg.SetBeatmap(l.bld.currentMap, false, false)

			l.prevMap = l.bld.currentMap
		}

		if l.selectWindow != nil {
			if l.selectWindow.PreviewedSong != nil {
				l.selectWindow.PreviewedSong.Update()
				l.coin.SetMap(l.selectWindow.prevMap, l.selectWindow.PreviewedSong)
				l.bg.SetTrack(l.selectWindow.PreviewedSong)
			} else {
				l.coin.SetMap(nil, nil)
				l.bg.SetTrack(nil)
			}
		}

		l.coin.SetPosition(vector.NewVec2d(468+155.5, 180+85))

		if l.christmas {
			l.coin.SetScale(float64(h) / 5)
		} else {
			l.coin.SetScale(float64(h) / 4)
		}

		l.coin.SetRotation(0.1)

		l.coin.Update(t)
		l.coin.Draw(t, l.batch)
	}

	l.batch.End()

	l.drawImgui()

	viewport.Pop()
}

func (l *launcher) drawImgui() {
	Begin()

	resetPopupHierarchyInfo()

	lock := l.danserRunning || l.danserStartPending

	if lock {
		imgui.PushItemFlag(imgui.ItemFlags(imgui.ItemFlagsDisabled), true)
	}

	wW, wH := int(settings.Graphics.WindowWidth), int(settings.Graphics.WindowHeight)

	imgui.SetNextWindowSize(vec2(float32(wW), float32(wH)))

	imgui.SetNextWindowPos(vzero())

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, vec2(20, 20))

	imgui.BeginV("main", nil, imgui.WindowFlagsNoDecoration /*|imgui.WindowFlagsNoMove*/ |imgui.WindowFlagsNoBackground|imgui.WindowFlagsNoScrollWithMouse|imgui.WindowFlagsNoBringToFrontOnFocus)

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, vec2(5, 5))

	l.drawMain()

	imgui.PopStyleVar()

	imgui.End()

	imgui.PopStyleVar()

	if lock {
		imgui.PopItemFlag()
	}

	DrawImgui()
}

func (l *launcher) drawMain() {
	w := contentRegionMax().X

	imgui.PushFont(Font, 24)

	if imgui.BeginTableV("ltpanel", 2, imgui.TableFlagsSizingStretchProp, vec2(float32(w)/2, 0), -1) {
		imgui.TableSetupColumnV("ltpanel1", imgui.TableColumnFlagsWidthFixed, 0, imgui.ID(0))
		imgui.TableSetupColumnV("ltpanel2", imgui.TableColumnFlagsWidthStretch, 0, imgui.ID(1))

		imgui.TableNextColumn()

		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted("Mode:")

		imgui.TableNextColumn()

		imgui.SetNextItemWidth(-1)

		if imgui.BeginCombo("##mode", launcherConfig.CurrentMode.String()) {
			for _, m := range modes {
				if imgui.SelectableBoolV(m.String(), launcherConfig.CurrentMode == m, 0, vzero()) {
					if m == Play {
						launcherConfig.CurrentPMode = Watch
					}

					if m != Replay {
						l.bld.replayPath = ""
						l.bld.removeReplay()
					}

					if m != Knockout {
						l.bld.knockoutReplays = nil
					}

					launcherConfig.CurrentMode = m
				}
			}

			imgui.EndCombo()
		}

		imgui.EndTable()
	}

	l.drawConfigPanel()

	l.drawControls()

	l.drawLowerPanel()

	if l.selectWindow != nil {
		l.selectWindow.update()
	}

	for i := 0; i < len(l.popupStack); i++ {
		p := l.popupStack[i]
		p.draw()
		if p.shouldClose() {
			l.popupStack = append(l.popupStack[:i], l.popupStack[i+1:]...)
			i--
		}
	}

	imgui.PopFont()

	if imgui.IsMouseClickedBool(0) && !l.danserRunning {
		gcontext.StopProgress()
		l.showProgressBar = false
		l.recordStatus = ""
		l.recordProgress = 0
	}

	if !l.danserRunning && l.beatmapDirUpdated && qpc.GetMilliTimeF() >= l.showBeatmapAlert {
		reload := launcherConfig.AutoRefreshDB

		if !reload {
			mapText := "Do you want to refresh the database?"
			if launcherConfig.SkipMapUpdate {
				mapText = "Do you want to load new beatmap sets?"
			}

			reload = showMessage(mQuestion, "%s", "Changes in osu!'s Song directory have been detected.\n\n"+mapText)
		}

		l.beatmapDirUpdated = false

		if reload {
			l.reloadMaps(nil)
		}
	}
}

func (l *launcher) drawCatalogProgress() {
	progress := l.currentCatalogProgress()
	if !progress.active || !progress.prominent {
		return
	}

	message := catalogProgressMessage(progress)
	if message == "" {
		return
	}

	imgui.PushFont(Font, 24)
	imgui.TextUnformatted(message)
	imgui.PopFont()
	imgui.Dummy(vec2(0, 4))
}

func (l *launcher) currentCatalogProgress() catalogProgressState {
	if l == nil {
		return catalogProgressState{}
	}

	value := l.catalogProgress.Load()
	if value == nil {
		return catalogProgressState{}
	}

	progress, ok := value.(catalogProgressState)
	if !ok || progress.generation != 0 && progress.generation != l.catalogGeneration.Load() {
		return catalogProgressState{}
	}

	return progress
}

func catalogProgressMessage(progress catalogProgressState) string {
	if !progress.active {
		return ""
	}

	switch progress.stage {
	case database.Discovery:
		return fmt.Sprintf("Scanning beatmaps: %d directories", progress.processed)
	case database.Comparison:
		return "Comparing beatmaps..."
	case database.Import:
		return fmt.Sprintf("Indexing beatmaps: %d / %d", progress.processed, progress.target)
	case database.Cleanup:
		return fmt.Sprintf("Removing beatmaps: %d / %d", progress.processed, progress.target)
	case database.StarRating:
		return fmt.Sprintf("Updating star ratings: %d / %d", progress.processed, progress.target)
	default:
		return ""
	}
}

func (l *launcher) drawControls() {
	imgui.SetCursorPos(vec2(20, 88))
	switch launcherConfig.CurrentMode {
	case Replay:
		l.selectReplay()
	case Knockout:
		l.newKnockout()
	default:
		l.showSelect()
	}

	imgui.SetCursorPos(vec2(20, 204+34))

	w := contentRegionMax().X

	if imgui.BeginTableV("abtn", 2, imgui.TableFlagsSizingStretchSame, vec2(float32(w)/2, -1), -1) {
		imgui.TableNextColumn()

		if imgui.ButtonV("Speed/Pitch", vec2(-1, imgui.TextLineHeight()*2)) {
			l.openPopup(newPopupF("Speed adjust", popMedium, func() {
				drawSpeedMenu(l.bld)
			}))
		}

		imgui.TableNextColumn()

		nilMap := l.bld.currentMap == nil

		if nilMap {
			imgui.BeginDisabled()
		}

		if imgui.ButtonV("Mods", vec2(-1, imgui.TextLineHeight()*2)) {
			l.openPopup(newModPopup(l.bld))
		}

		if nilMap && imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenDisabled) {
			imgui.SetTooltip("Select map/replay first")
		}

		imgui.TableNextColumn()

		if imgui.ButtonV("Time/Offset", vec2(-1, imgui.TextLineHeight()*2)) {
			if l.timeMenu == nil {
				l.timeMenu = newTimePopup(l.bld)

				l.timeMenu.setCloseListener(func() {
					if l.bld.currentMap != nil && l.bld.currentMap.LocalOffset != int(l.bld.offset.value) {
						l.bld.currentMap.LocalOffset = int(l.bld.offset.value)
						database.UpdateLocalOffset(l.bld.currentMap)
					}
				})
			}

			l.openPopup(l.timeMenu)
		}

		if nilMap {
			imgui.EndDisabled()
			if imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenDisabled) {
				imgui.SetTooltip("Select map/replay first")
			}
		}

		imgui.TableNextColumn()

		if usesGeneratedCursors(launcherConfig.CurrentMode) {
			if imgui.ButtonV("Mirrors/Tags", vec2(-1, imgui.TextLineHeight()*2)) {
				l.openPopup(newPopupF("Difficulty adjust", popDynamic, func() {
					drawCDMenu(l.bld)
				}))
			}
		}

		imgui.EndTable()
	}

}

func (l *launcher) selectReplay() {
	bSize := vec2((imgui.WindowWidth()-40)/4, imgui.TextLineHeight()*2)

	imgui.PushFont(Font, 32)

	if imgui.ButtonV("Select replay", bSize) {
		dir := l.currentConfig.General.GetReplaysDir()
		if _, err := os.Lstat(dir); err != nil {
			dir = env.DataDir()
		}

		showFilePicker("Select replay file", []string{"osr"}, dir, false, false, func(paths []string, err error) {
			if err == nil && len(paths) > 0 {
				l.trySelectReplayFromPath(paths[0])
			}
		})
	}

	imgui.PopFont()

	imgui.PushFont(Font, 20)
	imgui.IndentV(5)

	if l.bld.currentReplay != nil && l.bld.currentMap != nil {
		b := l.bld.currentMap

		mString := fmt.Sprintf("%s - %s [%s]\nPlayed by: %s", b.Artist, b.Name, b.Difficulty, l.bld.currentReplay.Username)

		imgui.PushTextWrapPosV(contentRegionMax().X / 2)
		imgui.TextUnformatted(mString)
		imgui.PopTextWrapPos()
	} else {
		imgui.TextUnformatted("No replay selected")
	}

	imgui.UnindentV(5)
	imgui.PopFont()
}

func (l *launcher) trySelectReplayFromPath(p string) {
	replay, err := l.loadReplay(p)

	if err != nil {
		message := err.Error()
		runes := []rune(message)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			message = string(runes)
		}
		showMessage(mError, "%s", message)
		return
	}

	l.trySelectReplay(replay)
}

func (l *launcher) trySelectReplaysFromPaths(p []string) {
	var errorCollection string
	var replays []*knockoutReplay

	for _, rPath := range p {
		replay, err := l.loadReplay(rPath)

		if err != nil {
			if errorCollection != "" {
				errorCollection += "\n"
			}

			errorCollection += fmt.Sprintf("%s:\n\t%s", filepath.Base(rPath), err)
		} else {
			replays = append(replays, replay)
		}
	}

	if errorCollection != "" {
		showMessage(mError, "There were errors opening replays:\n%s", errorCollection)
	}

	if replays != nil && len(replays) > 0 {
		found := false

		for _, replay := range replays {
			entry := l.findCatalogEntryByMD5(replay.parsedReplay.BeatmapMD5)
			if entry != nil {
				bMap, err := l.materializeCatalogEntry(entry)
				if err != nil {
					showMessage(mError, "Failed to load replay map: %s", err)
					break
				}

				launcherConfig.CurrentMode = Knockout
				l.bld.setMap(bMap)
				found = true
			}

			if found {
				break
			}
		}

		if !found {
			showMessage(mError, "Replays use an unknown map. Please download the map beforehand.")
		} else {
			var finalReplays []*knockoutReplay

			for _, replay := range replays {
				if strings.ToLower(l.bld.currentMap.MD5) == strings.ToLower(replay.parsedReplay.BeatmapMD5) {
					finalReplays = append(finalReplays, replay)
				}
			}

			slices.SortFunc(finalReplays, func(a, b *knockoutReplay) int {
				return -cmp.Compare(a.parsedReplay.Score, b.parsedReplay.Score)
			})

			l.bld.knockoutReplays = finalReplays
			l.knockoutManager = newKnockoutManagerPopup(l.bld)
		}
	}
}

func (l *launcher) trySelectReplay(replay *knockoutReplay) {
	if replay == nil || replay.parsedReplay == nil {
		showMessage(mError, "Replay data is empty.")
		return
	}

	entry := l.findCatalogEntryByMD5(replay.parsedReplay.BeatmapMD5)
	if entry != nil {
		bMap, err := l.materializeCatalogEntry(entry)
		if err != nil {
			showMessage(mError, "Failed to load replay map: %s", err)
			return
		}

		launcherConfig.CurrentMode = Replay
		l.bld.replayPath = replay.path
		l.bld.setMap(bMap)
		l.bld.setReplay(replay.parsedReplay)

		return
	}

	showMessage(mError, "Replay uses an unknown map. Please download the map beforehand.")
}

func (l *launcher) newKnockout() {
	bSize := vec2((imgui.WindowWidth()-40)/4, imgui.TextLineHeight()*2)

	imgui.PushFont(Font, 32)

	if imgui.ButtonV("Select replays", bSize) {
		kPath := getAbsPath(launcherConfig.LastKnockoutPath)

		if _, err := os.Lstat(kPath); err != nil {
			kPath = env.DataDir()
		}

		showFilePicker("Select replay files", []string{"osr"}, kPath, true, false, func(p []string, err error) {
			if err == nil && len(p) > 0 {
				launcherConfig.LastKnockoutPath = getRelativeOrABSPath(filepath.Dir(p[0]))
				saveLauncherConfig()

				l.trySelectReplaysFromPaths(p)
			}
		})
	}

	imgui.PopFont()

	imgui.PushFont(Font, 20)

	imgui.IndentV(5)

	if l.bld.knockoutReplays != nil && l.bld.currentMap != nil {
		b := l.bld.currentMap

		imgui.PushTextWrapPosV(contentRegionMax().X / 2)

		imgui.TextUnformatted(fmt.Sprintf("%s - %s [%s]", b.Artist, b.Name, b.Difficulty))

		imgui.AlignTextToFramePadding()

		imgui.TextUnformatted(fmt.Sprintf("%d replays loaded", len(l.bld.knockoutReplays)))

		imgui.PopTextWrapPos()

		imgui.SameLine()

		if imgui.Button("Manage##knockout") && l.knockoutManager != nil {
			l.openPopup(l.knockoutManager)
		}
	} else {
		imgui.TextUnformatted("No replays selected")
	}

	imgui.UnindentV(5)

	imgui.PopFont()
}

func (l *launcher) loadReplay(p string) (*knockoutReplay, error) {
	if !strings.EqualFold(filepath.Ext(p), ".osr") {
		return nil, fmt.Errorf("it's not a replay file")
	}

	rData, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	replay, err := rplpa.ParseReplay(rData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse replay: %w", err)
	}
	if replay == nil {
		return nil, errors.New("failed to parse replay: empty replay")
	}

	if replay.PlayMode != 0 {
		return nil, errors.New("only osu!standard mode is supported")
	}

	if replay.ReplayData == nil || len(replay.ReplayData) < 2 {
		return nil, errors.New("replay is missing input data")
	}

	// dump unneeded data as it's not needed anymore to save memory
	replay.LifebarGraph = nil
	replay.ReplayData = nil

	diff := difficulty.NewDifficulty(5, 5, 5, 5)
	diff.SetGameplayMode(difficulty.GameplayModeFromReplayVersion(int(replay.OsuVersion)))

	if replay.ScoreInfo != nil && replay.ScoreInfo.Mods != nil && len(replay.ScoreInfo.Mods) > 0 {
		modsNew := make([]rplpa.ModInfo, 0, len(replay.ScoreInfo.Mods))

		for _, mod := range replay.ScoreInfo.Mods {
			if mod != nil {
				modsNew = append(modsNew, *mod)
			}
		}

		diff.SetMods2(modsNew)
	} else {
		diff.SetMods(difficulty.Modifier(replay.Mods))
	}

	return &knockoutReplay{
		path:         p,
		parsedReplay: replay,
		included:     true,
		mods:         diff.Mods,
	}, nil
}

func (l *launcher) showSelect() {
	bSize := vec2((imgui.WindowWidth()-40)/4, imgui.TextLineHeight()*2)

	imgui.PushFont(Font, 32)

	enabled, disabledReason := l.mapSelectionAvailability()
	if !enabled {
		imgui.BeginDisabled()
	}
	clicked := imgui.ButtonV("Select map", bSize)
	if !enabled {
		imgui.EndDisabled()
		if imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenDisabled) {
			imgui.SetTooltip(disabledReason)
		}
	}
	if clicked && enabled {
		l.selectWindow.open()
		l.openPopup(l.selectWindow)
	}

	imgui.PopFont()

	imgui.PushFont(Font, 20)

	imgui.IndentV(5)

	if l.bld.currentMap != nil {
		b := l.bld.currentMap

		mString := fmt.Sprintf("%s - %s [%s]", b.Artist, b.Name, b.Difficulty)

		imgui.PushTextWrapPosV(contentRegionMax().X / 2)
		imgui.TextUnformatted(mString)
		imgui.PopTextWrapPos()
	} else {
		imgui.TextUnformatted("No map selected")
	}

	imgui.UnindentV(5)

	imgui.PopFont()
}

func (l *launcher) ensureSongSelect() *songSelectPopup {
	if l.selectWindow == nil {
		l.selectWindow = newSongSelectPopup(l.bld, l.catalog, l.materializeCatalogEntry, l.currentCatalogProgress)
	}

	return l.selectWindow
}

func (l *launcher) mapSelectionAvailability() (bool, string) {
	if !l.catalogSnapshotReady.Load() {
		return false, "Loading map catalog..."
	}
	if l.catalog == nil || l.catalog.Len() == 0 {
		if progress := l.currentCatalogProgress(); progress.active {
			return false, "Scanning beatmaps..."
		}
		return false, "No beatmaps found in the configured Songs folder."
	}
	if l.selectWindow == nil || !l.selectWindow.hasSelectableMaps() {
		return false, "Preparing map list..."
	}

	return true, ""
}

func (l *launcher) drawLowerPanel() {
	w, h := contentRegionMax().X, contentRegionMax().Y

	showProgress := launcherConfig.CurrentPMode == Record && l.showProgressBar
	frameHeight := imgui.FrameHeightWithSpacing()
	outputSpacing := frameHeight
	if showProgress {
		outputSpacing *= 2
	}

	if launcherConfig.CurrentMode == Play {
		// Play mode intentionally has no output selector. Reuse that row's
		// coordinates for catalog progress so the status remains aligned with
		// every other mode without adding a separate header layout.
		imgui.SetCursorPos(vec2(20, h-frameHeight))
		l.drawCatalogProgress()
	} else {
		// Reconciliation status belongs immediately above the output selector.
		// Recording reserves one additional row for the taskbar/progress bar.
		imgui.SetCursorPos(vec2(20, h-outputSpacing-frameHeight))
		l.drawCatalogProgress()

		imgui.SetCursorPos(vec2(20, h-outputSpacing))
		imgui.SetNextItemWidth((imgui.WindowWidth() - 40) / 4)

		l.recordSnowPos = vector.NewVec2f(20+(imgui.WindowWidth()-40)/4/2, h-outputSpacing-2)

		if imgui.BeginCombo("##Watch mode", launcherConfig.CurrentPMode.String()) {
			for _, m := range pModes {
				if imgui.SelectableBoolV(m.String(), launcherConfig.CurrentPMode == m, 0, vzero()) {
					launcherConfig.CurrentPMode = m
				}
			}

			imgui.EndCombo()
		}

		if launcherConfig.CurrentPMode != Watch {
			imgui.SameLine()
			if imgui.Button("Configure") {
				l.openPopup(newPopupF("Record settings", popDynamic, func() {
					drawRecordMenu(l.bld)
				}))
			}
		}

		imgui.SetCursorPos(vec2(contentRegionMin().X, h-frameHeight))

		if showProgress {
			if strings.HasPrefix(l.recordStatus, "Done") {
				imgui.PushStyleColorVec4(imgui.ColPlotHistogram, imgui.Vec4{
					X: 0.16,
					Y: 0.75,
					Z: 0.18,
					W: 1,
				})
			} else {
				imgui.PushStyleColorVec4(imgui.ColPlotHistogram, *imgui.StyleColorVec4(imgui.ColCheckMark))
			}

			imgui.ProgressBarV(l.recordProgress, vec2(w/2, imgui.FrameHeight()), l.recordStatus)

			if l.encodeInProgress {
				imgui.PushFont(Font, 16)

				cPos := imgui.CursorPos()

				imgui.TextUnformatted(l.recordStatusSpeed)

				cPos.X += 95

				imgui.SetCursorPos(cPos)

				eta := int(time.Since(l.encodeStart).Seconds())

				imgui.TextUnformatted("| Elapsed: " + util.FormatSeconds(eta))

				cPos.X += 135

				imgui.SetCursorPos(cPos)

				imgui.TextUnformatted("| " + l.recordStatusETA)

				imgui.PopFont()
			}

			imgui.PopStyleColor()
		}
	}

	fHwS := imgui.FrameHeightWithSpacing()*2 - imgui.CurrentStyle().FramePadding().X

	bW := (w) / 4

	imgui.SetCursorPos(vec2(contentRegionMax().X-w/2.5, h-imgui.FrameHeightWithSpacing()*2))

	centerTable("dansebutton", w/2.5, func() {
		imgui.PushFont(Font, 48)
		{
			dRun := l.danserRunning && launcherConfig.CurrentPMode == Record

			s := l.bld.launchDisabled()

			if dRun {
				// drawImgui disables the entire interface while a child is
				// running. Temporarily remove that outer flag only for CANCEL.
				imgui.PopItemFlag()
			} else if s {
				imgui.PushItemFlag(imgui.ItemFlags(imgui.ItemFlagsDisabled), true)
			}

			name := "danse!"
			if dRun {
				name = "CANCEL"
			}

			if imgui.ButtonV(name, vec2(bW, fHwS)) {
				if dRun {
					if l.processSupervisor != nil && showMessage(mQuestion, "Do you really want to cancel?") {
						l.processSupervisor.stop()
					}
				} else {
					if l.selectWindow != nil {
						l.selectWindow.stopPreview()
					}

					log.Println(l.bld.getArguments())

					l.triangleSpeed.AddEventS(l.triangleSpeed.GetTime(), l.triangleSpeed.GetTime()+1000, 50, 1)

					if launcherConfig.CurrentPMode != Watch {
						l.startDanser()
					} else {
						l.scheduleDanserStart()
					}
				}
			}

			if dRun {
				imgui.PushItemFlag(imgui.ItemFlags(imgui.ItemFlagsDisabled), true)
			} else if s {
				imgui.PopItemFlag()
			}

			imgui.PopFont()
		}
	})
}

func (l *launcher) drawConfigPanel() {
	if l.currentEditor != nil {
		l.currentEditor.setDanserRunning(l.danserRunning && launcherConfig.CurrentPMode == Watch)
	}

	w := contentRegionMax().X

	imgui.SetCursorPos(vec2(contentRegionMax().X-float32(w)/2.5, 20))

	if imgui.BeginTableV("rtpanel", 2, imgui.TableFlagsSizingStretchProp, vec2(float32(w)/2.5, 0), -1) {
		imgui.TableSetupColumnV("rtpanel1", imgui.TableColumnFlagsWidthStretch, 0, 0)
		imgui.TableSetupColumnV("rtpanel2", imgui.TableColumnFlagsWidthFixed, 0, 1)

		imgui.TableNextColumn()

		if imgui.ButtonV("Launcher settings", vec2(-1, 0)) {
			wSize := imgui.WindowSize()

			lEditor := newPopupF("About", popCustom, drawLauncherConfig)
			lEditor.width = wSize.X / 2
			lEditor.height = wSize.Y * 0.9

			lEditor.setCloseListener(func() {
				saveLauncherConfig()
			})

			l.openPopup(lEditor)
		}

		imgui.TableNextColumn()

		if imgui.Button("About") {
			l.openPopup(newPopupF("About", popDynamic, func() {
				drawAbout(l.coin.Texture.Texture)
			}))
		}

		imgui.TableNextColumn()

		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted("Config:")

		imgui.SameLine()

		imgui.SetNextItemWidth(-1)

		mWidth := imgui.CalcItemWidth() - imgui.CurrentStyle().FramePadding().X*2

		if imgui.BeginComboV("##config", l.bld.config, imgui.ComboFlagsHeightLarge) {
			for _, s := range l.configList {
				mWidth = max(mWidth, imgui.CalcTextSizeV(s, false, 0).X+20)
			}

			imgui.SetNextItemWidth(mWidth)

			focusScroll := searchBox("##configSearch", &l.configSearch)

			if !imgui.IsMouseClickedBool(0) && !imgui.IsMouseClickedBool(1) && !imgui.IsAnyItemActive() && !l.configEditOpened && !l.configScrolling {
				imgui.SetKeyboardFocusHereV(-1)
			}

			if imgui.SelectableBool("Create new...") {
				l.newCloneOpened = true
				l.configManiMode = New
			}

			imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, 0)
			imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
			imgui.PushStyleVarVec2(imgui.StyleVarFramePadding, vzero())
			imgui.PushStyleColorVec4(imgui.ColFrameBg, imgui.Vec4{X: 0, Y: 0, Z: 0, W: 0})

			searchResults := make([]string, 0, len(l.configList))

			search := strings.ToLower(l.configSearch)

			for _, s := range l.configList {
				if l.configSearch == "" || strings.Contains(strings.ToLower(s), search) {
					searchResults = append(searchResults, s)
				}
			}

			if len(searchResults) > 0 {
				sHeight := float32(min(8, len(searchResults)))*imgui.FrameHeightWithSpacing() - imgui.CurrentStyle().ItemSpacing().Y/2

				if imgui.BeginListBoxV("##blistbox", vec2(mWidth, sHeight)) {
					l.configScrolling = handleDragScroll()
					focusScroll = focusScroll || imgui.IsWindowAppearing()

					for _, s := range searchResults {
						if selectableFocus(s, s == l.bld.config, focusScroll) {
							if s != l.bld.config {
								l.setConfig(s)
							}
						}

						if _, ok := l.cHold[s]; !ok {
							l.cHold[s] = new(bool)
						}

						if imgui.IsMouseClickedBool(1) && imgui.IsItemHovered() {
							*l.cHold[s] = true
							l.configEditOpened = true

							imgui.SetNextWindowPosV(imgui.MousePos(), imgui.CondAlways, vzero())

							imgui.OpenPopupStr("##context" + s)
						}

						befHold := *l.cHold[s]

						if imgui.BeginPopupModalV("##context"+s, l.cHold[s], imgui.WindowFlagsNoCollapse|imgui.WindowFlagsNoResize|imgui.WindowFlagsAlwaysAutoResize|imgui.WindowFlagsNoMove|imgui.WindowFlagsNoTitleBar) {
							if s != "default" {
								if imgui.SelectableBool("Rename") {
									l.newCloneOpened = true
									l.configPrevName = s
									l.configManiMode = Rename
								}
							}

							if imgui.SelectableBool("Clone") {
								l.newCloneOpened = true
								l.configPrevName = s
								l.configManiMode = Clone
							}

							if s != "default" {
								if imgui.SelectableBool("Remove") {
									if showMessage(mQuestion, "Are you sure you want to remove \"%s\" profile?", s) {
										l.removeConfig(s)
									}
								}
							}

							if (imgui.IsMouseClickedBool(0) || imgui.IsMouseClickedBool(1)) && !imgui.IsWindowHoveredV(imgui.HoveredFlagsRootAndChildWindows|imgui.HoveredFlagsAllowWhenBlockedByActiveItem|imgui.HoveredFlagsAllowWhenBlockedByPopup) {
								*l.cHold[s] = false
								//imgui.CloseCurrentPopup()
							}

							imgui.EndPopup()
						}

						if befHold && !*l.cHold[s] {
							l.configEditOpened = false
						}
					}

					imgui.EndListBox()
				}
			}

			imgui.PopStyleVar()
			imgui.PopStyleVar()
			imgui.PopStyleVar()
			imgui.PopStyleColor()

			imgui.EndCombo()
		}

		imgui.TableNextColumn()

		dRun := l.danserRunning && launcherConfig.CurrentPMode == Watch

		if dRun {
			imgui.PushItemFlag(imgui.ItemFlags(imgui.ItemFlagsDisabled), false)
		}

		if imgui.ButtonV("Edit", vec2(-1, 0)) {
			l.openCurrentSettingsEditor()
		}

		if dRun {
			imgui.PopItemFlag()
		}

		imgui.EndTable()
	}

	if l.newCloneOpened {
		popupSmall("Clone/new box", &l.newCloneOpened, true, 0, 0, func() {
			if imgui.BeginTable("rfa", 1) {
				imgui.TableNextColumn()

				imgui.TextUnformatted("Name:")

				imgui.SameLine()

				imgui.SetNextItemWidth(imgui.TextLineHeight() * 10)

				if inputTextV("##nclonename", &l.newCloneName, imgui.InputTextFlagsCallbackCharFilter, imguiPathFilter) {
					l.newCloneName = strings.TrimSpace(l.newCloneName)
				}

				if !imgui.IsAnyItemActive() && !imgui.IsMouseClickedBool(0) {
					imgui.SetKeyboardFocusHereV(-1)
				}

				imgui.TableNextColumn()

				cPos := imgui.CursorPos()

				imgui.SetCursorPos(vec2(cPos.X+(imgui.ContentRegionAvail().X-imgui.CalcTextSizeV("Save", false, 0).X-imgui.CurrentStyle().FramePadding().X*2)/2, cPos.Y))

				e := l.newCloneName == ""

				if e {
					imgui.PushItemFlag(imgui.ItemFlags(imgui.ItemFlagsDisabled), true)
				}

				if imgui.Button("Save##newclone") || (!e && (imgui.IsKeyPressedBool(imgui.KeyEnter) || imgui.IsKeyPressedBool(imgui.KeyKeypadEnter))) {
					profilePath, pathErr := profileFilePath(l.newCloneName)
					if pathErr != nil {
						showMessage(mError, "Invalid profile name: %s", pathErr)
						return
					}

					_, err := os.Stat(profilePath)
					switch {
					case err == nil:
						showMessage(mError, "Config with that name already exists!\nPlease pick a different name")
					case !os.IsNotExist(err):
						showMessage(mError, "Failed to inspect the target profile: %s", err)
					default:
						switch l.configManiMode {
						case Rename:
							l.renameConfig(l.configPrevName, l.newCloneName)
						case Clone:
							l.cloneConfig(l.configPrevName, l.newCloneName)
						case New:
							l.createConfig(l.newCloneName)
						}

						l.newCloneOpened = false
						l.newCloneName = ""
					}
				}

				if e {
					imgui.PopItemFlag()
				}

				imgui.EndTable()
			}
		})
	}
}

func (l *launcher) openCurrentSettingsEditor() {
	saveFunc := func() {
		if err := settings.SaveCredentialsChecked(false); err != nil {
			showMessage(mError, "Failed to save credentials: %s", err)
		}
		if l.currentConfig == nil {
			return
		}
		if err := l.currentConfig.SaveChecked("", false); err != nil {
			showMessage(mError, "Failed to save profile: %s", err)
			return
		}

		if !compareDirs(l.currentConfig.General.OsuSongsDir, settings.General.OsuSongsDir) {
			showMessage(mInfo, "This config has different osu! Songs directory.\nRestart the launcher to see updated maps")
		}
	}

	if l.currentEditor == nil || l.currentEditor.current != l.currentConfig {
		l.currentEditor = newSettingsEditor(l.currentConfig)
	}

	l.currentEditor.setDanserRunning(l.danserRunning)
	l.currentEditor.setCloseListener(saveFunc)
	l.currentEditor.setSaveListener(saveFunc)

	l.openPopup(l.currentEditor)
}

func (l *launcher) tryCreateDefaultConfig() {
	defaultPath, err := profileFilePath("default")
	if err != nil {
		showMessage(mError, "Failed to resolve the default profile: %s", err)
		return
	}

	_, err = os.Stat(defaultPath)
	if os.IsNotExist(err) {
		l.createConfig("default")
	} else if err != nil {
		showMessage(mError, "Failed to inspect the default profile: %s", err)
	}
}

func (l *launcher) createConfig(name string) {
	vm := gcontext.GetPrimaryVideoMode()
	path, err := profileFilePath(name)
	if err != nil {
		showMessage(mError, "Failed to create profile: %s", err)
		return
	}

	conf := settings.NewConfigFile()
	conf.Graphics.SetDefaults(int64(vm.W), int64(vm.H))
	if err := conf.SaveChecked(path, true); err != nil {
		showMessage(mError, "Failed to create profile: %s", err)
		return
	}

	l.createConfigList()

	l.setConfig(name)
}

func (l *launcher) removeConfig(name string) {
	if strings.EqualFold(name, "default") {
		return
	}

	path, err := profileFilePath(name)
	if err != nil {
		showMessage(mError, "Failed to remove profile: %s", err)
		return
	}

	if err := os.Remove(path); err != nil {
		showMessage(mError, "Failed to remove profile: %s", err)
		return
	}

	l.createConfigList()

	if l.bld.config == name {
		l.setConfig("default")
	}
}

func (l *launcher) cloneConfig(toClone, name string) {
	cConfig, err := l.loadConfig(toClone)

	if err != nil {
		showMessage(mError, "%s", err.Error())
		return
	}

	path, err := profileFilePath(name)
	if err != nil {
		showMessage(mError, "Failed to clone profile: %s", err)
		return
	}

	if err := cConfig.SaveChecked(path, true); err != nil {
		showMessage(mError, "Failed to clone profile: %s", err)
		return
	}

	l.createConfigList()

	l.setConfig(name)
}

func (l *launcher) renameConfig(toRename, name string) {
	cConfig, err := l.loadConfig(toRename)

	if err != nil {
		showMessage(mError, "%s", err.Error())
		return
	}

	path, err := profileFilePath(name)
	if err != nil {
		showMessage(mError, "Failed to save renamed profile: %s", err)
		return
	}

	if err := cConfig.SaveChecked(path, true); err != nil {
		showMessage(mError, "Failed to save renamed profile: %s", err)
		return
	}

	oldPath, err := profileFilePath(toRename)
	if err != nil {
		showMessage(mError, "Profile was copied but the old profile path was invalid: %s", err)
		return
	}

	if err := os.Remove(oldPath); err != nil {
		showMessage(mError, "Profile was copied but the old profile could not be removed: %s", err)
		return
	}

	l.createConfigList()

	l.setConfig(name)
}

func (l *launcher) createConfigList() {
	l.configList = []string{}

	if err := filepath.WalkDir(env.ConfigDir(), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			log.Println("Launcher: Failed to inspect profile:", err)
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}

		profile := profilePath(env.ConfigDir(), path)
		if profile != "" && !isReservedProfileName(profile) {
			l.configList = append(l.configList, profile)
		}

		return nil
	}); err != nil {
		log.Println("Launcher: Failed to enumerate profiles:", err)
	}

	log.Println("Available configs:", strings.Join(l.configList, ", "))

	sort.Strings(l.configList)

	l.configList = append([]string{"default"}, l.configList...)
}

func (l *launcher) loadConfig(name string) (config *settings.Config, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			config = nil
			err = fmt.Errorf("load profile %q panicked: %v", name, recovered)
		}
	}()

	path, err := profileFilePath(name)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("invalid profile file state: %w", err)
	}

	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close profile %q: %w", name, closeErr)
			config = nil
		}
	}()

	return settings.LoadConfig(f)
}

func (l *launcher) setConfig(s string) {
	eConfig, err := l.loadConfig(s)

	if err != nil {
		showMessage(mError, "Failed to read \"%s\" profile. Error: %s", s, err)
	} else {
		if !compareDirs(eConfig.General.OsuSongsDir, settings.General.OsuSongsDir) {
			showMessage(mInfo, "This config has different osu! Songs directory.\nRestart the launcher to see updated maps")
		}

		l.bld.config = s
		l.currentConfig = eConfig

		if launcherConfig.Profile == nil {
			launcherConfig.Profile = new(string)
		}
		*launcherConfig.Profile = l.bld.config
		saveLauncherConfig()
	}
}

func (l *launcher) openPopup(p iPopup) {
	p.open()
	l.popupStack = append(l.popupStack, p)
}

func (l *launcher) loadOSZs(names []string) {
	l.closeWatcher()
	l.beatmapDirUpdated = false

	reload := false

	for _, name := range names {
		if strings.EqualFold(filepath.Ext(name), ".osz") {
			fileName := filepath.Base(name)

			err := files.MoveFile(name, filepath.Join(settings.General.GetSongsDir(), fileName))
			if err != nil {
				showMessage(mError, "Failed to move \"%s\" to Songs folder: %s", name, err)
			} else {
				reload = true
			}
		}
	}

	if reload {
		// Archive moves are complete before the watcher is reinstalled. The
		// refresh itself is asynchronous, so a failure must not leave the
		// launcher without future filesystem notifications.
		l.setupWatcher()
		l.reloadMaps(func() {
			if l.bld.knockoutReplays == nil && l.bld.currentReplay == nil {
				l.ensureSongSelect().selectNewestWhenReady()
			}

		})
	} else {
		l.setupWatcher()
	}
}

func (l *launcher) reloadMaps(after func()) {
	l.loadBeatmaps(after)
}

func (l *launcher) setupWatcher() {
	l.closeWatcher()

	watcher, err := newDirectoryWatcher(settings.General.GetSongsDir(), func() {
		l.postBeatmapChangedEvent()
	})
	if err != nil {
		log.Println("DirWatcher: Could not watch Songs directory:", err)
		return
	}

	l.watcher = watcher
}

func (l *launcher) closeWatcher() {
	if l.watcher != nil {
		l.watcher.close()
		l.watcher = nil
	}
}
