//go:build !windows

package launcher

import (
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

// nativeMessageBox is the non-Windows stub of the user32 fallback: without a
// guaranteed native dialog API there is nothing safe to fall back to, so the
// failure stays logged. Per project platform scope, Windows is the supported
// target and this branch exists only to keep cross-compilation compiling.
func nativeMessageBox(flags sdl.MessageBoxFlags, message string, yesNo bool) int32 {
	log.Println("Message box unavailable on this platform:", message)
	return msgButtonNone
}
