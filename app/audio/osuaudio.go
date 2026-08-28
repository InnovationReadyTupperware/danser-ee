package audio

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/app/skin"
	"github.com/wieku/danser-go/framework/bass"
	"github.com/wieku/danser-go/framework/math/mutils"
)

const (
	sampleSetCount = 3
	hitSoundCount  = 7
	minimumVolume  = 0.08
)

var sets = map[string]int{
	"normal": 1,
	"soft":   2,
	"drum":   3,
}

var hitsounds = map[string]int{
	"hitnormal":     1,
	"hitwhistle":    2,
	"hitfinish":     3,
	"hitclap":       4,
	"slidertick":    5,
	"sliderslide":   6,
	"sliderwhistle": 7,
}

type sampleBank struct {
	mu       sync.RWMutex
	defaults [sampleSetCount][hitSoundCount]*bass.Sample
	beatmap  [sampleSetCount][hitSoundCount]map[int]*bass.Sample
}

var bank sampleBank

var (
	listenersMu sync.RWMutex
	listeners   []func(sampleSet int, hitsoundIndex, index int, volume float64, objNum int64)

	clockMu sync.RWMutex
	clock   ClockSnapshot
)

// ClockSnapshot maps gameplay time to the output position of the master
// mixer. Gameplay updates may notice a hit several milliseconds after its
// nominal timestamp; using this snapshot lets BASS start the sample at the
// correct output position instead of at the late update.
type ClockSnapshot struct {
	GameplayTimeMs   float64
	MixerTimeSeconds float64
	PlaybackRate     float64
	Valid            bool
}

// SliderLoopState owns the two continuous samples used by one slider. It is
// intentionally stored on the slider rather than globally: multiple sliders
// can overlap, and a global whistle/slide pair lets one slider stop another's
// sound.
type SliderLoopState struct {
	whistle *bass.SampleChannel
	slide   *bass.SampleChannel

	lastSampleSet   int
	lastAdditionSet int
	lastIndex       int
}

// SampleChannel is the native handle returned for a playing sample loop.
// Callers should use the helpers in this package to control it so all native
// calls retain the same serialized ownership boundary.
type SampleChannel = bass.SampleChannel

// AddListener registers a diagnostic or visualization callback for each
// resolved hit-sound component. Listener callbacks run synchronously on the
// caller's update thread after sample resolution.
func AddListener(function func(sampleSet int, hitsoundIndex, index int, volume float64, objNum int64)) {
	if function == nil {
		return
	}

	listenersMu.Lock()
	listeners = append(listeners, function)
	listenersMu.Unlock()
}

// SetClockSnapshot publishes the relationship between the gameplay and
// output timelines. A snapshot is normally published once per gameplay
// update, before objects submit their audio events.
func SetClockSnapshot(snapshot ClockSnapshot) {
	if !finite(snapshot.GameplayTimeMs) || !finite(snapshot.MixerTimeSeconds) || !finite(snapshot.PlaybackRate) || snapshot.PlaybackRate <= 0 {
		snapshot.Valid = false
	} else {
		snapshot.Valid = true
	}

	clockMu.Lock()
	clock = snapshot
	clockMu.Unlock()
}

// ResetClock invalidates output-domain scheduling until the next gameplay
// update publishes a new anchor. This prevents samples from an abandoned
// seek or player session from being scheduled against the new timeline.
func ResetClock() {
	clockMu.Lock()
	clock = ClockSnapshot{}
	clockMu.Unlock()
}

func currentGameplayTime() float64 {
	clockMu.RLock()
	defer clockMu.RUnlock()

	if !clock.Valid {
		return math.NaN()
	}

	return clock.GameplayTimeMs
}

