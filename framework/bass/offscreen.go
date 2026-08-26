package bass

/*
#include <stdint.h>
#include "bass.h"
#include "bassmix.h"
*/
import "C"
import (
	"unsafe"
)

func GetMixerRequiredBufferSize(seconds float64) int {
	return int(C.BASS_ChannelSeconds2Bytes(masterMixer, C.double(seconds)))
}

func ProcessMixer(buffer []byte) {
	C.BASS_ChannelGetData(masterMixer, unsafe.Pointer(&buffer[0]), C.DWORD(len(buffer)))
}

// MixerPosition returns how many seconds of audio the master mixer has
// produced so far. During offline rendering this is the authoritative
// gameplay clock: it counts exactly the samples already handed to ffmpeg,
// while source-channel positions report input-side timing that runs ahead
// by the tempo engine's internal window and decode buffering.
func MixerPosition() float64 {
	pos := C.BASS_ChannelGetPosition(masterMixer, C.BASS_POS_BYTE)
	return float64(C.BASS_ChannelBytes2Seconds(masterMixer, pos))
}
