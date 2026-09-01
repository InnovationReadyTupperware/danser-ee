package bass

/*
#include <stdlib.h>
#include "bass.h"
#include "bass_fx.h"
#include "bassmix.h"
*/
import "C"

import (
	"math"
	"runtime"
	"unicode/utf16"
	"unsafe"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

// TrackBass is a tempo-capable music stream attached to the master mixer.
// Native state is accessed only through the BASS executor; this matters when
// the gameplay clock, rendering worker, and mixer thread are active together.
type TrackBass struct {
	channel           C.HSTREAM
	fft               []float32
	boost             float64
	peak              float64
	leftChannel       float64
	rightChannel      float64
	lowMax            float64
	speed             float64
	pitch             float64
	playing           bool
	addedToMixer      bool
	baseFrequency     float64
	relativeFrequency float64
	closed            bool
}

// PlaybackSnapshot is one coherent observation of a realtime track. Grouping
// these values avoids repeated scheduler round trips to the BASS owner thread
// in the 1 kHz gameplay update loop.
type PlaybackSnapshot struct {
	State    int
	Position float64
	Speed    float64
}

// PlaybackClock describes the mixer and source rates after a playback-rate
// update. Both values come from the same audio-thread transaction.
type PlaybackClock struct {
	MixerPosition float64
	Speed         float64
}

// NewTrack loads a tempo-capable track and attaches it to the master mixer in
// a paused state. Attaching before playback permits seeks to use mixer-source
// positions without racing a later add operation.
func NewTrack(path string) *TrackBass {
	if path == "" {
		return nil
	}

	return runOnAudioThreadResult(func() *TrackBass {
		player := &TrackBass{
			fft:               make([]float32, 512),
			speed:             1,
			pitch:             1,
			relativeFrequency: 1,
		}

		flags := C.BASS_STREAM_DECODE | C.BASS_STREAM_PRESCAN

		if runtime.GOOS == "windows" {
			wFile := utf16.Encode([]rune(path))
			wFile = append(wFile, 0)

			player.channel = C.BASS_StreamCreateFile(0, unsafe.Pointer(&wFile[0]), 0, 0, C.DWORD(flags|C.BASS_UNICODE))
		} else {
			// Linux keeps ASYNCFILE because the Windows implementation has a
			// known interaction with the current BASS stream buffer settings.
			flags |= C.BASS_ASYNCFILE
			cPath := C.CString(path)
			player.channel = C.BASS_StreamCreateFile(0, unsafe.Pointer(cPath), 0, 0, C.DWORD(flags))
			C.free(unsafe.Pointer(cPath))
		}

		if player.channel == 0 {
			return nil
		}

		source := player.channel
		player.channel = C.BASS_FX_TempoCreate(source, C.BASS_FX_FREESOURCE|C.BASS_STREAM_DECODE)
		if player.channel == 0 {
			// BASS_FX_FREESOURCE only transfers ownership after a
			// successful wrapper creation. Free the original source on the
			// failure path so a corrupt or unsupported track cannot leak a
			// decoder handle.
			C.BASS_ChannelFree(source)
			return nil
		}

		setupFXChannel(player.channel)

		var freq C.float
		if C.BASS_ChannelGetAttribute(player.channel, C.BASS_ATTRIB_FREQ, &freq) == 0 {
			C.BASS_ChannelFree(player.channel)
			player.channel = 0
			return nil
		}

		player.baseFrequency = float64(freq)
		if player.baseFrequency <= 0 {
			player.baseFrequency = float64(sampleRate)
		}

		if !addTrackToMixerInternal(player) {
			C.BASS_ChannelFree(player.channel)
			player.channel = 0
			return nil
		}

		C.BASS_Mixer_ChannelFlags(player.channel, C.BASS_MIXER_CHAN_PAUSE, C.BASS_MIXER_CHAN_PAUSE)
		if C.BASS_Mixer_ChannelFlags(player.channel, 0, 0)&C.BASS_MIXER_CHAN_PAUSE == 0 {
			C.BASS_Mixer_ChannelRemove(player.channel)
			C.BASS_ChannelFree(player.channel)
			player.channel = 0
			return nil
		}

		return player
	})
}

func setupFXChannel(channel C.HSTREAM) {
	C.BASS_ChannelSetAttribute(channel, C.BASS_ATTRIB_TEMPO_OPTION_OLDPOS, 1)
	C.BASS_ChannelSetAttribute(channel, C.BASS_ATTRIB_TEMPO_OPTION_USE_QUICKALGO, 1)
	C.BASS_ChannelSetAttribute(channel, C.BASS_ATTRIB_TEMPO_OPTION_OVERLAP_MS, C.float(4.0))
	C.BASS_ChannelSetAttribute(channel, C.BASS_ATTRIB_TEMPO_OPTION_SEQUENCE_MS, C.float(30.0))
}

func addTrackToMixerInternal(track *TrackBass) bool {
	if track == nil || track.channel == 0 || track.closed {
		return false
	}

	if track.addedToMixer {
		return true
	}

	if masterMixer == 0 || C.BASS_Mixer_StreamAddChannel(masterMixer, track.channel, C.BASS_MIXER_CHAN_NORAMPIN|C.BASS_MIXER_CHAN_BUFFER) == 0 {
		return false
	}

	track.addedToMixer = true
	return true
}

// Close removes and frees the native track. Repeated calls are safe.
func (track *TrackBass) Close() {
	if track == nil {
		return
	}

	runOnAudioThread(func() {
		if track.closed {
			return
		}
		track.closed = true
		track.playing = false

		if track.channel == 0 {
			return
		}

		if track.addedToMixer {
			C.BASS_Mixer_ChannelRemove(track.channel)
			track.addedToMixer = false
		}

		C.BASS_ChannelFree(track.channel)
		track.channel = 0
	})
}

func (track *TrackBass) AddSilence(seconds float64) {
	if track == nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return
	}

	runOnAudioThread(func() {
		if track.channel != 0 && !track.closed {
			C.BASS_ChannelSetAttribute(track.channel, C.BASS_ATTRIB_TAIL, C.float(max(0, seconds)))
		}
	})
}

