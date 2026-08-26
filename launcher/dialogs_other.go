//go:build !windows

package launcher

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/Zyko0/go-sdl3/sdl"
)

// nativeMessageBox is the Linux/best-effort counterpart to the Windows
// user32 fallback in dialogs_windows.go. SDL's message box is unavailable
// when LoadLibrary failed (the exact moment we need to report a startup
// error) and also when SDL itself reports an error at runtime. On Windows
// MessageBoxW is always present; on Linux there is no single guaranteed
// toolkit, so this implementation probes common desktop dialog helpers in
// order of availability - zenity (GNOME/Ubuntu) and kdialog (KDE) - and
// falls back to logging plus stderr when no helper is present or no display
// is available. This keeps the tree building on every GOOS, preserves
// interactive Yes/No semantics where possible, and stays dependency-free
// (no new Go packages, no mandatory external requirement).
func nativeMessageBox(flags sdl.MessageBoxFlags, message string, yesNo bool) int32 {
	if res, ok := tryZenity(flags, message, yesNo); ok {
		return res
	}
	if res, ok := tryKDialog(flags, message, yesNo); ok {
		return res
	}
	// No helper available or no display - make the error visible in both the
	// log file and the launching terminal (if any). Returning msgButtonNone
	// preserves the previous stub's "No/Cancel" semantics for question
	// dialogs (callers check == msgButtonYes).
	log.Println("Message box unavailable on this platform:", message)
	fmt.Fprintf(os.Stderr, "danser: %s\n", message)
	return msgButtonNone
}

func tryZenity(flags sdl.MessageBoxFlags, message string, yesNo bool) (int32, bool) {
	if _, err := exec.LookPath("zenity"); err != nil {
		return msgButtonNone, false
	}
	var args []string
	if yesNo {
		args = []string{"--question", "--text", message, "--title", "danser", "--width", "400", "--ok-label", "Yes", "--cancel-label", "No"}
		switch flags {
		case sdl.MESSAGEBOX_ERROR:
			args = append(args, "--icon-name", "dialog-error")
		case sdl.MESSAGEBOX_WARNING:
			args = append(args, "--icon-name", "dialog-warning")
		}
	} else {
		switch flags {
		case sdl.MESSAGEBOX_ERROR:
			args = []string{"--error", "--text", message, "--title", "danser", "--width", "400"}
		case sdl.MESSAGEBOX_WARNING:
			args = []string{"--warning", "--text", message, "--title", "danser", "--width", "400"}
		default:
			args = []string{"--info", "--text", message, "--title", "danser", "--width", "400"}
		}
	}
	cmd := exec.Command("zenity", args...)
	err := cmd.Run()
	if err == nil {
		if yesNo {
			return msgButtonYes, true
		}
		return msgButtonOK, true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		if yesNo {
			if code == 0 {
				return msgButtonYes, true
			}
			if code == 1 {
				return msgButtonNo, true
			}
			return msgButtonNone, true
		}
		if code == 0 {
			return msgButtonOK, true
		}
		return msgButtonNone, true
	}
	return msgButtonNone, false
}

func tryKDialog(flags sdl.MessageBoxFlags, message string, yesNo bool) (int32, bool) {
	if _, err := exec.LookPath("kdialog"); err != nil {
		return msgButtonNone, false
	}
	var args []string
	if yesNo {
		args = []string{"--yesno", message, "--title", "danser"}
	} else {
		switch flags {
		case sdl.MESSAGEBOX_ERROR:
			args = []string{"--error", message, "--title", "danser"}
		case sdl.MESSAGEBOX_WARNING:
			args = []string{"--sorry", message, "--title", "danser"}
		default:
			args = []string{"--msgbox", message, "--title", "danser"}
		}
	}
	cmd := exec.Command("kdialog", args...)
	err := cmd.Run()
	if err == nil {
		if yesNo {
			return msgButtonYes, true
		}
		return msgButtonOK, true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		if yesNo {
			if code == 0 {
				return msgButtonYes, true
			}
			if code == 1 {
				return msgButtonNo, true
			}
			return msgButtonNone, true
		}
		if code == 0 {
			return msgButtonOK, true
		}
		return msgButtonNone, true
	}
	return msgButtonNone, false
}
