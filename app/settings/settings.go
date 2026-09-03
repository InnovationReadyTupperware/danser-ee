package settings

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/innovationreadytupperware/danser-ee/framework/files"
	"github.com/itchio/lzma"
)

type defaultsFactory struct{}

var DefaultsFactory = &defaultsFactory{}

type Config struct {
	srcPath string
	srcData []byte

	General     *general     `icon:"\uF0AD"`                   // wrench
	Graphics    *graphics    `icon:"\uE163"  liveedit:"false"` // display
	Audio       *audio       `icon:"\uF028"`                   // volume-high
	Input       *input       `icon:"\uF11C"`                   // keyboard
	Gameplay    *gameplay    `icon:"\uF192"`                   // circle-dot
	Skin        *skin        `icon:"\uF1FC"`                   // paintbrush
	Cursor      *cursor      `icon:"\uF245"`                   // arrow-pointer
	Objects     *objects     `icon:"\uF1E0"`                   // share-nodes
	Playfield   *playfield   `icon:"\uF43C"`                   // chess-board
	CursorDance *cursorDance `icon:"\uE599"`                   // worm
	Knockout    *knockout    `icon:"\uF0CB"`                   // list-ol
	Recording   *recording   `icon:"\uF03D"`                   // video
	Debug       *debug       `icon:"\uF188"`                   // bug
	Dance       *danceOld    `json:",omitempty" icon:"\uF5B7"`
}

type CombinedConfig struct {
	Credentials *credentials `icon:"\uF084" label:"Credentials (Global)" liveedit:"false"` // key
	General     *general     `icon:"\uF0AD" liveedit:"false"`                              // wrench
	Graphics    *graphics    `icon:"\uE163"`                                               // display
	Audio       *audio       `icon:"\uF028"`                                               // volume-high
	Input       *input       `icon:"\uF11C"`                                               // keyboard
	Gameplay    *gameplay    `icon:"\uF192"`                                               // circle-dot
	Skin        *skin        `icon:"\uF1FC"`                                               // paintbrush
	Cursor      *cursor      `icon:"\uF245"`                                               // arrow-pointer
	Objects     *objects     `icon:"\uF1E0"`                                               // share-nodes
	Playfield   *playfield   `icon:"\uF43C"`                                               // chess-board
	CursorDance *cursorDance `icon:"\uE599"`                                               // worm
	Knockout    *knockout    `icon:"\uF0CB"`                                               // list-ol
	Recording   *recording   `icon:"\uF03D"`                                               // video
	Debug       *debug       `icon:"\uF188"`                                               // bug
}

func LoadConfig(file *os.File) (*Config, error) {
	log.Println(fmt.Sprintf(`SettingsManager: Loading "%s"`, file.Name()))

	data, err := io.ReadAll(files.NewUnicodeReader(file))
	if err != nil {
		return nil, fmt.Errorf("SettingsManager: Failed to read %s! Error: %s", file.Name(), err)
	}

	config := NewConfigFile()

	config.General.OsuReplaysDir = "" // Clear Replay path, so we can migrate it from Songs if JSON misses it

	config.srcPath = file.Name()
	config.srcData = data

	if err = json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("SettingsManager: Failed to parse %s! Please re-check the file for mistakes. Error: %s", file.Name(), err)
	}

	config.normalizeSections()
	config.normalizeRecording()
	config.normalizeCursorDance()
	config.migrateCursorDance()
	config.normalizeCursorDance()
	config.migrateHitCounterColors()
	config.migrateBlendWeights()
	config.migratePPVersion()

	if config.General.OsuReplaysDir == "" { // Set the replay directory if it hasn't been loaded
		config.General.OsuReplaysDir = filepath.Join(filepath.Dir(config.General.OsuSongsDir), "Replays")
	}

	log.Println(fmt.Sprintf(`SettingsManager: "%s" loaded!`, file.Name()))

	return config, nil
}

