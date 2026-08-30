package app

import "C"
import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/wieku/rplpa"

	"github.com/innovationreadytupperware/danser-ee/app/audio"
	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	difficulty2 "github.com/innovationreadytupperware/danser-ee/app/beatmap/difficulty"
	camera2 "github.com/innovationreadytupperware/danser-ee/app/bmath/camera"
	"github.com/innovationreadytupperware/danser-ee/app/database"
	"github.com/innovationreadytupperware/danser-ee/app/discord"
	"github.com/innovationreadytupperware/danser-ee/app/ffmpeg"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/app/states"
	"github.com/innovationreadytupperware/danser-ee/app/utils"
	"github.com/innovationreadytupperware/danser-ee/build"
	"github.com/innovationreadytupperware/danser-ee/framework/assets"
	"github.com/innovationreadytupperware/danser-ee/framework/bass"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/frame"
	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	batch2 "github.com/innovationreadytupperware/danser-ee/framework/graphics/batch"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/blend"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/buffer"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/font"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/viewport"
	"github.com/innovationreadytupperware/danser-ee/framework/math/vector"
	"github.com/innovationreadytupperware/danser-ee/framework/platform"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
	"github.com/innovationreadytupperware/danser-ee/framework/profiler"
	"github.com/innovationreadytupperware/danser-ee/framework/qpc"
	"github.com/innovationreadytupperware/danser-ee/framework/util"
)

const (
	base           = "Specify the"
	artistDesc     = base + " artist of a song"
	titleDesc      = base + " title of a song"
	creatorDesc    = base + " creator of a map"
	difficultyDesc = base + " difficulty(version) of a map"
	replayDesc     = "Play a map from specific replay file. Overrides -knockout, -mods and all beatmap arguments."
	shorthand      = " (shorthand)"
)

var player states.State

var scheduleScreenshot = false

var batch *batch2.QuadBatch

var limiter *frame.Limiter
var screenFBO *buffer.Framebuffer
var lastSamples int
var lastVSync bool

var output string

var recordMode bool
var screenshotMode bool
var screenshotTime float64

var preciseProgress bool

var monitorHz int

func printCLIUsage() {
	output := flag.CommandLine.Output()
	cliName := filepath.Base(os.Args[0])
	launcherName := "danser"
	if runtime.GOOS == "windows" {
		launcherName += ".exe"
	}

	fmt.Fprintln(output, "No beatmap or replay selected.")
	fmt.Fprintf(output, "Run %s to open the graphical launcher, or provide input to %s.\n", launcherName, cliName)
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Examples:")
	fmt.Fprintf(output, "  %s -md5 <beatmap-md5>\n", cliName)
	fmt.Fprintf(output, "  %s -replay path/to/replay.osr\n", cliName)
	fmt.Fprintln(output)
	fmt.Fprintf(output, "Use %s -h for all available options.\n", cliName)
}

