package bass

/*
#include <stdint.h>
#include "bass.h"
#include "bassmix.h"
*/
import "C"
import (
	"math"
	"unsafe"
)

func GetMixerRequiredBufferSize(seconds float64) int {
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0
	}

	return runOnAudioThreadResult(func() int {
		if masterMixer == 0 {
			return 0
		}

		bytes := C.BASS_ChannelSeconds2Bytes(masterMixer, C.double(seconds))
		if uint64(bytes) == ^uint64(0) || uint64(bytes) > uint64(^uint(0)>>1) {
			return 0
		}

		return int(bytes)
	})
}

// ProcessMixer fills buffer with the next master-mixer output block and
// returns the number of bytes BASS produced. Decode mixers can return fewer
// bytes at end of stream, so the remainder is explicitly zeroed before the
// caller hands the fixed-size block to an encoder.
func ProcessMixer(buffer []byte) int {
	if len(buffer) == 0 {
		return 0
	}

	return runOnAudioThreadResult(func() int {
		if masterMixer == 0 {
			clear(buffer)
			return 0
		}

		clear(buffer)
		written := C.BASS_ChannelGetData(masterMixer, unsafe.Pointer(&buffer[0]), C.DWORD(len(buffer)))
		if uint32(written) == ^uint32(0) {
			return 0
		}

		return min(int(written), len(buffer))
	})
}

// MixerPosition returns how many seconds of audio the master mixer has
// produced so far. During offline rendering this is the authoritative
// gameplay clock: it counts exactly the samples already handed to ffmpeg,
// while source-channel positions report input-side timing that runs ahead
// by the tempo engine's internal window and decode buffering.
func MixerPosition() float64 {
	return runOnAudioThreadResult(mixerPositionSecondsInternal)
}

func mixerPositionSecondsInternal() float64 {
	if masterMixer == 0 {
		return 0
	}

	pos := C.BASS_ChannelGetPosition(masterMixer, C.BASS_POS_BYTE)
	if uint64(pos) == ^uint64(0) {
		return 0
	}

	return float64(C.BASS_ChannelBytes2Seconds(masterMixer, pos))
}

// MixerPositionBytes returns the absolute output position used by BASSmix
// scheduling. It is kept separate from seconds because StreamAddChannelEx
// takes mixer-output bytes and converting twice can introduce a rounding
// error for short samples.
func MixerPositionBytes() uint64 {
	return runOnAudioThreadResult(func() uint64 {
		if masterMixer == 0 {
			return 0
		}

		pos := C.BASS_ChannelGetPosition(masterMixer, C.BASS_POS_BYTE)
		if uint64(pos) == ^uint64(0) {
			return 0
		}

		return uint64(pos)
	})
}

// OutputBytesForSeconds converts an output duration using the configured
// mixer format. It is a pure helper for timeline tests and callers that need
// to reason about scheduling without touching a native handle.
func OutputBytesForSeconds(seconds float64) int64 {
	info := GetOutputInfo()
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) || info.SampleRate <= 0 || info.Channels <= 0 || info.BytesPerSample <= 0 {
		return 0
	}

	bytesPerSecond := float64(info.SampleRate) * float64(info.Channels) * float64(info.BytesPerSample)
	bytes := seconds * bytesPerSecond
	if bytes >= float64(math.MaxInt64) {
		return 0
	}

	return int64(bytes)
}
