package bass

/*
#cgo CFLAGS: -I/usr/include -I.
#cgo LDFLAGS: -Wl,-rpath,$ORIGIN -L${SRCDIR} -L${SRCDIR}/../../ -L/usr/lib/danser -L/usr/lib -lbass -lbass_fx -lbassmix
#include "bass.h"
#include "bass_fx.h"
#include "bassmix.h"
*/
import "C"

import (
	"fmt"
	"log"
	"runtime"
	"sync"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

var masterMixer C.HSTREAM

var sampleRate = 44100

const (
	// BASS 2.4.18 removed the obsolete name from bass.h, but retains the
	// numeric configuration slot for compatibility. Keep the existing value
	// because Windows timing behavior is an intentional Stable compatibility
	// boundary in this application.
	bassConfigVistaTruePosition = 30
	bassConfigMP3OldGaps        = 68
)

// OutputInfo describes the format of the master mixer. Realtime BASS output
// can use a device rate different from the requested rate, so callers must
// use this value when converting timeline positions to mixer bytes.
type OutputInfo struct {
	SampleRate     int
	Channels       int
	BytesPerSample int
	LatencyMs      int
}

var (
	outputInfoMu sync.RWMutex
	outputInfo   OutputInfo
)

func Init(offscreen bool) {
	log.Println("Initializing BASS...")
	startExecutor()

	playbackBufferLength := 100
	deviceBufferLength := 10
	updatePeriod := 5
	devUpdatePeriod := 10

	if runtime.GOOS != "windows" {
		playbackBufferLength = int(settings.Audio.NonWindows.BassPlaybackBufferLength)
		deviceBufferLength = int(settings.Audio.NonWindows.BassDeviceBufferLength)
		updatePeriod = int(settings.Audio.NonWindows.BassUpdatePeriod)
		devUpdatePeriod = int(settings.Audio.NonWindows.BassDeviceUpdatePeriod)
	}

	deviceId := -1 //default audio device

	// Float output gives the mix sum headroom: stacked hitsounds over loud
	// music stay clean until the single graceful clamp inside the Windows
	// audio engine, instead of hard-clipping against a 16-bit mixing chain.
	// Sources remain 16-bit; BASSmix converts them losslessly at mix-in.
	mixerFlags := C.BASS_MIXER_NONSTOP | C.BASS_SAMPLE_FLOAT
	requestedRate := sampleRate

	if offscreen {
		requestedRate = 48000
		deviceId = 0 //If we're rendering, we don't want BASS to be tied to specific device, especially in headless system
		mixerFlags |= C.BASS_SAMPLE_FLOAT | C.BASS_STREAM_DECODE
	}

	var initError Error
	var initOK bool

	runOnAudioThread(func() {
		// Output data regardless if audio is playing.
		C.BASS_SetConfig(C.BASS_CONFIG_DEV_NONSTOP, C.DWORD(1))

		// Worse time resolution but lower latency with DirectSound.
		C.BASS_SetConfig(C.DWORD(bassConfigVistaTruePosition), C.DWORD(0))

		// Smaller stream buffer length, reduces latency.
		C.BASS_SetConfig(C.BASS_CONFIG_BUFFER, C.DWORD(playbackBufferLength))

		// Update BASS stream buffer more frequently.
		C.BASS_SetConfig(C.BASS_CONFIG_UPDATEPERIOD, C.DWORD(updatePeriod))

		// Smaller device buffer length, reduces latency.
		C.BASS_SetConfig(C.BASS_CONFIG_DEV_BUFFER, C.DWORD(deviceBufferLength))

		// Update BASS device buffer more frequently.
		C.BASS_SetConfig(C.BASS_CONFIG_DEV_PERIOD, C.DWORD(devUpdatePeriod))

		// BASS_CONFIG_MP3_OLDGAPS keeps legacy MP3 gap behavior stable.
		C.BASS_SetConfig(C.DWORD(bassConfigMP3OldGaps), C.DWORD(1))

		if C.BASS_Init(C.int(deviceId), C.DWORD(requestedRate), C.DWORD(0), nil, nil) == 0 {
			initError = Error(C.BASS_ErrorGetCode())
			return
		}

		var info C.BASS_INFO
		if C.BASS_GetInfo(&info) == 0 || info.freq == 0 {
			initError = Error(C.BASS_ErrorGetCode())
			C.BASS_Free()
			return
		}

		sampleRate = int(info.freq)
		outputInfoMu.Lock()
		outputInfo = OutputInfo{
			SampleRate:     sampleRate,
			Channels:       2,
			BytesPerSample: 4,
			LatencyMs:      int(info.latency),
		}
		outputInfoMu.Unlock()

		masterMixer = C.BASS_Mixer_StreamCreate(C.DWORD(sampleRate), C.DWORD(outputInfo.Channels), C.DWORD(mixerFlags))
		if masterMixer == 0 {
			initError = Error(C.BASS_ErrorGetCode())
			C.BASS_Free()
			return
		}

		C.BASS_ChannelSetAttribute(masterMixer, C.BASS_ATTRIB_BUFFER, 0)
		C.BASS_ChannelSetDevice(masterMixer, C.BASS_GetDevice())

		if !offscreen && C.BASS_ChannelPlay(masterMixer, 0) == 0 {
			initError = Error(C.BASS_ErrorGetCode())
			C.BASS_ChannelFree(masterMixer)
			masterMixer = 0
			C.BASS_Free()
			return
		}

		initOK = true
	})

	if !initOK {
		stopExecutor()
		panic(fmt.Sprintf("Failed to run BASS, error id: %d, message: %s", initError, initError.Message()))
	}

	versions := runOnAudioThreadResult(func() struct{ bass, fx, mix string } {
		return struct{ bass, fx, mix string }{
			bass: parseVersion(int(C.BASS_GetVersion())),
			fx:   parseVersion(int(C.BASS_FX_GetVersion())),
			mix:  parseVersion(int(C.BASS_Mixer_GetVersion())),
		}
	})

	log.Println("BASS Initialized!")
	log.Println("BASS Version:       ", versions.bass)
	log.Println("BASS FX Version:    ", versions.fx)
	log.Println("BASS Mix Version:   ", versions.mix)

	// We're not interested in BASSEnc in onscreen mode, show audio device instead.
	if !offscreen {
		log.Println("BASS Audio Device:  ", runOnAudioThreadResult(getDeviceName))
		log.Println("BASS Audio Latency: ", fmt.Sprintf("%dms", GetOutputInfo().LatencyMs))
	}
}

func parseVersion(version int) string {
	main := version >> 24 & 0xFF
	revision0 := version >> 16 & 0xFF
	revision1 := version >> 8 & 0xFF
	revision2 := version & 0xFF

	return fmt.Sprintf("%d.%d.%d.%d", main, revision0, revision1, revision2)
}

func getDeviceName() string {
	var info C.BASS_DEVICEINFO

	C.BASS_GetDeviceInfo(C.BASS_GetDevice(), &info)

	return C.GoString(info.name)
}

// GetOutputInfo returns the immutable output format selected during Init.
func GetOutputInfo() OutputInfo {
	outputInfoMu.RLock()
	defer outputInfoMu.RUnlock()
	return outputInfo
}

// Shutdown stops BASS after all application-owned tracks and samples have
// been released. It is safe to call more than once.
func Shutdown() {
	runOnAudioThread(func() {
		// Normally Player.Dispose performs this first. Keep the native boundary
		// self-contained as well so a direct shutdown cannot leave sample loops
		// attached while the master mixer is being freed.
		stopAllSamplesInternal()

		if masterMixer != 0 {
			C.BASS_ChannelStop(masterMixer)
			C.BASS_ChannelFree(masterMixer)
			masterMixer = 0
		}

		C.BASS_Free()
		outputInfoMu.Lock()
		outputInfo = OutputInfo{}
		outputInfoMu.Unlock()
	})

	stopExecutor()
}