func run() {
	defer func() {
		if err := recover(); err != nil {
			stackTrace := goroutines.GetStackTrace(4)
			closeHandler(err, stackTrace)
		}
	}()

	goroutines.CallMain(func() {
		id := flag.Int64("id", -1, "Specify the beatmap id. Overrides other beatmap search flags")

		md5 := flag.String("md5", "", "Specify the beatmap md5 hash. Overrides other beatmap search flags")

		artist := flag.String("artist", "", artistDesc)
		flag.StringVar(artist, "a", "", artistDesc+shorthand)

		title := flag.String("title", "", titleDesc)
		flag.StringVar(title, "t", "", titleDesc+shorthand)

		difficulty := flag.String("difficulty", "", difficultyDesc)
		flag.StringVar(difficulty, "d", "", difficultyDesc+shorthand)

		creator := flag.String("creator", "", creatorDesc)
		flag.StringVar(creator, "c", "", creatorDesc+shorthand)

		settingsVersion := flag.String("settings", "", "Specify settings version, -settings=b/abc means that settings/b/abc.json will be loaded. \"Credentials\"")
		cursors := flag.Int("cursors", 1, "How many repeated cursors should be visible, recommended 2 for mirror, 8 for mandala")
		tag := flag.Int("tag", 1, "How many generated cursors to create. In cursor-dance TAG mode they take turns on objects; in solo knockout each is scored as a full participant.")

		knockout := flag.Bool("knockout", false, "Use (classic) knockout feature. Replays are sourced from \"replays/{a}\" where {a} is an md5 hash of .osu file. Danser automatically organizes replay files put directly in \"replays\", using maps' md5s provided by the replay files.")
		knockout2 := flag.String("knockout2", "", "Use (new) knockout feature, JSON list of paths to compatible replay files has to be provided. \"Knockout.ExcludeMods\" and \"Knockout.MaxPlayers\" options are ignored, they have to be filtered beforehand.")
		soloKnockout := flag.Bool("solo-knockout", false, "Run map-driven knockout with generated danser participants. Use -cursors and -tag to configure mirrors and participants.")

		speed := flag.Float64("speed", 1.0, "Specify music's speed, set to 1.5 to have DoubleTime mod experience")
		pitch := flag.Float64("pitch", 1.0, "Specify music's pitch, set to 1.5 with -speed=1.5 to have Nightcore mod experience")
		debug := flag.Bool("debug", false, "Show info about map and rendering engine, overrides Graphics.ShowFPS setting. Ignored in record/screenshot modes.")

		gldebug := flag.Bool("gldebug", false, "Turns on OpenGL debug logging, may reduce performance heavily")

		play := flag.Bool("play", false, "Practice playing osu!standard maps")
		start := flag.Float64("start", 0, "Start at the given time in seconds")
		end := flag.Float64("end", math.Inf(1), "End at the given time in seconds")

		skip := flag.Bool("skip", false, "Skip straight to map's drain time")

		quickstart := flag.Bool("quickstart", false, "Sets -skip flag, sets LeadInTime and LeadInHold settings temporarily to 0")

		record := flag.Bool("record", false, "Records a video")
		out := flag.String("out", "", "If -ss is used, sets the screenshot name. Otherwise, specifies a recorded-video base name without directories; existing videos are not overwritten. Extensions are managed automatically.")
		ss := flag.Float64("ss", math.NaN(), "Screenshot mode. Snap single frame from danser at given time in seconds. Specify the name of file by -out, resolution is managed by Recording settings")

		mods := flag.String("mods", "", "Specify beatmap/play mods")
		mods2 := flag.String("mods2", "", "Specify beatmap/play mods, lazer style")

		replay := flag.String("replay", "", replayDesc)
		flag.StringVar(replay, "r", "", replayDesc+shorthand)

		skin := flag.String("skin", "", "Replace Skin.CurrentSkin setting temporarily")

		noDbCheck := flag.Bool("nodbcheck", false, "Don't validate the database and only import new beatmap sets if there are any. Useful for slow drives.")
		rebuildDB := flag.Bool("rebuilddb", false, "Rebuild danser's beatmap catalog by rescanning the Songs folder while preserving local play statistics.")
		noUpdCheck := flag.Bool("noupdatecheck", strings.HasPrefix(env.LibDir(), "/usr/lib/"), "Don't check for updates. Speeds up startup if older version of danser is needed for various reasons. Has no effect if danser is running as a linux package")

		ar := flag.Float64("ar", math.NaN(), "Modify map's AR, only in cursordance/play modes")
		od := flag.Float64("od", math.NaN(), "Modify map's OD, only in cursordance/play modes")
		cs := flag.Float64("cs", math.NaN(), "Modify map's CS, only in cursordance/play modes")
		hp := flag.Float64("hp", math.NaN(), "Modify map's HP, only in cursordance/play modes")

		offset := flag.Int("offset", 0, "Specify local audio offset in ms. Applies to recordings, unlike 'Audio.Offset'. Inverted compared to stable's local offset.")

		flag.BoolVar(&preciseProgress, "preciseprogress", false, "Show rendering progress in 1% increments")

		sPatch := flag.String("sPatch", "", "Patches the currently loaded settings")

		flag.Parse()

		// The GUI binary routes no-argument launches to the launcher. The CLI
		// binary reaches this package directly, so provide a useful next step
		// without initializing SDL, OpenGL, audio, or the beatmap database.
		if len(os.Args) == 1 {
			printCLIUsage()
			return
		}

		if *mods != "" && *mods2 != "" {
			panic("You can't specify legacy and structured mods at the same time")
		}

		var knockoutReplays []string

		if *knockout2 != "" {
			if err := json.Unmarshal([]byte(*knockout2), &knockoutReplays); err != nil {
				panic(fmt.Sprintf("Failed to parse replay list: %s", err))
			}

			*knockout = true

			if *soloKnockout {
				panic("Incompatible flags selected: -solo-knockout, -knockout2")
			}
		}

		if *soloKnockout {
			*knockout = true
		}

		if *knockout2 != "" && len(knockoutReplays) == 0 {
			panic("-knockout2 requires at least one replay path")
		}

		if !*noUpdCheck {
			checkForUpdates()
		}

		if *out != "" {
			output = *out
			if math.IsNaN(*ss) {
				*record = true
			}
		}

		recordMode = *record
		screenshotMode = !math.IsNaN(*ss)
		screenshotTime = *ss
		if recordMode {
			if err := ffmpeg.ValidateOutputName(output); err != nil {
				panic(fmt.Errorf("invalid recording output name: %w", err))
			}
		}

		if *record && *play {
			panic("Incompatible flags selected: -record, -play")
		} else if *replay != "" && *play {
			panic("Incompatible flags selected: -replay, -play")
		} else if *knockout && *play {
			panic("Incompatible flags selected: -knockout, -play")
		} else if *replay != "" && *knockout {
			panic("Incompatible flags selected: -replay, -knockout")
		} else if screenshotMode && *play {
			panic("Incompatible flags selected: -ss, -play")
		} else if screenshotMode && recordMode {
			panic("Incompatible flags selected: -ss, -record")
		}

		modsParsed := difficulty2.ParseMods(*mods)
		var modsNew []rplpa.ModInfo = nil

		if *replay != "" {
			bytes, err := ioutil.ReadFile(*replay)
			if err != nil {
				panic(err)
			}

			rp, err := rplpa.ParseReplay(bytes)
			if err != nil {
				panic(err)
			}

			if rp.PlayMode != 0 {
				panic("Modes other than osu!standard are not supported")
			}

			if rp.ReplayData == nil || len(rp.ReplayData) < 2 {
				panic("Replay is missing input data")
			}

			*md5 = rp.BeatmapMD5
			*id = -1
			modsParsed = difficulty2.Modifier(rp.Mods)

			if rp.ScoreInfo != nil && rp.ScoreInfo.Mods != nil && len(rp.ScoreInfo.Mods) > 0 {
				modsNew = make([]rplpa.ModInfo, 0, len(rp.ScoreInfo.Mods))

				for _, mod := range rp.ScoreInfo.Mods {
					modsNew = append(modsNew, *mod)
				}
			}

			*knockout = true
			settings.REPLAY = *replay
		}

		if *mods2 != "" {
			var mods2I []rplpa.ModInfo

			if err := json.Unmarshal([]byte(*mods2), &mods2I); err != nil {
				panic(fmt.Sprintf("Failed to parse replay list: %s", err))
			}

			modsNew = mods2I
		}

		if modsNew != nil {
			tempDiff := difficulty2.NewDifficulty(1, 1, 1, 1)
			tempDiff.SetMods2(modsNew)
			modsParsed = tempDiff.Mods
		}

		if !modsParsed.Compatible() {
			panic("Incompatible mods selected!")
		}

		closeAfterSettingsLoad := false

		if (*md5+*artist+*title+*difficulty+*creator) == "" && *id < 0 {
			log.Println("No beatmap specified; provide beatmap details or use the launcher.")
			closeAfterSettingsLoad = true
		}

		settings.DEBUG = *debug
		settings.KNOCKOUT = *knockout
		settings.SOLOKNOCKOUT = *soloKnockout
		settings.KNOCKOUTREPLAYS = knockoutReplays
		settings.PLAY = *play
		settings.DIVIDES = *cursors
		settings.TAG = *tag
		settings.SPEED = *speed
		settings.PITCH = *pitch
		settings.SKIP = *skip
		settings.START = *start
		settings.END = *end
		settings.RECORD = recordMode || screenshotMode
		settings.LOCALOFFSET = *offset

		if *settingsVersion == "credentials" || *settingsVersion == "launcher" {
			panic(fmt.Sprintf("flag -settings: name \"%s\" is forbidden", *settingsVersion))
		}

		newSettings := settings.LoadSettings(*settingsVersion)

		if !newSettings {
			settings.JsonPatch = *sPatch
			settings.LoadPatch()
			log.Println("Current config:", settings.GetCompressedString())
		}

		player = nil
		var beatMap *beatmap.BeatMap = nil

		if !closeAfterSettingsLoad {
			err := database.Init()
			if err != nil {
				log.Println("Failed to initialize database:", err)
			} else {
				if *rebuildDB {
					database.RebuildCatalog(nil)
				}

				var entries []*database.BeatmapEntry
				if *rebuildDB {
					database.LoadCachedCatalog().ForEach(func(entry *database.BeatmapEntry) bool {
						entries = append(entries, entry)
						return true
					})
				} else {
					entries = database.LoadCatalog(*noDbCheck, nil)
				}
				var selectedEntry *database.BeatmapEntry

				if *id > -1 {
					for _, entry := range entries {
						if entry.ID == *id {
							selectedEntry = entry

							break
						}
					}
				} else if *md5 != "" {
					for _, entry := range entries {
						if strings.EqualFold(entry.MD5, *md5) {
							selectedEntry = entry

							break
						}
					}
				} else {
					for _, entry := range entries {
						if (*artist == "" || strings.EqualFold(*artist, entry.Artist)) &&
							(*title == "" || strings.EqualFold(*title, entry.Name)) &&
							(*difficulty == "" || strings.EqualFold(*difficulty, entry.Difficulty)) &&
							(*creator == "" || strings.EqualFold(*creator, entry.Creator)) {
							selectedEntry = entry

							break
						}
					}

					if selectedEntry == nil {
						log.Println("Beatmap with exact parameters not found, searching partially...")
						artistQuery := strings.ToLower(*artist)
						titleQuery := strings.ToLower(*title)
						difficultyQuery := strings.ToLower(*difficulty)
						creatorQuery := strings.ToLower(*creator)
						for _, entry := range entries {
							if (*artist == "" || strings.Contains(strings.ToLower(entry.Artist), artistQuery)) &&
								(*title == "" || strings.Contains(strings.ToLower(entry.Name), titleQuery)) &&
								(*difficulty == "" || strings.Contains(strings.ToLower(entry.Difficulty), difficultyQuery)) &&
								(*creator == "" || strings.Contains(strings.ToLower(entry.Creator), creatorQuery)) {
								selectedEntry = entry

								break
							}
						}
					}
				}

				if selectedEntry != nil {
					beatMap, err = database.LoadRuntimeBeatMap(selectedEntry)
					if err != nil {
						log.Println("Failed to load selected beatmap:", err)
					}
				}
			}

			if beatMap == nil {
				log.Println("Beatmap not found, closing...")
				closeAfterSettingsLoad = true
			} else {
				beatMap.UpdatePlayStats()
				database.UpdatePlayStats(beatMap)
			}

			database.Close()
		}

		assets.Init(build.Stream == "Dev")

		if !closeAfterSettingsLoad {
			log.Println("Initializing SDL...")
		}

		err := gcontext.Initialize(settings.RECORD)
		if err != nil {
			panic("Failed to initialize SDL: " + err.Error())
		}

		if !closeAfterSettingsLoad {
			log.Println("SDL Initialized!")
		}

		vm := gcontext.GetPrimaryVideoMode()

		monitorHz = int(vm.RefreshRate)

		if newSettings {
			settings.Graphics.SetDefaults(int64(vm.W), int64(vm.H))
			settings.Save()

			settings.JsonPatch = *sPatch
			settings.LoadPatch()

			log.Println("Current config:", settings.GetCompressedString())
		}

		if closeAfterSettingsLoad {
			os.Exit(0)
		}

		automatedPlayback := false

		// An AT launch without explicit knockout or play mode uses the replay
		// pipeline so custom AR, OD, CS, and HP values can be applied. Keep this
		// provenance available to gameplay setup so generated playback can use
		// the same failure eligibility as osu!lazer.
		if !settings.KNOCKOUT && modsParsed.Active(difficulty2.Autoplay) {
			settings.PLAY = false
			settings.KNOCKOUT = true
			settings.Knockout.MaxPlayers = 0
			automatedPlayback = true
		}

		lastSamples = int(settings.Graphics.MSAA)

		if strings.TrimSpace(*skin) != "" {
			settings.Skin.CurrentSkin = *skin
		}

		if *quickstart {
			settings.SKIP = true
			settings.Playfield.LeadInTime = 0
			settings.Playfield.LeadInHold = 0
		}

		if settings.RECORD {
			//HACK: some in-app variables depend on these settings so we force them here
			settings.Graphics.VSync = false
			settings.Graphics.ShowFPS = false
			settings.DEBUG = false
			settings.Graphics.Fullscreen = false
			settings.Graphics.WindowWidth = int64(settings.Recording.FrameWidth)
			settings.Graphics.WindowHeight = int64(settings.Recording.FrameHeight)
			settings.Playfield.LeadInTime = 0
		}

		if screenshotMode {
			settings.Playfield.LeadInHold = 0
			settings.START = screenshotTime - 5
			settings.SKIP = false
		}

		log.Println("Creating window...")

		iconName := "dansercoin*"
		if cTime := time.Now(); cTime.Month() == 12 && cTime.Day() >= 6 {
			iconName += "-s"
		}

		err = gcontext.SDLCreateWindow(
			int(settings.Graphics.GetWidth()),
			int(settings.Graphics.GetHeight()),
			"danser "+build.VERSION+" - "+beatMap.Artist+" - "+beatMap.Name+" ["+beatMap.Difficulty+"]",
			gcontext.OptionalProps{
				IconName:       iconName,
				BuiltinMSAA:    false,
				Resizable:      false,
				ScaleToMonitor: false,
				Hidden:         settings.RECORD,
				Fullscreen:     settings.Graphics.Fullscreen,
			})
		if err != nil {
			panic("Failed to create SDL window: " + err.Error())
		}

		log.Println("Window created!")

		err = gcontext.GLInit(*gldebug)
		if err != nil {
			panic("Failed to initialize OpenGL: " + err.Error())
		}

		if !settings.RECORD {
			discord.Connect()
		}

		gl.Enable(gl.BLEND)
		gl.ClearColor(0, 0, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT)

		file, _ := assets.Open("assets/fonts/Quicksand-Bold.ttf")
		font.LoadFont(file)
		file.Close()

		batch = batch2.NewQuadBatch()
		batch.Begin()
		batch.SetColor(1, 1, 1, 1)
		camera := camera2.NewCamera()
		camera.SetViewport(int(settings.Graphics.GetWidth()), int(settings.Graphics.GetHeight()), true)
		camera.SetOrigin(vector.NewVec2d(settings.Graphics.GetWidthF()/2, settings.Graphics.GetHeightF()/2))
		camera.Update()
		batch.SetCamera(camera.GetProjectionView())

		font.GetFont("Quicksand Bold").Draw(batch, 0, settings.Graphics.GetHeightF()-10, 32, "Loading...")

		batch.End()
		gcontext.SwapBuffers()

		gcontext.SetSwapInterval(1)
		lastVSync = true

		bass.Init(settings.RECORD)
		audio.LoadSamples()

		if settings.PLAY || !settings.KNOCKOUT || settings.SOLOKNOCKOUT || automatedPlayback {
			if modsNew == nil {
				modsNew = modsParsed.ConvertToModInfoList()
			}

			daMap := make(map[string]any)

			if !math.IsNaN(*ar) {
				daMap["approach_rate"] = *ar
			}

			if !math.IsNaN(*od) {
				daMap["overall_difficulty"] = *od
			}

			if !math.IsNaN(*cs) {
				daMap["circle_size"] = *cs
			}

			if !math.IsNaN(*hp) {
				daMap["drain_rate"] = *hp
			}

			// Add DA only if DA hasn't been added already
			if len(daMap) > 0 && !slices.ContainsFunc(modsNew, func(info rplpa.ModInfo) bool { return info.Acronym == "DA" }) {
				modsNew = append(modsNew, rplpa.ModInfo{
					Acronym:  "DA",
					Settings: daMap,
				})
			}

			if math.Abs(settings.SPEED-1) > 0.001 {
				skipMods := []string{"HT", "DC", "DT", "NC"}

				found := slices.ContainsFunc(modsNew, func(info rplpa.ModInfo) bool { return slices.Contains(skipMods, info.Acronym) })

				// Don't modify current mods
				//if settings.SPEED >= 1 {
				//	if i := slices.IndexFunc(modsNew, func(info rplpa.ModInfo) bool {
				//		return info.Acronym == "DT" || info.Acronym == "NC"
				//	}); i != -1 {
				//		found = true
				//		modsNew[i].Settings["speed_change"] = settings.SPEED
				//	}
				//} else {
				//	if i := slices.IndexFunc(modsNew, func(info rplpa.ModInfo) bool {
				//		return info.Acronym == "HT" || info.Acronym == "DC"
				//	}); i != -1 {
				//		found = true
				//		modsNew[i].Settings["speed_change"] = settings.SPEED
				//	}
				//}

				if !found {
					modsNew = slices.DeleteFunc(modsNew, func(info rplpa.ModInfo) bool {
						return info.Acronym == "DT" || info.Acronym == "NC" || info.Acronym == "HT" || info.Acronym == "DC"
					})

					acr := "HT"
					if settings.SPEED >= 1 {
						acr = "DT"
					}

					modsNew = append(modsNew, rplpa.ModInfo{
						Acronym: acr,
						Settings: map[string]any{
							"speed_change": settings.SPEED,
						},
					})
				}

				settings.SPEED = 1
			}
		}

		if modsNew != nil {
			beatMap.Diff.SetMods2(modsNew)
		} else {
			beatMap.Diff.SetMods(modsParsed)
		}

		beatmap.ParseTimingPointsAndPauses(beatMap)
		beatmap.ParseObjects(beatMap, false, true)
		beatMap.LoadCustomSamples()
		player = states.NewPlayer(beatMap, automatedPlayback)

		if !settings.RECORD {
			gcontext.Restore()
			gcontext.Focus()
		}

		limiter = frame.NewLimiter(int(settings.Graphics.FPSCap))
	})

	if recordMode {
		if err := mainLoopRecord(); err != nil {
			panic(err)
		}
	} else if screenshotMode {
		mainLoopSS()
	} else {
		mainLoopNormal()
	}
}

