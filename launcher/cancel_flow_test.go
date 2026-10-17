package launcher

import (
	"context"
	"testing"
	"time"

	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"
	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
)

// The cancellation flow is owned by the launcher event thread. A cancellable
// managedProcess exercises it without starting a child or creating SDL windows.
func newCancelFlowFixture(t *testing.T, outputMode PMode) (*launcher, *managedProcess, context.Context) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	run := &managedProcess{
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
		outputMode: outputMode,
	}
	l := &launcher{
		danserRunning: true,
		popupStack:    make([]iPopup, 0),
	}
	l.processSupervisor = &processSupervisor{owner: l, active: run}
	return l, run, ctx
}

func requireCancelFlowDialog(t *testing.T, l *launcher) *popupDialog {
	t.Helper()
	if l.processCancelDialog == nil {
		t.Fatal("expected an in-app cancellation confirmation")
	}
	dialog := l.processCancelDialog
	if len(l.popupStack) != 1 || l.popupStack[0] != dialog {
		t.Fatalf("popup stack = %#v, want exactly the active cancellation dialog", l.popupStack)
	}
	if dialog.primaryAction == nil || dialog.dismissAction == nil {
		t.Fatalf("cancel dialog missing confirmation or dismissal action: %#v", dialog)
	}
	if !dialog.opened {
		t.Fatal("cancellation modal was not opened")
	}
	return dialog
}

func TestRunningSessionStopAndExitRequireConfirmation(t *testing.T) {
	for _, test := range []struct {
		name       string
		mode       Mode
		outputMode PMode
		exit       bool
		title      string
		confirm    string
		dismiss    string
	}{
		{"watch stop", CursorDance, Watch, false, "Stop playback?", "Stop playback", "Keep watching"},
		{"watch exit", CursorDance, Watch, true, "Stop playback and exit?", "Stop and exit", "Keep watching"},
		{"play stop", Play, Watch, false, "Stop game?", "Stop game", "Keep playing"},
		{"play exit", Play, Watch, true, "Stop game and exit?", "Stop and exit", "Keep playing"},
		{"record stop", CursorDance, Record, false, "Stop recording?", "Stop recording", "Keep recording"},
		{"record exit", CursorDance, Record, true, "Stop recording and exit?", "Stop and exit", "Keep recording"},
		{"screenshot stop", CursorDance, Screenshot, false, "Stop screenshot?", "Stop capture", "Keep capturing"},
		{"screenshot exit", CursorDance, Screenshot, true, "Stop screenshot and exit?", "Stop and exit", "Keep capturing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			l, run, ctx := newCancelFlowFixture(t, test.outputMode)
			run.mode = test.mode
			l.requestProcessCancellation(test.exit)
			dialog := requireCancelFlowDialog(t, l)

			if dialog.title != test.title || dialog.primaryAction.label != test.confirm ||
				dialog.dismissAction.label != test.dismiss {
				t.Fatalf("confirmation copy = %q / %q / %q", dialog.title, dialog.primaryAction.label, dialog.dismissAction.label)
			}
			if dialog.tone != popupDialogToneDefault ||
				dialog.actionOrder != popupDialogPrimaryFirst ||
				!dialog.primaryAction.destructive || dialog.primaryAction.appearance != popupDialogActionOutlined ||
				dialog.dismissAction.destructive || dialog.dismissAction.appearance != popupDialogActionFilled {
				t.Fatal("confirmation must use icon-free, primary-first, outlined Stop and filled Keep action")
			}
			if ctx.Err() != nil || l.closeAfterDanser {
				t.Fatal("stop/exit took effect before confirmation")
			}
			dialog.activateActionAt(*dialog.primaryAction, time.Now())
			if ctx.Err() != context.Canceled || l.closeAfterDanser != test.exit {
				t.Fatalf("confirmed stop/exit = canceled %v, launcher exit %t; want exit %t", ctx.Err(), l.closeAfterDanser, test.exit)
			}
		})
	}
}

func TestRecordCancelDismissalPreservesChild(t *testing.T) {
	for _, test := range []struct {
		name    string
		dismiss func(*popupDialog)
	}{
		{
			name: "Cancel action",
			dismiss: func(d *popupDialog) {
				d.activateActionAt(*d.dismissAction, time.Now())
			},
		},
		{
			name: "Escape or outside-click request",
			dismiss: func(d *popupDialog) {
				d.requestDismissAt(time.Now())
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			l, _, ctx := newCancelFlowFixture(t, Record)
			l.requestProcessCancellation(false)
			dialog := requireCancelFlowDialog(t, l)
			dialog.phase = popupDialogPhaseVisible

			test.dismiss(dialog)
			if ctx.Err() != nil {
				t.Fatalf("dismissing modal canceled Record child: %v", ctx.Err())
			}
			if l.closeAfterDanser {
				t.Fatal("dismissing modal scheduled launcher exit")
			}

			// A dialog that is disappearing is revived rather than stacking
			// an identically named ImGui modal on top of itself.
			if dialog.phase != popupDialogPhaseDisappearing {
				t.Fatalf("dismissed modal phase = %v, want disappearing", dialog.phase)
			}
			l.requestProcessCancellation(false)
			if l.processCancelDialog != dialog || len(l.popupStack) != 1 {
				t.Fatal("repeated request replaced or stacked the disappearing modal")
			}
			if !dialog.opened || dialog.phase == popupDialogPhaseDisappearing {
				t.Fatal("re-request did not revive the dismissed modal")
			}
			if l.processCancelDialog.primaryAction == nil {
				t.Fatal("reopened cancel confirmation has no primary action")
			}
		})
	}
}

