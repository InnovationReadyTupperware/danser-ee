package bass

/*
#include "bass.h"
#include "bassmix.h"
*/
import "C"

import (
	"io"
	"math"
	"os"
	"sync"
	"unsafe"

	"github.com/wieku/danser-go/app/settings"
)

// SampleChannel identifies a playing sample voice. One-shot streams are
// automatically freed by the mixer when they finish, while looping streams
// remain owned by the caller until StopSample is called. The private timing
// fields let teardown cancel a stream that has not reached its scheduled
// start yet without probing a possibly already auto-freed native handle.
type SampleChannel struct {
	source            C.HSAMPLE
	channel           C.HSTREAM
	autoFree          bool
	startMixerSeconds float64
	endMixerSeconds   float64
}

// Sample is an immutable BASS sample handle. The handle is released by Close
// when its owning sample registry is torn down.
type Sample struct {
	mu         sync.RWMutex
	bassSample C.HSAMPLE
	silent     bool
	closed     bool
}

var (
	loopingStreamsMu sync.Mutex
	loopingStreams   = make(map[*SampleChannel]struct{})
	oneShotStreamsMu sync.Mutex
	oneShotStreams   = make(map[*SampleChannel]struct{})
)

// StopLoops stops every active looping sample. It is used during map/player
// teardown and must not depend on gameplay objects still accepting audio
// submissions.
func StopLoops() {
	runOnAudioThread(func() {
		channels := snapshotChannels(&loopingStreamsMu, loopingStreams)

		for _, channel := range channels {
			stopSampleInternal(channel)
		}
	})
}

// StopAllSamples stops looping and one-shot sample channels. One-shot voices
// are normally released by BASS when they finish, but pending voices remain
// tracked until then so a seek or map transition can remove them before they
// reach the mixer.
func StopAllSamples() {
	runOnAudioThread(stopAllSamplesInternal)
}

func stopAllSamplesInternal() {
	channels := snapshotChannels(&loopingStreamsMu, loopingStreams)
	oneShots := snapshotChannels(&oneShotStreamsMu, oneShotStreams)

	for _, channel := range channels {
		stopSampleInternal(channel)
	}
	for _, channel := range oneShots {
		stopSampleInternal(channel)
	}
}

func snapshotChannels(mu *sync.Mutex, channels map[*SampleChannel]struct{}) []*SampleChannel {
	mu.Lock()
	defer mu.Unlock()

	snapshot := make([]*SampleChannel, 0, len(channels))
	for channel := range channels {
		snapshot = append(snapshot, channel)
	}

	return snapshot
}

// NewSample loads an encoded audio file into a reusable BASS sample. File
// errors and invalid audio are reported as a nil sample so a broken optional
// skin asset cannot crash gameplay.
func NewSample(path string) *Sample {
	if path == "" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil
	}

	return NewSampleData(data)
}

// NewSampleData loads an encoded audio buffer. Empty legacy WAV placeholders
// are represented as silent samples rather than fake native channels. This
// preserves skins that intentionally disable slider-loop sounds while
// allowing valid short samples to be loaded normally.
func NewSampleData(data []byte) *Sample {
	if len(data) == 0 {
		return nil
	}

	if isEmptyWAV(data) {
		return &Sample{silent: true}
	}

	handle := runOnAudioThreadResult(func() C.HSAMPLE {
		return C.BASS_SampleLoad(1, unsafe.Pointer(&data[0]), 0, C.DWORD(len(data)), 32, C.BASS_SAMPLE_OVER_POS)
	})

	if handle == 0 {
		return nil
	}

	return &Sample{bassSample: handle}
}

// isEmptyWAV detects the 44-byte RIFF placeholders used by the default skin.
// It deliberately does not classify arbitrary short buffers as silence: a
// valid encoded sample can be small, and BASS is the authority for decoding
// those formats.
func isEmptyWAV(data []byte) bool {
	return len(data) == 44 &&
		string(data[0:4]) == "RIFF" &&
		string(data[8:12]) == "WAVE"
}

// Close releases the native sample handle. It is safe to call repeatedly;
// callers must stop all voices created from the sample before closing it.
func (sample *Sample) Close() {
	if sample == nil {
		return
	}

	// Mark the object closed before submitting the native free. This keeps a
	// concurrent Play from claiming the handle while Close is waiting for the
	// serialized audio thread, and it also makes Close useful after BASS has
	// already been shut down (where there is no native call left to make).
	sample.mu.Lock()
	if sample.closed {
		sample.mu.Unlock()
		return
	}
	sample.closed = true
	handle := sample.bassSample
	sample.bassSample = 0
	sample.mu.Unlock()

	if handle != 0 {
		runOnAudioThread(func() {
			C.BASS_SampleFree(handle)
		})
	}
}