func mainLoopRecord() (recordingErr error) {
	count := int64(0)

	audioFPS := 1000.0

	w, h := int(settings.Graphics.GetWidth()), int(settings.Graphics.GetHeight())
	config, err := ffmpeg.PrepareRecording(ffmpeg.RecordingSessionRequest{
		Output: output, Width: w, Height: h, AudioBlockRate: audioFPS,
	})
	if err != nil {
		return fmt.Errorf("Recorder: preflight: %w", err)
	}
	p, _ := player.(*states.Player)
	timeline, err := ffmpeg.NewRecordingTimeline(p.RunningTime, config)
	if err != nil {
		return fmt.Errorf("Recorder: timeline: %w", err)
	}

	var fbo *buffer.Framebuffer

	goroutines.CallMain(func() {
		fbo = buffer.NewFrameMultisampleScreen(w, h, false, 0)
	})

	if err = ffmpeg.StartPreparedRecording(config); err != nil {
		return err
	}
	stopped := false
	defer func() {
		if stopped {
			return
		}

		_, stopErr := ffmpeg.StopFFmpeg()
		recordingErr = errors.Join(recordingErr, stopErr)
	}()

	lastCount := int64(0)
	lastRealTime := qpc.GetMilliTimeF()
	lastSimulationSample := int64(0)
	lastAudioSample := int64(0)
	outputFPS := float64(timeline.OutputRate())
	totalFrames := timeline.OutputFrames()

	var lastProgress, progress int

	if preciseProgress {
		lastProgress = -1
	}

	for event, ok := timeline.Next(); ok; event, ok = timeline.Next() {
		if event.SimulationSample > lastSimulationSample {
			deltaSamples := event.SimulationSample - lastSimulationSample
			p.UpdateRecording(float64(deltaSamples) * 1000 / float64(timeline.SampleRate()))
			lastSimulationSample = event.SimulationSample
		}
		if event.AudioSample > lastAudioSample {
			deltaSamples := event.AudioSample - lastAudioSample
			if deltaSamples > math.MaxInt {
				return fmt.Errorf("Recorder: audio interval %d exceeds supported block size", deltaSamples)
			}
			if err = ffmpeg.PushAudioFrames(int(deltaSamples)); err != nil {
				return fmt.Errorf("Recorder: submit audio: %w", err)
			}
			lastAudioSample = event.AudioSample
		}

		if event.RenderSource {
			var frameErr error
			goroutines.CallMain(func() {
				fbo.Bind()
				defer fbo.Unbind()

				ffmpeg.PreFrame()

				viewport.Push(int(settings.Graphics.GetWidth()), int(settings.Graphics.GetHeight()))
				pushFrame()
				viewport.Pop()

				frameErr = ffmpeg.MakeFrameDue(event.EmitOutput)
			})
			if frameErr != nil {
				return fmt.Errorf("Recorder: submit video frame: %w", frameErr)
			}
		}

		if event.EmitOutput {
			count++
			progress = 100
			if totalFrames > 0 {
				progress = int(count * 100 / totalFrames)
			}
			if (preciseProgress || progress%5 == 0) && lastProgress != progress {
				realElapsed := qpc.GetMilliTimeF() - lastRealTime
				speed := 0.0
				if realElapsed > 0 {
					speed = float64(count-lastCount) * (1000 / outputFPS) / realElapsed
				}
				eta := 0
				if speed > 0 {
					eta = int(float64(totalFrames-count) / outputFPS / speed)
				}
				if config.FFmpegLogsEnabled() {
					fmt.Println()
				}
				log.Printf("Progress: %d%%, Speed: %.2fx, ETA: %s", progress, speed, util.FormatSeconds(eta))
				lastProgress = progress
				lastCount = count
				lastRealTime = qpc.GetMilliTimeF()
			}
		}
	}

	var stopErr error
	goroutines.CallMain(func() {
		_, stopErr = ffmpeg.StopFFmpeg()
	})
	stopped = true

	return stopErr
}