func (track *TrackBass) Play() {
	track.playWithVolume(settings.Audio.GeneralVolume * settings.Audio.MusicVolume)
}

func (track *TrackBass) PlayV(volume float64) {
	track.playWithVolume(volume)
}

func (track *TrackBass) playWithVolume(volume float64) {
	if track == nil {
		return
	}

	runOnAudioThread(func() {
		if track.channel == 0 || track.closed {
			return
		}

		if !addTrackToMixerInternal(track) {
			return
		}

		C.BASS_ChannelSetAttribute(track.channel, C.BASS_ATTRIB_VOL, C.float(volume))
		C.BASS_Mixer_ChannelFlags(track.channel, 0, C.BASS_MIXER_CHAN_PAUSE)
		track.playing = true
	})
}

func (track *TrackBass) Pause() {
	if track == nil {
		return
	}

	runOnAudioThread(func() {
		if track.channel == 0 || track.closed || !track.addedToMixer {
			return
		}

		C.BASS_Mixer_ChannelFlags(track.channel, C.BASS_MIXER_CHAN_PAUSE, C.BASS_MIXER_CHAN_PAUSE)
		track.playing = false
	})
}

func (track *TrackBass) Resume() {
	if track == nil {
		return
	}

	runOnAudioThread(func() {
		if track.channel == 0 || track.closed || !track.addedToMixer {
			return
		}

		C.BASS_Mixer_ChannelFlags(track.channel, 0, C.BASS_MIXER_CHAN_PAUSE)
		track.playing = true
	})
}

func (track *TrackBass) Stop() {
	if track == nil {
		return
	}

	runOnAudioThread(func() {
		if track.channel == 0 || track.closed {
			return
		}

		track.playing = false
		if track.addedToMixer {
			C.BASS_Mixer_ChannelRemove(track.channel)
			track.addedToMixer = false
		}
		// Stop is a restart boundary for the track API. Resetting the source
		// position preserves the old behavior when a caller later invokes Play
		// again instead of resuming from the previous end position.
		C.BASS_ChannelStop(track.channel)
	})
}

func (track *TrackBass) SetVolume(vol float64) {
	if track == nil {
		return
	}

	runOnAudioThread(func() {
		if track.channel != 0 && !track.closed {
			C.BASS_ChannelSetAttribute(track.channel, C.BASS_ATTRIB_VOL, C.float(vol))
		}
	})
}

func (track *TrackBass) SetVolumeRelative(vol float64) {
	track.SetVolume(settings.Audio.GeneralVolume * settings.Audio.MusicVolume * vol)
}

func (track *TrackBass) GetLength() float64 {
	if track == nil {
		return 0
	}

	return runOnAudioThreadResult(func() float64 {
		if track.channel == 0 || track.closed {
			return 0
		}

		length := C.BASS_ChannelGetLength(track.channel, C.BASS_POS_BYTE)
		if uint64(length) == ^uint64(0) {
			return 0
		}

		return float64(C.BASS_ChannelBytes2Seconds(track.channel, length))
	})
}

func (track *TrackBass) SetPosition(pos float64) {
	if track == nil || math.IsNaN(pos) || math.IsInf(pos, 0) {
		return
	}

	runOnAudioThread(func() {
		if track.channel == 0 || track.closed {
			return
		}

		pos = max(0, pos)
		bytes := C.BASS_ChannelSeconds2Bytes(track.channel, C.double(pos))
		if track.addedToMixer {
			C.BASS_Mixer_ChannelSetPosition(track.channel, bytes, C.BASS_POS_BYTE)
		} else {
			C.BASS_ChannelSetPosition(track.channel, bytes, C.BASS_POS_BYTE)
		}
	})
}

