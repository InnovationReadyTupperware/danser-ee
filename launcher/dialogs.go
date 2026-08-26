package launcher

import (
	"errors"
	"log"
	"strings"
	"sync/atomic"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/wieku/danser-go/framework/goroutines"
	"github.com/wieku/danser-go/framework/platform/gcontext"
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

// message box button ids: callers branch on these through showMessage, so the
// mapping must stay stable regardless of display order below.
const (
	msgButtonOK   = 0
	msgButtonYes  = 1
	msgButtonNo   = 2
	msgButtonNone = -1 // returned by ShowMessageBox on failure; never matches Yes
)

// yesNoButtons defines a Yes/No pair where Enter confirms Yes and Escape
// activates No, matching conventional native dialog behavior.
func yesNoButtons() []sdl.MessageBoxButtonData {
	return []sdl.MessageBoxButtonData{
		{ButtonID: msgButtonYes, Text: "Yes", Flags: sdl.MESSAGEBOX_BUTTON_RETURNKEY_DEFAULT},
		{ButtonID: msgButtonNo, Text: "No", Flags: sdl.MESSAGEBOX_BUTTON_ESCAPEKEY_DEFAULT},
	}
}

// okButtons defines the single-button layout used for informational and plain
// error popups.
func okButtons() []sdl.MessageBoxButtonData {
	return []sdl.MessageBoxButtonData{
		{ButtonID: msgButtonOK, Text: "OK", Flags: sdl.MESSAGEBOX_BUTTON_RETURNKEY_DEFAULT | sdl.MESSAGEBOX_BUTTON_ESCAPEKEY_DEFAULT},
	}
}

// showMessageBox shows a blocking native message box parented to the launcher
// window and returns the pressed button id. Unlike the file pickers this can
// stay synchronous: native message boxes pump their own modal message loop,
// so the host window stays responsive while one is up. If the SDL bindings
// are unusable (they nil-deref until LoadLibrary succeeds, which is exactly
// when callers need to report a fatal startup error), execution falls back
// to raw platform APIs so the user always sees what went wrong.
func showMessageBox(flags sdl.MessageBoxFlags, message string, buttons []sdl.MessageBoxButtonData) (pressed int32) {
	yesNo := len(buttons) == 2

	defer func() {
		if r := recover(); r != nil {
			log.Println("SDL message box unavailable, using native fallback:", r)
			pressed = nativeMessageBox(flags, message, yesNo)
		}
	}()

	id, err := sdl.ShowMessageBox(&sdl.MessageBoxData{
		Flags:   flags,
		Window:  gcontext.SDLWindow(),
		Title:   "danser",
		Message: message,
		Buttons: buttons,
	})
	if err != nil {
		log.Println("SDL message box failed, using native fallback:", err)
		return nativeMessageBox(flags, message, yesNo)
	}

	return id
}