func mainLoopSS() {
	w, h := int(settings.Graphics.GetWidth()), int(settings.Graphics.GetHeight())

	var fbo *buffer.Framebuffer

	goroutines.CallMain(func() {
		fbo = buffer.NewFrameMultisampleScreen(w, h, false, 0)
	})

	p, _ := player.(*states.Player)

	for !p.Update(1) {
		if p.GetTime() >= screenshotTime*1000 {
			log.Println("Scheduling screenshot")
			goroutines.CallMain(func() {
				fbo.Bind()

				viewport.Push(int(settings.Graphics.GetWidth()), int(settings.Graphics.GetHeight()))
				pushFrame()
				viewport.Pop()

				utils.MakeScreenshot(int(settings.Graphics.GetWidth()), int(settings.Graphics.GetHeight()), output, false)

				fbo.Unbind()
			})

			break
		}
	}
}

func mainLoopNormal() {
	goroutines.CallMain(func() {
		gcontext.RegisterListener(func(event gcontext.KeyEvent) {
			if event.Action == gcontext.Press {
				switch event.Name {
				case "F11":
					if (event.Mod & sdl.KMOD_CTRL) > 0 {
						settings.CallGraph = !settings.CallGraph
					} else if (event.Mod & sdl.KMOD_SHIFT) > 0 {
						settings.PerfGraph = !settings.PerfGraph
					} else {
						settings.DEBUG = !settings.DEBUG
					}
				case "ESCAPE":
					gcontext.SetShouldClose(true)
				case "MINUS":
					settings.DIVIDES = max(1, settings.DIVIDES-1)
				case "=":
					settings.DIVIDES += 1
				case "O":
					if event.Mod&sdl.KMOD_CTRL > 0 {
						log.Println("Launcher: Open settings")
					}
				default:
					if event.Name == settings.Input.ScreenshotKey {
						scheduleScreenshot = true
					}
				}
			}
		})
	})

	goroutines.RunMainLoop(func() bool {
		return !gcontext.ShouldClose()
	}, func() {
		if lastVSync != settings.Graphics.VSync {
			if settings.Graphics.VSync {
				gcontext.SetSwapInterval(1)
			} else {
				gcontext.SetSwapInterval(0)
			}

			lastVSync = settings.Graphics.VSync
		}

		profiler.StartGroup("gcontext.HandleEvents", profiler.PInput)

		gcontext.HandleEvents()

		profiler.EndGroup()

		pushFrame()

		if scheduleScreenshot {
			w, h := gcontext.GetFramebufferSize()
			utils.MakeScreenshot(w, h, "", true)
			scheduleScreenshot = false
		}

		profiler.StartGroup("App.mainLoopNormal", profiler.PSwapBuffers)

		gcontext.SwapBuffers()

		profiler.EndGroup()

		profiler.StartGroup("App.mainLoopNormal", profiler.PSleep)
		if !settings.Graphics.VSync {
			fCap := int(settings.Graphics.FPSCap)

			if fCap < 0 {
				fCap = -fCap * monitorHz
			}

			limiter.SetFPS(fCap)
			limiter.Sync()
		}
		profiler.EndGroup()
	})

	settings.CloseWatcher()
}

