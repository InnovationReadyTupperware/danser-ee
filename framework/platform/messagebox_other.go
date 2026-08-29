//go:build !windows

package platform

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// nativeMessageBox is the best-effort fallback for platforms without a
// guaranteed message-box API. The helpers are invoked directly rather than
// through a shell so an error string can never become command syntax.
func nativeMessageBox(kind MessageBoxKind, message string, yesNo bool) MessageBoxResult {
	if result, ok := tryZenity(kind, message, yesNo); ok {
		return result
	}
	if result, ok := tryKDialog(kind, message, yesNo); ok {
		return result
	}

	// A headless Linux session may not have either helper or a usable display.
	// Keep the failure visible in the log and terminal instead of hiding it
	// behind an unavailable graphical surface.
	log.Println("Message box unavailable on this platform:", message)
	if _, err := fmt.Fprintln(os.Stderr, "danser:", message); err != nil {
		log.Println("could not write message box fallback to stderr:", err)
	}
	return MessageBoxNone
}

func tryZenity(kind MessageBoxKind, message string, yesNo bool) (MessageBoxResult, bool) {
	if _, err := exec.LookPath("zenity"); err != nil {
		return MessageBoxNone, false
	}

	var args []string
	if yesNo {
		args = []string{"--question", "--text", message, "--title", "danser", "--width", "400", "--ok-label", "Yes", "--cancel-label", "No"}
	} else {
		switch kind {
		case MessageBoxError:
			args = []string{"--error", "--text", message, "--title", "danser", "--width", "400"}
		case MessageBoxWarning:
			args = []string{"--warning", "--text", message, "--title", "danser", "--width", "400"}
		default:
			args = []string{"--info", "--text", message, "--title", "danser", "--width", "400"}
		}
	}

	return runDialogCommand("zenity", args, yesNo)
}

func tryKDialog(kind MessageBoxKind, message string, yesNo bool) (MessageBoxResult, bool) {
	if _, err := exec.LookPath("kdialog"); err != nil {
		return MessageBoxNone, false
	}

	var args []string
	if yesNo {
		args = []string{"--yesno", message, "--title", "danser"}
	} else {
		switch kind {
		case MessageBoxError:
			args = []string{"--error", message, "--title", "danser"}
		case MessageBoxWarning:
			args = []string{"--sorry", message, "--title", "danser"}
		default:
			args = []string{"--msgbox", message, "--title", "danser"}
		}
	}

	return runDialogCommand("kdialog", args, yesNo)
}

func runDialogCommand(name string, args []string, yesNo bool) (MessageBoxResult, bool) {
	err := exec.Command(name, args...).Run()
	if err == nil {
		if yesNo {
			return MessageBoxYes, true
		}
		return MessageBoxOK, true
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		if yesNo {
			switch code {
			case 0:
				return MessageBoxYes, true
			case 1:
				return MessageBoxNo, true
			default:
				return MessageBoxNone, true
			}
		}
		if code == 0 {
			return MessageBoxOK, true
		}
		return MessageBoxNone, true
	}

	return MessageBoxNone, false
}
