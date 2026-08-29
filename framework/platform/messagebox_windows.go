//go:build windows

package platform

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// user32 is part of Windows itself, so this fallback remains available when
// SDL3.dll is missing or cannot initialize. Lazy loading also avoids turning a
// normal non-Windows build concern into a startup dependency.
var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

// MessageBoxW style bits used by the fallback. They stay local so platform
// callers depend only on MessageBoxKind and MessageBoxResult.
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

func nativeMessageBox(kind MessageBoxKind, message string, yesNo bool) MessageBoxResult {
	var icon uintptr
	switch kind {
	case MessageBoxError:
		icon = mbStyleIconError
	case MessageBoxWarning:
		icon = mbStyleIconWarning
	default:
		icon = mbStyleIconInfo
	}

	style := icon | mbStyleForeground | mbStyleTopmost
	if yesNo {
		// A question has a distinct native icon even when the prompt originated
		// from an error path, and the button layout communicates the decision.
		style = mbStyleIconQuestion | mbStyleYesNo | mbStyleForeground | mbStyleTopmost
	} else {
		style |= mbStyleOK
	}

	text, err := windows.UTF16PtrFromString(message)
	if err != nil {
		return MessageBoxNone
	}

	title, err := windows.UTF16PtrFromString("danser")
	if err != nil {
		return MessageBoxNone
	}

	rc, _, callErr := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		style,
	)
	if rc == 0 && callErr != nil {
		return MessageBoxNone
	}

	switch rc {
	case mbRetYes:
		return MessageBoxYes
	case mbRetNo:
		return MessageBoxNo
	case mbRetOK:
		return MessageBoxOK
	default:
		return MessageBoxNone
	}
}