// GetLength returns the decoded sample duration in seconds.
func (sample *Sample) GetLength() float64 {
	if sample == nil || sample.silent {
		return 0
	}

	return runOnAudioThreadResult(func() float64 {
		sample.mu.RLock()
		defer sample.mu.RUnlock()

		if sample.closed || sample.bassSample == 0 {
			return 0
		}

		length := C.BASS_ChannelGetLength(sample.bassSample, C.BASS_POS_BYTE)
		if uint64(length) == ^uint64(0) {
			return 0
		}

		return float64(C.BASS_ChannelBytes2Seconds(sample.bassSample, length))
	})
}

// Play starts a sample at the current mixer position.
func (sample *Sample) Play() *SampleChannel {
	return sample.PlayRV(1)
}

// PlayLoop starts a looping sample at the current mixer position.
func (sample *Sample) PlayLoop() *SampleChannel {
	return sample.playAt(settings.Audio.GeneralVolume*settings.Audio.SampleVolume, 0, -1, true)
}

// PlayV starts a sample using the supplied post-mixer volume.
func (sample *Sample) PlayV(volume float64) *SampleChannel {
	return sample.playAt(volume, 0, -1, false)
}

// PlayVLoop starts a looping sample using the supplied post-mixer volume.
func (sample *Sample) PlayVLoop(volume float64) *SampleChannel {
	return sample.playAt(volume, 0, -1, true)
}

// PlayRV applies the configured general and sample volumes before playback.
func (sample *Sample) PlayRV(volume float64) *SampleChannel {
	return sample.PlayV(settings.Audio.GeneralVolume * settings.Audio.SampleVolume * volume)
}

// PlayRVLoop applies configured volume and starts a looping sample.
func (sample *Sample) PlayRVLoop(volume float64) *SampleChannel {
	return sample.playAt(settings.Audio.GeneralVolume*settings.Audio.SampleVolume*volume, 0, -1, true)
}

// PlayRVPos plays a sample immediately with configured volume and stereo pan.
func (sample *Sample) PlayRVPos(volume float64, balance float64) *SampleChannel {
	return sample.playAt(settings.Audio.GeneralVolume*settings.Audio.SampleVolume*volume, balance, -1, false)
}

// PlayRVPosAt schedules a sample at an absolute master-mixer position in
// seconds. If the requested position is already behind the mixer frontier,
// playback starts at the next available mixer position.
func (sample *Sample) PlayRVPosAt(volume float64, balance float64, mixerSeconds float64) *SampleChannel {
	return sample.playAt(settings.Audio.GeneralVolume*settings.Audio.SampleVolume*volume, balance, mixerSeconds, false)
}

// PlayRVPosLoop plays a looping sample immediately with configured volume and
// stereo pan.
func (sample *Sample) PlayRVPosLoop(volume float64, balance float64) *SampleChannel {
	return sample.PlayRVPosLoopAt(volume, balance, -1)
}

// PlayRVPosLoopAt starts a looping sample at an absolute mixer position.
func (sample *Sample) PlayRVPosLoopAt(volume float64, balance float64, mixerSeconds float64) *SampleChannel {
	return sample.playAt(settings.Audio.GeneralVolume*settings.Audio.SampleVolume*volume, balance, mixerSeconds, true)
}

func (sample *Sample) playAt(volume, balance, mixerSeconds float64, looping bool) *SampleChannel {
	if sample == nil || sample.silent {
		return nil
	}

	return runOnAudioThreadResult(func() *SampleChannel {
		cleanupFinishedOneShotStreamsInternal()

		sample.mu.RLock()
		defer sample.mu.RUnlock()

		if sample.closed || sample.bassSample == 0 {
			return nil
		}

		channel := &SampleChannel{source: sample.bassSample}
		channel.channel = C.BASS_SampleGetChannel(channel.source, C.BASS_SAMCHAN_STREAM|C.BASS_STREAM_DECODE)
		if channel.channel == 0 {
			return nil
		}

		C.BASS_ChannelSetAttribute(channel.channel, C.BASS_ATTRIB_VOL, C.float(volume))
		C.BASS_ChannelSetAttribute(channel.channel, C.BASS_ATTRIB_PAN, C.float(balance))

		flags := C.DWORD(C.BASS_MIXER_CHAN_NORAMPIN)
		if looping {
			// Set looping before the channel is attached. A very short sample can
			// finish between two executor submissions if this is done afterward.
			C.BASS_ChannelFlags(channel.channel, C.BASS_SAMPLE_LOOP, C.BASS_SAMPLE_LOOP)
		} else {
			// BASS_STREAM_AUTOFREE is a mixer-source flag here, not a
			// BASS_SampleGetChannel flag. It gives BASS ownership after the
			// one-shot reaches its end while the map below still lets us cancel
			// a voice that is waiting for an absolute mixer start position.
			flags |= C.BASS_STREAM_AUTOFREE
			channel.autoFree = true
		}

		var added C.BOOL
		current := mixerPositionSecondsInternal()
		channel.startMixerSeconds = current
		if mixerSeconds >= 0 {
			if mixerSeconds > current {
				channel.startMixerSeconds = mixerSeconds
				startBytes := C.BASS_ChannelSeconds2Bytes(masterMixer, C.double(mixerSeconds))
				added = C.BASS_Mixer_StreamAddChannelEx(masterMixer, channel.channel, flags|C.BASS_MIXER_CHAN_ABSOLUTE, startBytes, 0)
			} else {
				added = C.BASS_Mixer_StreamAddChannel(masterMixer, channel.channel, flags)
			}
		} else {
			added = C.BASS_Mixer_StreamAddChannel(masterMixer, channel.channel, flags)
		}

		if added == 0 {
			C.BASS_ChannelFree(channel.channel)
			return nil
		}

		if looping {
			loopingStreamsMu.Lock()
			loopingStreams[channel] = struct{}{}
			loopingStreamsMu.Unlock()
		} else {
			length := C.BASS_ChannelGetLength(channel.source, C.BASS_POS_BYTE)
			if uint64(length) != ^uint64(0) {
				duration := float64(C.BASS_ChannelBytes2Seconds(channel.source, length))
				if duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) {
					channel.endMixerSeconds = channel.startMixerSeconds + duration
				}
			}

			oneShotStreamsMu.Lock()
			oneShotStreams[channel] = struct{}{}
			oneShotStreamsMu.Unlock()
		}

		return channel
	})
}