func pushFrame() {
	profiler.StartGroup("App.pushFrame", profiler.PDraw)
	profiler.ResetStats()

	gl.Enable(gl.SCISSOR_TEST)
	gl.Disable(gl.DITHER)

	blend.Enable()
	blend.SetFunction(blend.One, blend.OneMinusSrcAlpha)

	viewport.Push(int(settings.Graphics.GetWidth()), int(settings.Graphics.GetHeight()))

	if screenFBO == nil ||
		lastSamples != int(settings.Graphics.MSAA) ||
		screenFBO.GetWidth() != int(settings.Graphics.GetWidth()) ||
		screenFBO.GetHeight() != int(settings.Graphics.GetHeight()) {
		if screenFBO != nil {
			screenFBO.Dispose()
		}

		screenFBO = buffer.NewFrameMultisampleScreen(int(settings.Graphics.GetWidth()), int(settings.Graphics.GetHeight()), false, int(settings.Graphics.MSAA))

		lastSamples = int(settings.Graphics.MSAA)
	}

	if lastSamples > 0 {
		screenFBO.Bind()
	}

	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	if player != nil {
		player.Draw(0)
	}

	if lastSamples > 0 {
		screenFBO.Unbind()
	}

	blend.ClearStack()
	viewport.Pop()

	profiler.EndGroup()
}

