package launcher

import (
	"errors"
	"strings"
	"sync/atomic"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/platform"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
)

// ErrDialogCancelled is passed to picker callbacks when the user closes or
// cancels a native dialog without choosing anything.
var ErrDialogCancelled = errors.New("dialog: cancelled")

// File and folder dialogs are asynchronous on SDL3: the ShowOpen* calls
// return immediately and SDL invokes our trampoline from its own thread once
// the user closes the dialog. The launcher's main thread must keep running
// its event loop while a dialog is open - waiting on the result there freezes
// the window ("not responding"), so results are routed through a continuation
// that is queued back onto the main thread instead.

// pickerBusy serializes dialogs, since interleaved launches would have no
// clean way to match results with their continuation. Launching while one is
// already open is ignored.
var pickerBusy atomic.Bool

// pendingResult holds the continuation of the currently open dialog.
var pendingResult atomic.Pointer[func(paths []string)]

// dialogCallback is registered once at package init; purego trampolines are a
// limited, non-freeable resource, so every dialog shares this entry point.
var dialogCallback = sdl.NewDialogFileCallback(func(files []string, _ int32) {
	if cont := pendingResult.Swap(nil); cont != nil {
		result := *cont
		goroutines.CallNonBlockMain(func() { result(files) })
	}
})

// showFilePicker opens a native file (or folder, when directory is set)
// picker parented to the launcher window and returns immediately. extensions
// are plain suffixes without dots, joined into a single filter entry ("jpg",
// "png" -> "*.jpg;*.png" equivalent on each platform backend). Exactly one
// onDone call follows, executed on the launcher's main thread: either the
// chosen paths, or ErrDialogCancelled for an empty selection.
func showFilePicker(title string, extensions []string, startDir string, multiple bool, directory bool, onDone func(paths []string, err error)) {
	if !pickerBusy.CompareAndSwap(false, true) {
		return
	}

	cont := func(files []string) {
		pickerBusy.Store(false)

		if len(files) == 0 {
			onDone(nil, ErrDialogCancelled)
			return
		}

		onDone(files, nil)
	}

	pendingResult.Store(&cont)

	var filters []sdl.DialogFileFilter
	if len(extensions) > 0 {
		filters = []sdl.DialogFileFilter{{
			Name:    title,
			Pattern: strings.Join(extensions, ";"),
		}}
	}

	if directory {
		sdl.ShowOpenFolderDialog(dialogCallback, gcontext.SDLWindow(), startDir, false)
	} else {
		sdl.ShowOpenFileDialog(dialogCallback, gcontext.SDLWindow(), filters, startDir, multiple)
	}
}

// Message-box result ids are kept local to the launcher so existing callers do
// not depend on the platform package's public result type.
const (
	msgButtonOK   = 0
	msgButtonYes  = 1
	msgButtonNo   = 2
	msgButtonNone = -1 // returned when no platform result is available
)

// showMessageBox shows a blocking native message box parented to the launcher
// window and returns the launcher-local button id. The platform layer owns the
// SDL/native fallback because startup failures can occur before SDL is loaded.
func showMessageBox(flags sdl.MessageBoxFlags, message string, yesNo bool) int32 {
	kind := platform.MessageBoxInformation
	switch flags {
	case sdl.MESSAGEBOX_WARNING:
		kind = platform.MessageBoxWarning
	case sdl.MESSAGEBOX_ERROR:
		kind = platform.MessageBoxError
	}

	switch platform.ShowMessageBox(gcontext.SDLWindow(), kind, message, yesNo) {
	case platform.MessageBoxOK:
		return msgButtonOK
	case platform.MessageBoxYes:
		return msgButtonYes
	case platform.MessageBoxNo:
		return msgButtonNo
	default:
		return msgButtonNone
	}
}
