package launcher

import "github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"

// requestProcessCancellation is called on the launcher thread. Pending starts
// cancel directly; any active child requires confirmation.
func (l *launcher) requestProcessCancellation(exitLauncher bool) {
	if l.pendingStart != nil {
		l.cancelPendingDanserStart()
		if l.triangleSpeed != nil {
			l.triangleSpeed.Reset()
			l.triangleSpeed.AddEvent(l.triangleSpeed.GetTime(), l.triangleSpeed.GetTime()+500, 1)
		}
		if exitLauncher {
			gcontext.SetShouldClose(true)
		}
		return
	}

	if l.processSupervisor == nil {
		if exitLauncher {
			gcontext.SetShouldClose(true)
		}
		return
	}

	run := l.processSupervisor.currentRun()
	if !l.processSupervisor.activeRun(run) {
		if exitLauncher {
			gcontext.SetShouldClose(true)
		}
		return
	}

	l.confirmProcessCancellation(run, exitLauncher)
}

func (l *launcher) cancelActiveProcess(run *managedProcess, exitLauncher bool) {
	if l.processSupervisor == nil || !l.processSupervisor.activeRun(run) {
		return
	}
	if exitLauncher {
		l.closeAfterDanser = true
	}
	run.cancel()
}

type processCancellationDialogCopy struct {
	title   string
	message string
	confirm string
	dismiss string
}

func processCancellationCopy(mode Mode, outputMode PMode, exitLauncher bool) processCancellationDialogCopy {
	if outputMode == Watch {
		if mode == Play {
			if exitLauncher {
				return processCancellationDialogCopy{
					title:   "Stop game and exit?",
					message: "Your current game will end and danser-ee will close.",
					confirm: "Stop and exit",
					dismiss: "Keep playing",
				}
			}
			return processCancellationDialogCopy{
				title:   "Stop game?",
				message: "Your current game will end.",
				confirm: "Stop game",
				dismiss: "Keep playing",
			}
		}
		if exitLauncher {
			return processCancellationDialogCopy{
				title:   "Stop playback and exit?",
				message: "The current playback will end and danser-ee will close.",
				confirm: "Stop and exit",
				dismiss: "Keep watching",
			}
		}
		return processCancellationDialogCopy{
			title:   "Stop playback?",
			message: "The current playback will end.",
			confirm: "Stop playback",
			dismiss: "Keep watching",
		}
	}
	if outputMode == Record {
		if exitLauncher {
			return processCancellationDialogCopy{
				title:   "Stop recording and exit?",
				message: "The recording is still in progress. Stopping now may leave the output incomplete.",
				confirm: "Stop and exit",
				dismiss: "Keep recording",
			}
		}
		return processCancellationDialogCopy{
			title:   "Stop recording?",
			message: "The recording is still in progress. Stopping now may leave the output incomplete.",
			confirm: "Stop recording",
			dismiss: "Keep recording",
		}
	}
	if exitLauncher {
		return processCancellationDialogCopy{
			title:   "Stop screenshot and exit?",
			message: "The screenshot is still being captured. Stopping now may prevent it from being saved.",
			confirm: "Stop and exit",
			dismiss: "Keep capturing",
		}
	}
	return processCancellationDialogCopy{
		title:   "Stop screenshot?",
		message: "The screenshot is still being captured. Stopping now may prevent it from being saved.",
		confirm: "Stop capture",
		dismiss: "Keep capturing",
	}
}

func (l *launcher) confirmProcessCancellation(run *managedProcess, exitLauncher bool) {
	if l.processCancelDialog != nil && l.processCancelDialogRun == run &&
		!l.processCancelDialog.shouldClose() {
		// A window-close request can upgrade an existing Stop confirmation
		// to Stop and exit, but a dismissed confirmation cannot carry an
		// earlier exit request into a new Stop-only request.
		if l.processCancelDialog.phase == popupDialogPhaseDisappearing {
			l.processCancelDialogExits = exitLauncher
		} else {
			l.processCancelDialogExits = l.processCancelDialogExits || exitLauncher
		}
		copy := processCancellationCopy(run.mode, run.outputMode, l.processCancelDialogExits)
		l.processCancelDialog.title = copy.title
		l.processCancelDialog.supportingText = copy.message
		l.processCancelDialog.primaryAction.label = copy.confirm
		l.processCancelDialog.dismissAction.label = copy.dismiss
		if l.processCancelDialog.phase == popupDialogPhaseDisappearing {
			l.processCancelDialog.open()
		}
		return
	}

	l.processCancelDialogExits = exitLauncher
	copy := processCancellationCopy(run.mode, run.outputMode, exitLauncher)
	dialog := newPopupDialog(
		"cancel-process",
		copy.title,
		withPopupDialogSupportingText(copy.message),
		withPopupDialogPrimaryAction(outlinedDestructivePopupDialogAction(copy.confirm, func() {
			l.cancelActiveProcess(run, l.processCancelDialogExits)
		})),
		withPopupDialogDismissAction(newPopupDialogAction(copy.dismiss, nil)),
		withPopupDialogPrimaryActionFirst(),
		withPopupDialogDismissOnClickOutside(false),
	)
	l.processCancelDialog = dialog
	l.processCancelDialogRun = run
	l.openPopup(dialog)
}

// The child may finish while its cancellation dialog is open. Never leave a
// stale confirmation that could be mistaken for the next launched process.
func (l *launcher) dismissProcessCancelDialog(run *managedProcess) {
	if l.processCancelDialog == nil || l.processCancelDialogRun != run {
		return
	}
	l.processCancelDialog.opened = false
	l.processCancelDialog.phase = popupDialogPhaseClosed
	l.processCancelDialog = nil
	l.processCancelDialogRun = nil
	l.processCancelDialogExits = false
}
