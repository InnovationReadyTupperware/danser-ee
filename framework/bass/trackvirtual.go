package bass

import (
	"math"
	"sync"

	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
)

// TrackVirtual advances against the master output clock without producing
// audio. It is used for replay-only runs where the map has no music stream,
// so it follows the same output timeline as TrackBass without owning a native
// handle.
type TrackVirtual struct {
	mu sync.Mutex

	fft              []float32
	length           float64
	tail             float64
	speed            float64
	pitch            float64
	rFreq            float64
	previousPosition float64
	startTime        float64
	playing          bool
	paused           bool
	closed           bool
}

// NewTrackVirtual creates a silent clock-backed track.
func NewTrackVirtual(length float64) *TrackVirtual {
	if length < 0 || math.IsNaN(length) || math.IsInf(length, 0) {
		length = 0
	}

	return &TrackVirtual{
		fft:    make([]float32, 512),
		speed:  1,
		pitch:  1,
		rFreq:  1,
		length: max(0, length),
	}
}

// Close releases the virtual track. It is intentionally a no-op for resource
// symmetry with TrackBass, but it still prevents future state changes.
func (track *TrackVirtual) Close() {
	if track == nil {
		return
	}

	track.mu.Lock()
	track.closed = true
	track.playing = false
	track.paused = false
	track.mu.Unlock()
}

func (track *TrackVirtual) AddSilence(seconds float64) {
	if track == nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return
	}

	track.mu.Lock()
	if !track.closed {
		track.tail = max(0, seconds)
	}
	track.mu.Unlock()
}

func (track *TrackVirtual) Play() {
	track.playInternal()
}

func (track *TrackVirtual) PlayV(_ float64) {
	track.playInternal()
}

func (track *TrackVirtual) playInternal() {
	if track == nil {
		return
	}

	track.mu.Lock()
	defer track.mu.Unlock()

	if track.closed || track.playing {
		return
	}

	track.playing = true
	track.paused = false
	track.startTime = MixerPosition()
}

func (track *TrackVirtual) Pause() {
	if track == nil {
		return
	}

	track.mu.Lock()
	defer track.mu.Unlock()

	if track.closed || !track.playing {
		return
	}

	mixerTime := MixerPosition()
	track.previousPosition = track.positionAt(mixerTime)
	track.playing = false
	track.paused = true
}

func (track *TrackVirtual) Resume() {
	if track == nil {
		return
	}

	track.mu.Lock()
	defer track.mu.Unlock()

	if track.closed || !track.paused {
		return
	}

	track.startTime = MixerPosition()
	track.playing = true
	track.paused = false
}

func (track *TrackVirtual) Stop() {
	if track == nil {
		return
	}

	track.mu.Lock()
	track.playing = false
	track.paused = false
	track.previousPosition = 0
	track.mu.Unlock()
}

func (track *TrackVirtual) SetVolume(_ float64) {
}

func (track *TrackVirtual) SetVolumeRelative(_ float64) {
}

func (track *TrackVirtual) GetLength() float64 {
	if track == nil {
		return 0
	}

	track.mu.Lock()
	defer track.mu.Unlock()
	return track.length
}

func (track *TrackVirtual) SetPosition(pos float64) {
	if track == nil || math.IsNaN(pos) || math.IsInf(pos, 0) {
		return
	}

	track.mu.Lock()
	defer track.mu.Unlock()

	if track.closed {
		return
	}

	track.previousPosition = mutils.Clamp(pos, 0, track.length+track.tail)
	track.startTime = MixerPosition()
}

func (track *TrackVirtual) GetPosition() float64 {
	if track == nil {
		return 0
	}

	track.mu.Lock()
	defer track.mu.Unlock()
	return track.positionAt(MixerPosition())
}

func (track *TrackVirtual) positionAt(mixerTime float64) float64 {
	if !track.playing {
		return track.previousPosition
	}

	position := track.previousPosition + (mixerTime-track.startTime)*track.speed*track.rFreq
	return mutils.Clamp(position, 0, track.length+track.tail)
}

func (track *TrackVirtual) SetTempo(tempo float64) {
	if track == nil || math.IsNaN(tempo) || math.IsInf(tempo, 0) || tempo <= 0 {
		return
	}

	track.mu.Lock()
	defer track.mu.Unlock()

	if track.closed || track.speed == tempo {
		return
	}

	mixerTime := MixerPosition()
	track.previousPosition = track.positionAt(mixerTime)
	track.startTime = mixerTime
	track.speed = tempo
}

func (track *TrackVirtual) GetTempo() float64 {
	if track == nil {
		return 1
	}

	track.mu.Lock()
	defer track.mu.Unlock()
	return track.speed
}

func (track *TrackVirtual) SetPitch(pitch float64) {
	if track == nil || math.IsNaN(pitch) || math.IsInf(pitch, 0) {
		return
	}

	track.mu.Lock()
	track.pitch = pitch
	track.mu.Unlock()
}

func (track *TrackVirtual) GetPitch() float64 {
	if track == nil {
		return 1
	}

	track.mu.Lock()
	defer track.mu.Unlock()
	return track.pitch
}

func (track *TrackVirtual) SetRelativeFrequency(rFreq float64) {
	if track == nil || math.IsNaN(rFreq) || math.IsInf(rFreq, 0) || rFreq <= 0 {
		return
	}

	track.mu.Lock()
	defer track.mu.Unlock()

	if track.closed || track.rFreq == rFreq {
		return
	}

	mixerTime := MixerPosition()
	track.previousPosition = track.positionAt(mixerTime)
	track.startTime = mixerTime
	track.rFreq = rFreq
}

func (track *TrackVirtual) GetRelativeFrequency() float64 {
	if track == nil {
		return 1
	}

	track.mu.Lock()
	defer track.mu.Unlock()
	return track.rFreq
}

func (track *TrackVirtual) GetSpeed() float64 {
	if track == nil {
		return 1
	}

	track.mu.Lock()
	defer track.mu.Unlock()
	return track.speed * track.rFreq
}

func (track *TrackVirtual) GetState() int {
	if track == nil {
		return MusicStopped
	}

	track.mu.Lock()
	defer track.mu.Unlock()

	if track.closed || (!track.playing && !track.paused) {
		return MusicStopped
	}
	if track.paused {
		return MusicPaused
	}

	position := track.positionAt(MixerPosition())
	if position >= track.length+track.tail {
		track.playing = false
		track.previousPosition = track.length + track.tail
		return MusicStopped
	}

	return MusicPlaying
}

func (track *TrackVirtual) Update() {
}

func (track *TrackVirtual) GetFFT() []float32 {
	if track == nil {
		return nil
	}

	track.mu.Lock()
	defer track.mu.Unlock()
	return append([]float32(nil), track.fft...)
}

func (track *TrackVirtual) GetPeak() float64 {
	return 0
}

func (track *TrackVirtual) GetLevelCombined() float64 {
	return 0
}

func (track *TrackVirtual) GetLeftLevel() float64 {
	return 0
}

func (track *TrackVirtual) GetRightLevel() float64 {
	return 0
}

func (track *TrackVirtual) GetBoost() float64 {
	return 0
}

func (track *TrackVirtual) GetBeat() float64 {
	return 0
}