// normalizeRecording repairs nullable recording subsections from defaults.
// Profiles are user-editable JSON and explicit null values must not survive
// into encoder option generation or motion-blur setup.
func (config *Config) normalizeRecording() {
	if config.Recording == nil {
		config.Recording = initRecording()
		return
	}

	configured := config.Recording
	defaults := initRecording()
	if configured.X264Settings == nil {
		configured.X264Settings = defaults.X264Settings
	}
	if configured.X265Settings == nil {
		configured.X265Settings = defaults.X265Settings
	}
	if configured.AV1Settings == nil {
		configured.AV1Settings = defaults.AV1Settings
	}
	if configured.H264NvencSettings == nil {
		configured.H264NvencSettings = defaults.H264NvencSettings
	}
	if configured.HEVCNvencSettings == nil {
		configured.HEVCNvencSettings = defaults.HEVCNvencSettings
	}
	if configured.AV1NvencSettings == nil {
		configured.AV1NvencSettings = defaults.AV1NvencSettings
	}
	if configured.H264QSVSettings == nil {
		configured.H264QSVSettings = defaults.H264QSVSettings
	}
	if configured.HEVCQSVSettings == nil {
		configured.HEVCQSVSettings = defaults.HEVCQSVSettings
	}
	if configured.H264AmfSettings == nil {
		configured.H264AmfSettings = defaults.H264AmfSettings
	}
	if configured.HEVCAmfSettings == nil {
		configured.HEVCAmfSettings = defaults.HEVCAmfSettings
	}
	if configured.AV1AmfSettings == nil {
		configured.AV1AmfSettings = defaults.AV1AmfSettings
	}
	if configured.CustomSettings == nil {
		configured.CustomSettings = defaults.CustomSettings
	}
	if configured.AACSettings == nil {
		configured.AACSettings = defaults.AACSettings
	}
	if configured.MP3Settings == nil {
		configured.MP3Settings = defaults.MP3Settings
	}
	if configured.OPUSSettings == nil {
		configured.OPUSSettings = defaults.OPUSSettings
	}
	if configured.FLACSettings == nil {
		configured.FLACSettings = defaults.FLACSettings
	}
	if configured.CustomAudioSettings == nil {
		configured.CustomAudioSettings = defaults.CustomAudioSettings
	}
	if configured.MotionBlur == nil {
		configured.MotionBlur = defaults.MotionBlur
	}
}

// normalizeSections keeps explicit null top-level sections from turning a
// user-editable profile into a later nil-pointer panic. Missing JSON fields
// already retain NewConfigFile defaults; only nil replacements need repair.
func (config *Config) normalizeSections() {
	defaults := NewConfigFile()

	if config.General == nil {
		config.General = defaults.General
	}
	if config.Graphics == nil {
		config.Graphics = defaults.Graphics
	}
	if config.Audio == nil {
		config.Audio = defaults.Audio
	}
	if config.Input == nil {
		config.Input = defaults.Input
	}
	if config.Gameplay == nil {
		config.Gameplay = defaults.Gameplay
	}
	if config.Skin == nil {
		config.Skin = defaults.Skin
	}
	if config.Cursor == nil {
		config.Cursor = defaults.Cursor
	}
	if config.Objects == nil {
		config.Objects = defaults.Objects
	}
	if config.Playfield == nil {
		config.Playfield = defaults.Playfield
	}
	if config.CursorDance == nil {
		config.CursorDance = defaults.CursorDance
	}
	if config.Knockout == nil {
		config.Knockout = defaults.Knockout
	}
	if config.Recording == nil {
		config.Recording = defaults.Recording
	}
	if config.Debug == nil {
		config.Debug = defaults.Debug
	}
}

func NewConfigFile() *Config {
	return &Config{
		General:     initGeneral(),
		Graphics:    initGraphics(),
		Audio:       initAudio(),
		Input:       initInput(),
		Gameplay:    initGameplay(),
		Skin:        initSkin(),
		Cursor:      initCursor(),
		Objects:     initObjects(),
		Playfield:   initPlayfield(),
		CursorDance: initCursorDance(),
		Knockout:    initKnockout(),
		Recording:   initRecording(),
		Debug:       initDebug(),
	}
}