func (track *TrackBass) GetPosition() float64 {
	if track == nil {
		return 0
	}

	return runOnAudioThreadResult(track.positionInternal)
}

func (track *TrackBass) positionInternal() float64 {
	if track.channel == 0 || track.closed {
		return 0
	}

	var pos C.QWORD
	if track.addedToMixer {
		pos = C.BASS_Mixer_ChannelGetPosition(track.channel, C.BASS_POS_BYTE)
	} else {
		pos = C.BASS_ChannelGetPosition(track.channel, C.BASS_POS_BYTE)
	}

	if uint64(pos) == ^uint64(0) {
		return 0
	}

	return float64(C.BASS_ChannelBytes2Seconds(track.channel, pos))
}

func (track *TrackBass) SetTempo(tempo float64) {
	if track == nil || math.IsNaN(tempo) || math.IsInf(tempo, 0) || tempo <= 0 {
		return
	}

	runOnAudioThread(func() { track.setTempoInternal(tempo) })
}

func (track *TrackBass) setTempoInternal(tempo float64) {
	if track.channel == 0 || track.closed || track.speed == tempo {
		return
	}

	track.speed = tempo
	C.BASS_ChannelSetAttribute(track.channel, C.BASS_ATTRIB_TEMPO, C.float((tempo-1.0)*100))
}

func (track *TrackBass) GetTempo() float64 {
	if track == nil {
		return 1
	}

	return runOnAudioThreadResult(func() float64 { return track.speed })
}

func (track *TrackBass) SetPitch(pitch float64) {
	if track == nil || math.IsNaN(pitch) || math.IsInf(pitch, 0) {
		return
	}

	runOnAudioThread(func() { track.setPitchInternal(pitch) })
}

func (track *TrackBass) setPitchInternal(pitch float64) {
	if track.channel == 0 || track.closed || track.pitch == pitch {
		return
	}

	track.pitch = pitch
	C.BASS_ChannelSetAttribute(track.channel, C.BASS_ATTRIB_TEMPO_PITCH, C.float((pitch-1.0)*14.4))
}

func (track *TrackBass) GetPitch() float64 {
	if track == nil {
		return 1
	}

	return runOnAudioThreadResult(func() float64 { return track.pitch })
}

func (track *TrackBass) SetRelativeFrequency(rFreq float64) {
	if track == nil || math.IsNaN(rFreq) || math.IsInf(rFreq, 0) || rFreq <= 0 {
		return
	}

	runOnAudioThread(func() { track.setRelativeFrequencyInternal(rFreq) })
}

func (track *TrackBass) setRelativeFrequencyInternal(relativeFrequency float64) {
	if track.channel == 0 || track.closed || track.relativeFrequency == relativeFrequency {
		return
	}

	track.relativeFrequency = relativeFrequency
	C.BASS_ChannelSetAttribute(track.channel, C.BASS_ATTRIB_FREQ, C.float(relativeFrequency*track.baseFrequency))
}

func (track *TrackBass) GetRelativeFrequency() float64 {
	if track == nil {
		return 1
	}

	return runOnAudioThreadResult(func() float64 { return track.relativeFrequency })
}

func (track *TrackBass) GetSpeed() float64 {
	if track == nil {
		return 1
	}

	return runOnAudioThreadResult(func() float64 { return track.speed * track.relativeFrequency })
}

func (track *TrackBass) GetState() int {
	if track == nil {
		return MusicStopped
	}

	return runOnAudioThreadResult(track.stateInternal)
}

func (track *TrackBass) stateInternal() int {
	if track.channel == 0 || track.closed || !track.addedToMixer {
		return MusicStopped
	}

	state := int(C.BASS_ChannelIsActive(track.channel))
	if state == MusicPlaying && C.BASS_Mixer_ChannelFlags(track.channel, 0, 0)&C.BASS_MIXER_CHAN_PAUSE > 0 {
		return MusicPaused
	}

	return state
}

// SnapshotPlayback reads the state, source position, and effective playback
// speed in one trip to the BASS owner thread.
func (track *TrackBass) SnapshotPlayback() PlaybackSnapshot {
	if track == nil {
		return PlaybackSnapshot{Speed: 1}
	}

	return runOnAudioThreadResult(func() PlaybackSnapshot {
		return PlaybackSnapshot{
			State:    track.stateInternal(),
			Position: track.positionInternal(),
			Speed:    track.speed * track.relativeFrequency,
		}
	})
}

