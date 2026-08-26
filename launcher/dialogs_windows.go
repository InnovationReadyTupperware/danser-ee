package launcher

import (
	"unsafe"

	"github.com/Zyko0/go-sdl3/sdl"
	"golang.org/x/sys/windows"
)

// Raw user32 fallback used when the SDL bindings are not loadable: user32
// ships with Windows itself, so this works in any situation, including
// reporting the missing-library failure that disabled SDL in the first
// place. MessageBoxW blocks and pumps its own modal loop.

var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

// MessageBoxW style bits (only what the fallback needs; kept local so the
// constants do not leak into launcher-wide naming).
const (
	mbStyleOK           = 0x00000000
	mbStyleYesNo        = 0x00000004
	mbStyleIconError    = 0x00000010
	mbStyleIconQuestion = 0x00000020
	mbStyleIconWarning  = 0x00000030
	mbStyleIconInfo     = 0x00000040
	mbStyleForeground   = 0x00010000
	mbStyleTopmost      = 0x00040000

	mbRetOK  = 1
	mbRetYes = 6
	mbRetNo  = 7
)

// nativeMessageBox shows a plain Win32 message box owned by the desktop and
// returns the launcher-wide button id matching the requested layout.
func nativeMessageBox(flags sdl.MessageBoxFlags, message string, yesNo bool) int32 {
	var icon uintptr
	switch flags {
	case sdl.MESSAGEBOX_ERROR:
		icon = mbStyleIconError
	case sdl.MESSAGEBOX_WARNING:
		icon = mbStyleIconWarning
	default:
		icon = mbStyleIconInfo
	}

	style := icon | mbStyleForeground | mbStyleTopmost
	if yesNo {
		style |= mbStyleYesNo | mbStyleIconQuestion
	} else {
		style |= mbStyleOK
	}

	text, err := windows.UTF16PtrFromString(message)
	if err != nil {
		return msgButtonNone
	}

	title, err := windows.UTF16PtrFromString("danser")
	if err != nil {
		return msgButtonNone
	}

	rc, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		style,
	)

	switch rc {
	case mbRetYes:
		return msgButtonYes
	case mbRetNo:
		return msgButtonNo
	case mbRetOK:
		return msgButtonOK
	default:
		return msgButtonNone
	}
}