// normalizeCursorDance repairs the part of the settings tree consumed by
// spinner generation. Config files are user-editable JSON, so a missing
// section, empty list, or explicit null entry must not turn into a panic in a
// render/update loop. The runtime mover also validates numeric values because
// settings can be changed programmatically after loading.
func (config *Config) normalizeCursorDance() {
	if config.CursorDance == nil {
		config.CursorDance = initCursorDance()
		return
	}

	defaults := initCursorDance()

	if config.CursorDance.SpinnerBehavior == nil {
		config.CursorDance.SpinnerBehavior = defaults.SpinnerBehavior
	}

	if config.CursorDance.MoverSettings == nil {
		config.CursorDance.MoverSettings = defaults.MoverSettings
	}

	// Every mover selects its settings by taking an id modulo one of these
	// lists. An explicit empty list therefore becomes a runtime divide-by-zero
	// or index-out-of-range panic even though the parent settings object exists.
	// Repair both empty lists and null entries here so the individual movers can
	// keep their hot paths focused on movement math.
	if len(config.CursorDance.MoverSettings.Bezier) == 0 {
		config.CursorDance.MoverSettings.Bezier = defaults.MoverSettings.Bezier
	}
	for i, configured := range config.CursorDance.MoverSettings.Bezier {
		if configured == nil {
			config.CursorDance.MoverSettings.Bezier[i] = DefaultsFactory.InitBezier()
		}
	}

	if len(config.CursorDance.MoverSettings.Flower) == 0 {
		config.CursorDance.MoverSettings.Flower = defaults.MoverSettings.Flower
	}
	for i, configured := range config.CursorDance.MoverSettings.Flower {
		if configured == nil {
			config.CursorDance.MoverSettings.Flower[i] = DefaultsFactory.InitFlower()
		}
	}

	if len(config.CursorDance.MoverSettings.HalfCircle) == 0 {
		config.CursorDance.MoverSettings.HalfCircle = defaults.MoverSettings.HalfCircle
	}
	for i, configured := range config.CursorDance.MoverSettings.HalfCircle {
		if configured == nil {
			config.CursorDance.MoverSettings.HalfCircle[i] = DefaultsFactory.InitCircular()
		}
	}

	if len(config.CursorDance.MoverSettings.Spline) == 0 {
		config.CursorDance.MoverSettings.Spline = defaults.MoverSettings.Spline
	}
	for i, configured := range config.CursorDance.MoverSettings.Spline {
		if configured == nil {
			config.CursorDance.MoverSettings.Spline[i] = DefaultsFactory.InitSpline()
		}
	}

	if len(config.CursorDance.MoverSettings.Momentum) == 0 {
		config.CursorDance.MoverSettings.Momentum = defaults.MoverSettings.Momentum
	}
	for i, configured := range config.CursorDance.MoverSettings.Momentum {
		if configured == nil {
			config.CursorDance.MoverSettings.Momentum[i] = DefaultsFactory.InitMomentum()
		}
	}

	if len(config.CursorDance.MoverSettings.ExGon) == 0 {
		config.CursorDance.MoverSettings.ExGon = defaults.MoverSettings.ExGon
	}
	for i, configured := range config.CursorDance.MoverSettings.ExGon {
		if configured == nil {
			config.CursorDance.MoverSettings.ExGon[i] = DefaultsFactory.InitExGon()
		}
	}

	if len(config.CursorDance.MoverSettings.Linear) == 0 {
		config.CursorDance.MoverSettings.Linear = defaults.MoverSettings.Linear
	}
	for i, configured := range config.CursorDance.MoverSettings.Linear {
		if configured == nil {
			config.CursorDance.MoverSettings.Linear[i] = DefaultsFactory.InitLinear()
		}
	}

	if len(config.CursorDance.MoverSettings.Pippi) == 0 {
		config.CursorDance.MoverSettings.Pippi = defaults.MoverSettings.Pippi
	}
	for i, configured := range config.CursorDance.MoverSettings.Pippi {
		if configured == nil {
			config.CursorDance.MoverSettings.Pippi[i] = DefaultsFactory.InitPippi()
		}
	}

	if len(config.CursorDance.Movers) == 0 {
		config.CursorDance.Movers = defaults.Movers
	}

	for i, configured := range config.CursorDance.Movers {
		if configured == nil {
			config.CursorDance.Movers[i] = DefaultsFactory.InitMover()
		}
	}

	if len(config.CursorDance.Spinners) == 0 {
		config.CursorDance.Spinners = defaults.Spinners
	}

	for i, configured := range config.CursorDance.Spinners {
		if configured == nil {
			config.CursorDance.Spinners[i] = DefaultsFactory.InitSpinner()
		}
	}
}

func (config *Config) migrateCursorDance() {
	if config.Dance == nil {
		return
	}

	movers := make([]*mover, 0, len(config.Dance.Movers))
	spinners := make([]*spinner, 0, len(config.Dance.Spinners))

	for _, m := range config.Dance.Movers {
		movers = append(movers, &mover{
			Mover:             m,
			SliderDance:       config.Dance.SliderDance,
			RandomSliderDance: config.Dance.RandomSliderDance,
		})
	}

	for _, m := range config.Dance.Spinners {
		spinners = append(spinners, &spinner{
			Mover:  m,
			Radius: config.Dance.SpinnerRadius,
		})
	}

	config.CursorDance.Movers = movers
	config.CursorDance.Spinners = spinners

	config.CursorDance.Battle = config.Dance.Battle
	config.CursorDance.DoSpinnersTogether = config.Dance.DoSpinnersTogether
	config.CursorDance.TAGSliderDance = config.Dance.TAGSliderDance

	if config.Dance.Bezier != nil {
		config.CursorDance.MoverSettings.Bezier = []*bezier{
			config.Dance.Bezier,
		}
	}

	if config.Dance.Flower != nil {
		config.CursorDance.MoverSettings.Flower = []*flower{
			config.Dance.Flower,
		}
	}

	if config.Dance.HalfCircle != nil {
		config.CursorDance.MoverSettings.HalfCircle = []*circular{
			config.Dance.HalfCircle,
		}
	}

	if config.Dance.Spline != nil {
		config.CursorDance.MoverSettings.Spline = []*spline{
			config.Dance.Spline,
		}
	}

	if config.Dance.Momentum != nil {
		config.CursorDance.MoverSettings.Momentum = []*momentum{
			config.Dance.Momentum,
		}
	}

	if config.Dance.ExGon != nil {
		config.CursorDance.MoverSettings.ExGon = []*exgon{
			config.Dance.ExGon,
		}
	}

	config.Dance = nil
}