func cleanupFinishedOneShotStreamsInternal() {
	current := mixerPositionSecondsInternal()
	channels := snapshotChannels(&oneShotStreamsMu, oneShotStreams)
	for _, channel := range channels {
		if channel == nil || channel.channel == 0 {
			forgetOneShotInternal(channel)
			continue
		}

		// A scheduled mixer source reports an inactive state until its start
		// delay expires. Never interpret that state as completion, or a dense
		// frame can delete every future hit sound before it is heard.
		if current < channel.startMixerSeconds {
			continue
		}

		if channel.endMixerSeconds > 0 && current < channel.endMixerSeconds {
			continue
		}

		if C.BASS_ChannelIsActive(channel.channel) == C.BASS_ACTIVE_STOPPED {
			// The mixer owns this stream after BASS_STREAM_AUTOFREE was
			// attached. Do not call BASS_ChannelFree on a handle that may
			// already have been freed asynchronously.
			forgetOneShotInternal(channel)
		}
	}
}

func forgetOneShotInternal(channel *SampleChannel) {
	if channel == nil {
		return
	}

	oneShotStreamsMu.Lock()
	delete(oneShotStreams, channel)
	oneShotStreamsMu.Unlock()
	channel.channel = 0
}

// SetRate changes a playing sample's frequency in Hz.
func SetRate(channel *SampleChannel, rate float64) {
	if channel == nil {
		return
	}

	runOnAudioThread(func() {
		if channel.channel != 0 {
			C.BASS_ChannelSetAttribute(channel.channel, C.BASS_ATTRIB_FREQ, C.float(rate))
		}
	})
}

// SetSampleProperties updates a playing sample's volume and stereo balance.
func SetSampleProperties(channel *SampleChannel, volume, balance float64) {
	if channel == nil {
		return
	}

	runOnAudioThread(func() {
		if channel.channel != 0 {
			C.BASS_ChannelSetAttribute(channel.channel, C.BASS_ATTRIB_VOL, C.float(volume))
			C.BASS_ChannelSetAttribute(channel.channel, C.BASS_ATTRIB_PAN, C.float(balance))
		}
	})
}

// StopSample stops and releases a sample channel. It is safe for nil or
// already-stopped channels, which is important when callers race natural
// one-shot completion with a seek or teardown.
func StopSample(channel *SampleChannel) {
	if channel == nil {
		return
	}

	runOnAudioThread(func() { stopSampleInternal(channel) })
}

func stopSampleInternal(channel *SampleChannel) {
	if channel == nil {
		return
	}

	loopingStreamsMu.Lock()
	delete(loopingStreams, channel)
	loopingStreamsMu.Unlock()
	oneShotStreamsMu.Lock()
	delete(oneShotStreams, channel)
	oneShotStreamsMu.Unlock()

	if channel.channel == 0 {
		return
	}

	C.BASS_Mixer_ChannelRemove(channel.channel)
	if !channel.autoFree {
		C.BASS_ChannelFree(channel.channel)
	}
	channel.channel = 0
}

// PauseSample pauses a sample loop without releasing it.
func PauseSample(channel *SampleChannel) {
	if channel == nil {
		return
	}

	runOnAudioThread(func() {
		if channel.channel != 0 {
			C.BASS_Mixer_ChannelFlags(channel.channel, C.BASS_MIXER_CHAN_PAUSE, C.BASS_MIXER_CHAN_PAUSE)
		}
	})
}

// PlaySample resumes a paused sample loop.
func PlaySample(channel *SampleChannel) {
	if channel == nil {
		return
	}

	runOnAudioThread(func() {
		if channel.channel != 0 {
			C.BASS_Mixer_ChannelFlags(channel.channel, 0, C.BASS_MIXER_CHAN_PAUSE)
		}
	})
}