func checkForUpdates() {
	status, url, err := utils.CheckForUpdate()

	switch status {
	case utils.Failed:
		log.Println("Can't get version from GitHub:", err)
	case utils.UpToDate:
		log.Println("You're using the newest version of danser.")
	case utils.Snapshot:
		log.Println("You're using a snapshot version of danser.")
		log.Println("For newer version of snapshots please visit the official danser discord server at:", url)
	case utils.UpdateAvailable:
		log.Println("You're using an older version of danser.")
		log.Println("You can download a newer version here:", url)
		time.Sleep(2 * time.Second)
	}
}

func Run() {
	defer func() {
		var err any
		var stackTrace []string

		if err = recover(); err != nil {
			stackTrace = goroutines.GetStackTrace(4)
		}

		closeHandler(err, stackTrace)
	}()

	runtime.GOMAXPROCS(runtime.NumCPU())

	goroutines.SetCrashHandler(closeHandler)

	platform.StartLogging("danser")

	platform.DisableQuickEdit()

	goroutines.RunMain(run)
}

func closeHandler(err any, stackTrace []string) {
	if player != nil {
		player.Dispose()
		player = nil
	}
	bass.Shutdown()
	audio.ClearBeatmapSamples()

	settings.CloseWatcher()
	discord.Disconnect()
	platform.EnableQuickEdit()

	if err != nil {
		log.Println("panic:", err)

		for _, s := range stackTrace {
			log.Println(s)
		}

		if !env.IsLauncherChild() {
			// Direct CLI launches have no launcher process to turn the panic line
			// into a native error surface. Pass the SDL window when one exists so
			// the dialog is owned and focused correctly; the platform layer falls
			// back to the OS dialog API when startup failed before SDL was usable.
			platform.ShowErrorDialog(
				gcontext.SDLWindow(),
				fmt.Sprintf("danser failed:\n\n%v\n\nSee danser.log for details.", err),
			)
		}

		os.Exit(1)
	}

	log.Println("Exiting normally.")
}
