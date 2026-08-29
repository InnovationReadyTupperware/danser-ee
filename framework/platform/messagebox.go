package platform

import (
	"log"

	"github.com/Zyko0/go-sdl3/sdl"
)

// MessageBoxKind selects the icon and semantic category shown by a native
// message box.
type MessageBoxKind uint8

const (
	// MessageBoxInformation is used for neutral status and confirmation text.
	MessageBoxInformation MessageBoxKind = iota
	// MessageBoxWarning is used when an operation can continue but needs
	// attention.
	MessageBoxWarning
	// MessageBoxError is used for failures that prevent the requested operation
	// from completing.
	MessageBoxError
)

// MessageBoxResult identifies the result of a message box. MessageBoxNone is
// returned when neither SDL nor the platform fallback can produce a result.
type MessageBoxResult uint8

const (
	MessageBoxNone MessageBoxResult = iota
	MessageBoxOK
	MessageBoxYes
	MessageBoxNo
)

const (
	messageBoxOKID  int32 = 0
	messageBoxYesID int32 = 1
	messageBoxNoID  int32 = 2
)

// ShowMessageBox presents a modal native message box. SDL is preferred because
// it keeps the popup associated with the application's window and already
// provides the cross-platform implementation. Startup failures can happen
// before SDL is loaded, however, so the platform-specific fallback is guarded
// against both returned errors and binding panics.
func ShowMessageBox(parent *sdl.Window, kind MessageBoxKind, message string, yesNo bool) (result MessageBoxResult) {
	flags := sdl.MESSAGEBOX_INFORMATION
	switch kind {
	case MessageBoxWarning:
		flags = sdl.MESSAGEBOX_WARNING
	case MessageBoxError:
		flags = sdl.MESSAGEBOX_ERROR
	}

	buttons := []sdl.MessageBoxButtonData{
		{ButtonID: messageBoxOKID, Text: "OK", Flags: sdl.MESSAGEBOX_BUTTON_RETURNKEY_DEFAULT | sdl.MESSAGEBOX_BUTTON_ESCAPEKEY_DEFAULT},
	}
	if yesNo {
		buttons = []sdl.MessageBoxButtonData{
			{ButtonID: messageBoxYesID, Text: "Yes", Flags: sdl.MESSAGEBOX_BUTTON_RETURNKEY_DEFAULT},
			{ButtonID: messageBoxNoID, Text: "No", Flags: sdl.MESSAGEBOX_BUTTON_ESCAPEKEY_DEFAULT},
		}
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("SDL message box unavailable, using native fallback: %v", recovered)
			result = safeNativeMessageBox(kind, message, yesNo)
		}
	}()

	pressed, err := sdl.ShowMessageBox(&sdl.MessageBoxData{
		Flags:   flags,
		Window:  parent,
		Title:   "danser",
		Message: message,
		Buttons: buttons,
	})
	if err != nil {
		log.Printf("SDL message box failed, using native fallback: %v", err)
		return safeNativeMessageBox(kind, message, yesNo)
	}

	switch pressed {
	case messageBoxOKID:
		return MessageBoxOK
	case messageBoxYesID:
		return MessageBoxYes
	case messageBoxNoID:
		return MessageBoxNo
	default:
		return MessageBoxNone
	}
}

// ShowErrorDialog presents a fatal error without requiring a functioning SDL
// window. It is intended for startup and crash boundaries where returning to a
// normal in-app error surface is no longer possible.
func ShowErrorDialog(parent *sdl.Window, message string) {
	ShowMessageBox(parent, MessageBoxError, message, false)
}

func safeNativeMessageBox(kind MessageBoxKind, message string, yesNo bool) (result MessageBoxResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("native message box failed: %v", recovered)
			result = MessageBoxNone
		}
	}()

	return nativeMessageBox(kind, message, yesNo)
}