func (config *Config) migrateHitCounterColors() {
	if config.Gameplay.HitCounter.Color == nil {
		return
	}

	idx := 0

	ln := len(config.Gameplay.HitCounter.Color)

	if config.Gameplay.HitCounter.Show300 {
		config.Gameplay.HitCounter.Color300 = config.Gameplay.HitCounter.Color[idx%ln]
		idx++
	}

	config.Gameplay.HitCounter.Color100 = config.Gameplay.HitCounter.Color[idx%ln]
	idx++

	config.Gameplay.HitCounter.Color50 = config.Gameplay.HitCounter.Color[idx%ln]
	idx++

	config.Gameplay.HitCounter.ColorMiss = config.Gameplay.HitCounter.Color[idx%ln]
	idx++

	if config.Gameplay.HitCounter.ShowSliderBreaks {
		config.Gameplay.HitCounter.ColorSB = config.Gameplay.HitCounter.Color[idx%ln]
		idx++
	}

	config.Gameplay.HitCounter.Color = nil
}

func (config *Config) migrateBlendWeights() {
	if config.Recording.MotionBlur.BlendWeights == nil {
		return
	}

	config.Recording.MotionBlur.BlendFunctionID = config.Recording.MotionBlur.BlendWeights.AutoWeightsID
	config.Recording.MotionBlur.GaussWeightsMult = config.Recording.MotionBlur.BlendWeights.GaussWeightsMult

	config.Recording.MotionBlur.BlendWeights = nil
}

func (config *Config) migratePPVersion() {
	if config.Gameplay == nil {
		return
	}

	config.Gameplay.PPVersion = CanonicalPPVersion(config.Gameplay.PPVersion)
}

// CanonicalPPVersion maps stored PPVersion values to stable identifiers.
// "latest" and "26xxxx" remain accepted as aliases of the calculators they
// already selected.
func CanonicalPPVersion(version string) string {
	switch version {
	case "latest":
		return "251020"
	case "26xxxx":
		return "260321"
	default:
		return version
	}
}

func (config *Config) attachToGlobals() {
	General = config.General
	Graphics = config.Graphics
	Audio = config.Audio
	Input = config.Input
	Gameplay = config.Gameplay
	Skin = config.Skin
	Cursor = config.Cursor
	Objects = config.Objects
	Playfield = config.Playfield
	CursorDance = config.CursorDance
	Knockout = config.Knockout
	Recording = config.Recording
	Debug = config.Debug
}

func (config *Config) GetCombined() *CombinedConfig {
	return &CombinedConfig{
		Credentials: Credentails,
		General:     config.General,
		Graphics:    config.Graphics,
		Audio:       config.Audio,
		Input:       config.Input,
		Gameplay:    config.Gameplay,
		Skin:        config.Skin,
		Cursor:      config.Cursor,
		Objects:     config.Objects,
		Playfield:   config.Playfield,
		CursorDance: config.CursorDance,
		Knockout:    config.Knockout,
		Recording:   config.Recording,
		Debug:       config.Debug,
	}
}

func (config *Config) Save(path string, forceSave bool) {
	if err := config.SaveChecked(path, forceSave); err != nil {
		panic(err)
	}
}

// SaveChecked writes a settings file and returns persistence errors to the
// caller. The launcher uses this form because a malformed or temporarily
// unavailable profile must not crash the UI process.
func (config *Config) SaveChecked(path string, forceSave bool) error {
	if config == nil {
		return fmt.Errorf("cannot save a nil settings config")
	}

	if strings.TrimSpace(path) == "" {
		path = config.srcPath
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("settings path is empty")
	}

	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	if forceSave || !bytes.Equal(data, config.srcData) { // Don't rewrite the file unless necessary
		log.Println(fmt.Sprintf(`SettingsManager: Saving settings to "%s"`, path))

		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create settings directory: %w", err)
		}

		if err = os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("write settings: %w", err)
		}

		config.srcData = data
		config.srcPath = path
	}

	return nil
}

func (config *Config) GetCompressedString() string {
	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		panic(err)
	}

	buf := new(bytes.Buffer)

	writer := lzma.NewWriter(buf)

	_, _ = writer.Write(data)
	_ = writer.Close()

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