func TestRecordStopConfirmationCanUpgradeToExit(t *testing.T) {
	l, _, ctx := newCancelFlowFixture(t, Record)
	l.requestProcessCancellation(false)
	dialog := requireCancelFlowDialog(t, l)

	l.requestProcessCancellation(true)
	if l.processCancelDialog != dialog || len(l.popupStack) != 1 {
		t.Fatal("exit request created a duplicate cancellation modal")
	}
	if dialog.title != "Stop recording and exit?" || dialog.primaryAction.label != "Stop and exit" {
		t.Fatalf("modal not upgraded for exit: title=%q action=%q", dialog.title, dialog.primaryAction.label)
	}
	if ctx.Err() != nil || l.closeAfterDanser {
		t.Fatal("exit upgrade canceled or closed before confirmation")
	}

	dialog.activateActionAt(*dialog.primaryAction, time.Now())
	if ctx.Err() != context.Canceled || !l.closeAfterDanser {
		t.Fatalf("upgraded confirm did not cancel and exit: child=%v exit=%t", ctx.Err(), l.closeAfterDanser)
	}
}

func TestDismissedExitConfirmationDoesNotUpgradeLaterStop(t *testing.T) {
	l, _, ctx := newCancelFlowFixture(t, Record)
	l.requestProcessCancellation(true)
	dialog := requireCancelFlowDialog(t, l)
	dialog.phase = popupDialogPhaseVisible
	dialog.activateActionAt(*dialog.dismissAction, time.Now())

	l.requestProcessCancellation(false)
	if l.processCancelDialog != dialog || len(l.popupStack) != 1 {
		t.Fatal("stop request stacked a new dialog over a dismissed exit confirmation")
	}
	if l.processCancelDialogExits || dialog.title != "Stop recording?" ||
		dialog.primaryAction.label != "Stop recording" {
		t.Fatal("new stop request retained the dismissed exit action")
	}
	dialog.activateActionAt(*dialog.primaryAction, time.Now())
	if ctx.Err() != context.Canceled || l.closeAfterDanser {
		t.Fatalf("stop after dismiss = child %v, exit %t", ctx.Err(), l.closeAfterDanser)
	}
}

func TestClosingPendingLaunchUsesCancellationAnimation(t *testing.T) {
	previousShouldClose := gcontext.ShouldClose()
	t.Cleanup(func() { gcontext.SetShouldClose(previousShouldClose) })
	gcontext.SetShouldClose(false)

	triangles := animation.NewGlider(1)
	triangles.Update(0)
	triangles.AddEvent(0, 500, 25)
	triangles.Update(250)
	l := &launcher{
		pendingStart:  &pendingDanserStart{outputMode: Watch},
		triangleSpeed: triangles,
	}
	l.handleCloseRequest()
	if l.pendingStart != nil || !gcontext.ShouldClose() {
		t.Fatal("window close left the pending launch active or failed to close the launcher")
	}
	triangles.Update(750)
	if got := triangles.GetValue(); got != 1 {
		t.Fatalf("triangle speed after closing pending launch = %v, want 1", got)
	}
}

func TestPendingWatchCancelDoesNotRequestModal(t *testing.T) {
	l := &launcher{
		triangleSpeed: animation.NewGlider(1),
		pendingStart: &pendingDanserStart{
			clickedAt:  time.Now(),
			outputMode: Watch,
		},
	}
	l.requestProcessCancellation(false)

	if l.pendingStart != nil {
		t.Fatal("cancel request left a pending Watch timer active")
	}
	if l.processCancelDialog != nil || len(l.popupStack) != 0 {
		t.Fatal("pre-spawn cancellation unexpectedly requested a modal")
	}
	if l.danserRunning {
		t.Fatal("pre-spawn cancellation started a child")
	}
}

func TestFinishedRunClearsOnlyItsCancellationModal(t *testing.T) {
	l, run, _ := newCancelFlowFixture(t, Record)
	l.requestProcessCancellation(false)
	dialog := requireCancelFlowDialog(t, l)

	l.dismissProcessCancelDialog(&managedProcess{})
	if l.processCancelDialog != dialog || !dialog.opened {
		t.Fatal("unrelated process dismissed active cancellation modal")
	}
	l.dismissProcessCancelDialog(run)
	if l.processCancelDialog != nil || l.processCancelDialogRun != nil || l.processCancelDialogExits {
		t.Fatal("completed process left stale cancellation state")
	}
	if dialog.opened || !dialog.shouldClose() {
		t.Fatal("completed process did not close its cancellation dialog")
	}
}

func TestOldCancelDialogCannotStopReplacementChild(t *testing.T) {
	l, _, originalContext := newCancelFlowFixture(t, Record)
	l.requestProcessCancellation(false)
	dialog := requireCancelFlowDialog(t, l)

	replacementContext, replacementCancel := context.WithCancel(context.Background())
	t.Cleanup(replacementCancel)
	l.processSupervisor.active = &managedProcess{
		ctx:        replacementContext,
		cancel:     replacementCancel,
		done:       make(chan struct{}),
		outputMode: Record,
	}

	dialog.activateActionAt(*dialog.primaryAction, time.Now())
	if replacementContext.Err() != nil {
		t.Fatalf("stale modal canceled replacement child: %v", replacementContext.Err())
	}
	if originalContext.Err() != nil {
		t.Fatalf("stale modal canceled unrelated previous child: %v", originalContext.Err())
	}
	if l.closeAfterDanser {
		t.Fatal("stale cancel confirmation requested launcher exit")
	}
}