func scheduledMixerTime(eventTime float64) float64 {
	if !finite(eventTime) {
		return -1
	}

	clockMu.RLock()
	snapshot := clock
	clockMu.RUnlock()

	if !snapshot.Valid {
		return -1
	}

	mixerTime := snapshot.MixerTimeSeconds + (eventTime-snapshot.GameplayTimeMs)/1000/snapshot.PlaybackRate
	if !finite(mixerTime) {
		return -1
	}

	return mixerTime
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// LoadSamples loads the standard skin samples used as the final fallback for
// every hit-sound bank. Skin.GetSample owns this cache; this package only
// stores the references needed for bank resolution.
func LoadSamples() {
	var defaults [sampleSetCount][hitSoundCount]*bass.Sample

	for setIndex, setName := range []string{"normal", "soft", "drum"} {
		for soundIndex, soundName := range []string{
			"hitnormal",
			"hitwhistle",
			"hitfinish",
			"hitclap",
			"slidertick",
			"sliderslide",
			"sliderwhistle",
		} {
			defaults[setIndex][soundIndex] = LoadSample(setName + "-" + soundName)
		}
	}

	bank.mu.Lock()
	bank.defaults = defaults
	bank.mu.Unlock()
}

// PlaySample preserves the legacy immediate API for callers outside the
// gameplay path. Gameplay code should use PlaySampleAt with the event's
// nominal timestamp so late frames do not move hit sounds.
func PlaySample(sampleSet, additionSet, hitsound, index int, volume float64, objNum int64, xPos float64) {
	PlaySampleAt(currentGameplayTime(), sampleSet, additionSet, hitsound, index, volume, 0, objNum, xPos)
}

// PlaySampleAt plays all requested hit-sound components at eventTime in the
// gameplay timeline. customVolume follows osu!'s legacy rule: a positive
// per-object volume overrides the timing-point volume, while zero means use
// the timing point.
func PlaySampleAt(eventTime float64, sampleSet, additionSet, hitsound, index int, volume, customVolume float64, objNum int64, xPos float64) {
	if additionSet == 0 {
		additionSet = sampleSet
	}

	volume = effectiveSampleVolume(volume, customVolume)

	// Play normal. Layered skins request the normal layer even when the
	// object's hit-sound bitmask does not contain the normal bit.
	if skin.GetInfo().LayeredHitSounds || hitsound&1 > 0 || hitsound == 0 {
		playSampleAt(eventTime, sampleSet, 0, index, volume*0.8, objNum, xPos)
	}

	if hitsound&2 > 0 {
		playSampleAt(eventTime, additionSet, 1, index, volume*0.85, objNum, xPos)
	}

	if hitsound&4 > 0 {
		playSampleAt(eventTime, additionSet, 2, index, volume, objNum, xPos)
	}

	if hitsound&8 > 0 {
		playSampleAt(eventTime, additionSet, 3, index, volume*0.85, objNum, xPos)
	}
}

func playSampleAt(eventTime float64, sampleSet, hitsoundIndex, index int, volume float64, objNum int64, xPos float64) {
	sampleSet = normalizeSampleSet(sampleSet)
	hitsoundIndex = normalizeHitSoundIndex(hitsoundIndex)
	index = normalizeSampleIndex(index)

	balance := 0.0
	if settings.DIVIDES == 1 {
		balance = mutils.Clamp((xPos-256)/512*settings.Audio.HitsoundPositionMultiplier, -1, 1)
	}

	for _, listener := range snapshotListeners() {
		listener(sampleSet, hitsoundIndex, index, volume, objNum)
	}

	sample := resolveSample(sampleSet, hitsoundIndex, index)
	if sample == nil {
		return
	}

	sample.PlayRVPosAt(volume, balance, scheduledMixerTime(eventTime))
}

func snapshotListeners() []func(sampleSet int, hitsoundIndex, index int, volume float64, objNum int64) {
	listenersMu.RLock()
	defer listenersMu.RUnlock()

	return append([]func(sampleSet int, hitsoundIndex, index int, volume float64, objNum int64){}, listeners...)
}

func resolveSample(sampleSet, hitsoundIndex, index int) *bass.Sample {
	bank.mu.RLock()
	defer bank.mu.RUnlock()

	if !settings.Audio.IgnoreBeatmapSamples {
		if samples := bank.beatmap[sampleSet-1][hitsoundIndex]; samples != nil {
			if sample := samples[index]; sample != nil {
				return sample
			}
		}
	}

	return bank.defaults[sampleSet-1][hitsoundIndex]
}

func normalizeSampleSet(sampleSet int) int {
	if sampleSet == 0 {
		return 2
	}
	if sampleSet < 0 || sampleSet > sampleSetCount {
		return 1
	}
	return sampleSet
}

func normalizeHitSoundIndex(index int) int {
	if index < 0 || index >= hitSoundCount {
		return 0
	}
	return index
}

func normalizeSampleIndex(index int) int {
	if index < 0 {
		return 1
	}
	return index
}

// EffectiveSampleVolume applies the legacy precedence and the stable
// minimum-volume floor. IgnoreBeatmapSampleVolume intentionally ignores both
// timing-point and per-object values, but keeps component-specific weighting
// such as the normal-layer 0.8 multiplier.
func EffectiveSampleVolume(timingVolume, customVolume float64) float64 {
	return effectiveSampleVolume(timingVolume, customVolume)
}

func effectiveSampleVolume(timingVolume, customVolume float64) float64 {
	if settings.Audio.IgnoreBeatmapSampleVolume {
		return 1
	}

	volume := timingVolume
	if customVolume > 0 && finite(customVolume) {
		volume = customVolume
	}
	if !finite(volume) {
		volume = 1
	}

	return max(minimumVolume, min(1, volume))
}

// PlaySliderLoopsAt updates the continuous slider samples owned by state.
// Existing loops are rebalanced as the slider ball moves, matching lazer's
// continuously updated positional sample balance.
func PlaySliderLoopsAt(state *SliderLoopState, eventTime float64, sampleSet, additionSet, hitsound, index int, volume float64, objNum int64, xPos float64) {
	if state == nil {
		return
	}

	if additionSet == 0 {
		additionSet = sampleSet
	}

	sampleSet = normalizeSampleSet(sampleSet)
	additionSet = normalizeSampleSet(additionSet)
	index = normalizeSampleIndex(index)
	volume = effectiveSampleVolume(volume, 0)

	balance := 0.0
	if settings.DIVIDES == 1 {
		balance = mutils.Clamp((xPos-256)/512*settings.Audio.HitsoundPositionMultiplier, -1, 1)
	}

	if hitsound&2 > 0 {
		needsNew := state.whistle == nil || state.lastAdditionSet != additionSet || state.lastIndex != index
		if needsNew {
			stopLoop(&state.whistle)
			state.whistle = playSampleLoopAt(eventTime, additionSet, 6, index, volume, objNum, balance)
		} else {
			bass.SetSampleProperties(state.whistle, sampleOutputVolume(volume), balance)
		}
	} else {
		stopLoop(&state.whistle)
	}

	if hitsound&2 == 0 || skin.GetInfo().LayeredHitSounds {
		needsNew := state.slide == nil || state.lastSampleSet != sampleSet || state.lastIndex != index
		if needsNew {
			stopLoop(&state.slide)
			state.slide = playSampleLoopAt(eventTime, sampleSet, 5, index, volume, objNum, balance)
		} else {
			bass.SetSampleProperties(state.slide, sampleOutputVolume(volume), balance)
		}
	} else {
		stopLoop(&state.slide)
	}

	state.lastSampleSet = sampleSet
	state.lastAdditionSet = additionSet
	state.lastIndex = index
}

// PlaySliderLoops is the compatibility wrapper for code that does not have a
// separate timestamp. New slider code should pass its current event time.
func PlaySliderLoops(state *SliderLoopState, sampleSet, additionSet, hitsound, index int, volume float64, objNum int64, xPos float64) {
	PlaySliderLoopsAt(state, currentGameplayTime(), sampleSet, additionSet, hitsound, index, volume, objNum, xPos)
}

// StopSliderLoops stops only the loops owned by state.
func StopSliderLoops(state *SliderLoopState) {
	if state == nil {
		return
	}

	stopLoop(&state.whistle)
	stopLoop(&state.slide)
	state.lastSampleSet = 0
	state.lastAdditionSet = 0
	state.lastIndex = 0
}

func stopLoop(channel **bass.SampleChannel) {
	if channel == nil || *channel == nil {
		return
	}

	bass.StopSample(*channel)
	*channel = nil
}

func playSampleLoopAt(eventTime float64, sampleSet, hitsoundIndex, index int, volume float64, objNum int64, balance float64) *bass.SampleChannel {
	sampleSet = normalizeSampleSet(sampleSet)
	hitsoundIndex = normalizeHitSoundIndex(hitsoundIndex)
	index = normalizeSampleIndex(index)

	for _, listener := range snapshotListeners() {
		listener(sampleSet, hitsoundIndex, index, volume, objNum)
	}

	sample := resolveSample(sampleSet, hitsoundIndex, index)
	if sample == nil {
		return nil
	}

	return sample.PlayRVPosLoopAt(volume, balance, scheduledMixerTime(eventTime))
}

func sampleOutputVolume(volume float64) float64 {
	return settings.Audio.GeneralVolume * settings.Audio.SampleVolume * volume
}

// PlaySliderTickAt plays the tick at its nominal event timestamp.
func PlaySliderTickAt(eventTime float64, sampleSet, index int, volume float64, objNum int64, xPos float64) {
	playSampleAt(eventTime, sampleSet, 4, index, effectiveSampleVolume(volume, 0), objNum, xPos)
}

// PlaySliderTick preserves the old immediate API.
func PlaySliderTick(sampleSet, index int, volume float64, objNum int64, xPos float64) {
	PlaySliderTickAt(currentGameplayTime(), sampleSet, index, volume, objNum, xPos)
}

// PlayNamedSampleAt schedules a skin sample that is not part of the hit-sound
// bank, such as combo-break or storyboard audio.
func PlayNamedSampleAt(name string, eventTime, volume float64) {
	sample := LoadSample(name)
	if sample == nil {
		return
	}

	sample.PlayRVPosAt(volume, 0, scheduledMixerTime(eventTime))
}

// PlayNamedSample preserves the old immediate named-sample API.
func PlayNamedSample(name string, volume float64) {
	PlayNamedSampleAt(name, currentGameplayTime(), volume)
}

// PlayNamedLoopAt starts a reusable loop at an event timestamp.
func PlayNamedLoopAt(name string, eventTime, volume float64) *bass.SampleChannel {
	sample := LoadSample(name)
	if sample == nil {
		return nil
	}

	return sample.PlayRVPosLoopAt(volume, 0, scheduledMixerTime(eventTime))
}

// PlaySampleHandleAt schedules an already-resolved sample. It is used for
// storyboard and other map-local audio that is not part of the hit-sound
// naming convention.
func PlaySampleHandleAt(sample *bass.Sample, eventTime, volume float64) {
	if sample == nil {
		return
	}

	sample.PlayRVPosAt(volume, 0, scheduledMixerTime(eventTime))
}

func PauseLoop(channel *bass.SampleChannel) {
	bass.PauseSample(channel)
}

func ResumeLoop(channel *bass.SampleChannel) {
	bass.PlaySample(channel)
}

func StopLoop(channel *bass.SampleChannel) {
	bass.StopSample(channel)
}

func SetLoopRate(channel *bass.SampleChannel, rate float64) {
	bass.SetRate(channel, rate)
}

// LoadBeatmapSamples replaces the map-specific sample bank. Keys are parsed
// case-insensitively and sorted before loading so duplicate malformed names
// have deterministic last-writer behavior.
func LoadBeatmapSamples(fileMap map[string]string) {
	ClearBeatmapSamples()

	keys := make([]string, 0, len(fileMap))
	for name := range fileMap {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	for _, originalName := range keys {
		fileName := strings.TrimSpace(originalName)
		lowerName := strings.ToLower(fileName)
		if strings.ContainsAny(fileName, `/\\`) {
			continue
		}

		extension := ""
		for _, candidate := range []string{".wav", ".mp3", ".ogg"} {
			if strings.HasSuffix(lowerName, candidate) {
				extension = candidate
				break
			}
		}
		if extension == "" {
			continue
		}

		rawName := strings.TrimSuffix(lowerName, extension)
		parts := strings.Split(rawName, "-")
		if len(parts) != 2 {
			continue
		}

		setID := sets[parts[0]]
		if setID == 0 {
			continue
		}

		nameParts := splitBeforeDigit(parts[1])
		hitSoundIndex := 1
		if len(nameParts) > 1 {
			parsedIndex, err := strconv.ParseInt(nameParts[1], 10, 32)
			if err != nil || parsedIndex < 0 {
				continue
			}
			hitSoundIndex = int(parsedIndex)
		}

		hitSoundID := hitsounds[nameParts[0]]
		if hitSoundID == 0 {
			continue
		}

		sample := bass.NewSample(fileMap[originalName])
		if sample == nil {
			continue
		}

		var replaced *bass.Sample
		bank.mu.Lock()
		if bank.beatmap[setID-1][hitSoundID-1] == nil {
			bank.beatmap[setID-1][hitSoundID-1] = make(map[int]*bass.Sample)
		}
		replaced = bank.beatmap[setID-1][hitSoundID-1][hitSoundIndex]
		bank.beatmap[setID-1][hitSoundID-1][hitSoundIndex] = sample
		bank.mu.Unlock()

		if replaced != nil && replaced != sample {
			replaced.Close()
		}
	}
}

func splitBeforeDigit(name string) []string {
	for i, character := range name {
		if unicode.IsDigit(character) {
			return []string{name[:i], name[i:]}
		}
	}

	return []string{name}
}

// ClearBeatmapSamples releases all map-specific native handles. Skin and
// default samples remain owned by the skin cache and are not touched here.
func ClearBeatmapSamples() {
	// A map-specific sample can still have a future voice attached to the
	// mixer after its object has been discarded. Stop those voices before
	// releasing the sample handles, otherwise the mixer can observe a freed
	// sample while the next map is loading.
	bass.StopAllSamples()

	bank.mu.Lock()
	var samples []*bass.Sample
	for setIndex := range sampleSetCount {
		for soundIndex := range hitSoundCount {
			for _, sample := range bank.beatmap[setIndex][soundIndex] {
				if sample != nil {
					samples = append(samples, sample)
				}
			}
			bank.beatmap[setIndex][soundIndex] = nil
		}
	}
	bank.mu.Unlock()

	seen := make(map[*bass.Sample]struct{}, len(samples))
	for _, sample := range samples {
		if _, exists := seen[sample]; exists {
			continue
		}
		seen[sample] = struct{}{}
		sample.Close()
	}
}

func LoadSample(name string) *bass.Sample {
	return skin.GetSample(name)
}

func PlayFailSound() {
	PlayNamedSampleAt("failsound", currentGameplayTime(), 1)
}