// ApplyPlaybackRate applies adjacent rate controls and returns the resulting
// output/source clock relationship without releasing the audio thread between
// operations.
func (track *TrackBass) ApplyPlaybackRate(tempo, pitch, relativeFrequency float64) PlaybackClock {
	if track == nil {
		return PlaybackClock{Speed: 1}
	}

	return runOnAudioThreadResult(func() PlaybackClock {
		if !math.IsNaN(tempo) && !math.IsInf(tempo, 0) && tempo > 0 {
			track.setTempoInternal(tempo)
		}
		if !math.IsNaN(pitch) && !math.IsInf(pitch, 0) {
			track.setPitchInternal(pitch)
		}
		if !math.IsNaN(relativeFrequency) && !math.IsInf(relativeFrequency, 0) && relativeFrequency > 0 {
			track.setRelativeFrequencyInternal(relativeFrequency)
		}

		return PlaybackClock{
			MixerPosition: mixerPositionSecondsInternal(),
			Speed:         track.speed * track.relativeFrequency,
		}
	})
}

// SetVolumeRelativeIfPlaying updates the realtime volume only while the track
// is active, keeping the state check and mutation in one audio transaction.
func (track *TrackBass) SetVolumeRelativeIfPlaying(volume float64) {
	if track == nil {
		return
	}

	volume *= settings.Audio.GeneralVolume * settings.Audio.MusicVolume
	runOnAudioThread(func() {
		if track.stateInternal() == MusicPlaying {
			C.BASS_ChannelSetAttribute(track.channel, C.BASS_ATTRIB_VOL, C.float(volume))
		}
	})
}

func (track *TrackBass) Update() {
	if track == nil {
		return
	}

	runOnAudioThread(track.updateAnalysisInternal)
}

// UpdateAnalysis refreshes the FFT and level data and returns the beat boost
// in the same audio-thread transaction.
func (track *TrackBass) UpdateAnalysis() float64 {
	if track == nil {
		return 0
	}

	return runOnAudioThreadResult(func() float64 {
		track.updateAnalysisInternal()
		return track.boost
	})
}

func (track *TrackBass) updateAnalysisInternal() {
	if track.channel == 0 || track.closed {
		return
	}

	if track.playing {
		if track.addedToMixer {
			C.BASS_Mixer_ChannelGetData(track.channel, unsafe.Pointer(&track.fft[0]), C.BASS_DATA_FFT1024)
		} else {
			C.BASS_ChannelGetData(track.channel, unsafe.Pointer(&track.fft[0]), C.BASS_DATA_FFT1024)
		}
	} else {
		for i := range track.fft {
			track.fft[i] = 0
		}
	}

	toPeak := 0.0
	beatAv := 0.0
	for i, value := range track.fft {
		h := math.Abs(float64(value))
		toPeak = max(toPeak, h)
		if i > 0 && i < 5 {
			beatAv = max(beatAv, float64(value))
		}
	}

	boost := 0.0
	for i := 0; i < 10; i++ {
		boost += float64(track.fft[i]*track.fft[i]) * float64(10-i) / float64(10)
	}

	track.lowMax = beatAv
	track.boost = boost
	track.peak = toPeak

	level := 0
	if track.playing {
		if track.addedToMixer {
			level = int(C.BASS_Mixer_ChannelGetLevel(track.channel))
		} else {
			level = int(C.BASS_ChannelGetLevel(track.channel))
		}
	}

	left := level & 65535
	right := level >> 16
	track.leftChannel = float64(left) / 32768
	track.rightChannel = float64(right) / 32768
}

func (track *TrackBass) GetFFT() []float32 {
	if track == nil {
		return nil
	}

	return runOnAudioThreadResult(func() []float32 {
		return append([]float32(nil), track.fft...)
	})
}

func (track *TrackBass) GetPeak() float64 {
	if track == nil {
		return 0
	}
	return runOnAudioThreadResult(func() float64 { return track.peak })
}

func (track *TrackBass) GetLevelCombined() float64 {
	if track == nil {
		return 0
	}
	return runOnAudioThreadResult(func() float64 { return (track.leftChannel + track.rightChannel) / 2 })
}

func (track *TrackBass) GetLeftLevel() float64 {
	if track == nil {
		return 0
	}
	return runOnAudioThreadResult(func() float64 { return track.leftChannel })
}

func (track *TrackBass) GetRightLevel() float64 {
	if track == nil {
		return 0
	}
	return runOnAudioThreadResult(func() float64 { return track.rightChannel })
}

func (track *TrackBass) GetBoost() float64 {
	if track == nil {
		return 0
	}
	return runOnAudioThreadResult(func() float64 { return track.boost })
}

func (track *TrackBass) GetBeat() float64 {
	if track == nil {
		return 0
	}
	return runOnAudioThreadResult(func() float64 { return track.lowMax })
}
