package launcher

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/innovationreadytupperware/danser-ee/framework/math/animation"

	"github.com/innovationreadytupperware/danser-ee/framework/platform/gcontext"
)

func TestTriangleLaunchAnimationProfile(t *testing.T) {
	glider := animation.NewGlider(1)
	glider.Update(1000)
	l := &launcher{triangleSpeed: glider}
	l.animateTriangleLaunch()

	for _, step := range []struct {
		ms   float64
		want float64
	}{
		{0, 1},
		{30, 7},
		{60, 25},
		{450, 1 + 24*math.Pow(150.0/540.0, 3)},
		{600, 1},
		{800, 1},
	} {
		glider.Update(1000 + step.ms)
		if got := glider.GetValue(); math.Abs(got-step.want) > 0.00001 {
			t.Errorf("speed at %.0fms = %.4f, want %.4f", step.ms, got, step.want)
		}
	}
}

func TestTriangleLaunchRestartDiscardsQueuedDeceleration(t *testing.T) {
	glider := animation.NewGlider(1)
	glider.Update(1000)
	l := &launcher{triangleSpeed: glider}
	l.animateTriangleLaunch()
	glider.Update(1030)
	interruptedSpeed := glider.GetValue()
	l.animateTriangleLaunch()

	glider.Update(1030)
	if got := glider.GetValue(); got != interruptedSpeed {
		t.Fatalf("restart jumped from %.2f to %.2f", interruptedSpeed, got)
	}
	glider.Update(1090)
	if got := glider.GetValue(); got != triangleLaunchPeakSpeed {
		t.Fatalf("restarted ramp peak = %.2f, want %.2f", got, triangleLaunchPeakSpeed)
	}
	glider.Update(1630)
	if got := glider.GetValue(); got != 1 {
		t.Fatalf("restarted animation ended at %.2f, want 1", got)
	}
}

func TestParseProcessProgress(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantStatus string
		wantValue  float32
		wantSpeed  float64
	}{
		{
			name:       "standard progress line",
			line:       "Progress: 37.5%, Speed: 2.25x, ETA: 00:12",
			wantOK:     true,
			wantStatus: "37.5%",
			wantValue:  0.375,
			wantSpeed:  2.25,
		},
		{
			name:       "prefix before progress",
			line:       "Encoder output - Progress: 100%, Speed: 1x, ETA: done, extra",
			wantOK:     true,
			wantStatus: "100%",
			wantValue:  1,
			wantSpeed:  1,
		},
		{
			name:   "missing percentage sign",
			line:   "Progress: 50, Speed: 1x, ETA: 00:10",
			wantOK: false,
		},
		{
			name:   "missing fields",
			line:   "Progress: 50%, Speed: 1x",
			wantOK: false,
		},
		{
			name:   "invalid percentage",
			line:   "Progress: NaN%, Speed: 1x, ETA: 00:10",
			wantOK: false,
		},
		{
			name:   "out of range percentage",
			line:   "Progress: 101%, Speed: 1x, ETA: 00:10",
			wantOK: false,
		},
		{
			name:   "missing separator",
			line:   "Progress 50%, Speed: 1x, ETA: 00:10",
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseProcessProgress(test.line)
			if ok != test.wantOK {
				t.Fatalf("parseProcessProgress() ok = %v, want %v", ok, test.wantOK)
			}
			if !test.wantOK {
				return
			}

			if got.status != test.wantStatus || got.value != test.wantValue || got.speedValue != test.wantSpeed {
				t.Fatalf("parseProcessProgress() = %#v, want status=%q value=%v speed=%v", got, test.wantStatus, test.wantValue, test.wantSpeed)
			}
		})
	}
}

func TestParseLabeledFloatRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []string{"Speed: NaN", "Speed: +Inf", "Speed: -Inf", "not a number"} {
		if got := parseLabeledFloat(value); got != 0 {
			t.Fatalf("parseLabeledFloat(%q) = %v, want 0", value, got)
		}
	}
}

func TestProcessOutputSplashHandoff(t *testing.T) {
	tests := []struct {
		name         string
		outputMode   PMode
		lines        string
		wantHandoffs int
	}{
		{
			name:         "watch receives one handoff despite repeated splash messages",
			outputMode:   Watch,
			lines:        "2026/10/16 15:00:00 Window created!\n2026/10/16 15:00:01 Gameplay: loading screen presented\n2026/10/16 15:00:01 Gameplay: loading screen presented\n",
			wantHandoffs: 1,
		},
		{
			name:       "window creation alone is not ready",
			outputMode: Watch,
			lines:      "2026/10/16 15:00:00 Window created!\n",
		},
		{
			name:       "record mode never hands off",
			outputMode: Record,
			lines:      "2026/10/16 15:00:00 Gameplay: loading screen presented\n",
		},
		{
			name:       "screenshot mode never hands off",
			outputMode: Screenshot,
			lines:      "2026/10/16 15:00:00 Gameplay: loading screen presented\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			owner := &launcher{
				runtimeDone:   ctx.Done(),
				runtimeEvents: make(chan launcherEvent, 4),
			}
			run := &managedProcess{ctx: ctx, done: make(chan struct{})}
			output := make(chan processOutput, 1)
			scanProcessOutput(ctx, owner, run, test.outputMode, time.Now(), strings.NewReader(test.lines), output)
			<-output

			handoffs := 0
			for len(owner.runtimeEvents) > 0 {
				event := <-owner.runtimeEvents
				if event.kind != launcherProcessSplashReadyEvent {
					t.Fatalf("unexpected process event %v", event.kind)
				}
				if event.process.run != run || event.process.outputMode != test.outputMode {
					t.Fatalf("handoff associated with wrong process: %#v", event.process)
				}
				handoffs++
			}
			if handoffs != test.wantHandoffs {
				t.Fatalf("handoffs = %d, want %d", handoffs, test.wantHandoffs)
			}
		})
	}
}

func TestProcessSupervisorActiveRunGuardsHandoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := &managedProcess{ctx: ctx, done: make(chan struct{})}
	supervisor := &processSupervisor{active: run}
	if !supervisor.activeRun(run) {
		t.Fatal("active process was rejected")
	}
	if supervisor.activeRun(&managedProcess{ctx: ctx, done: make(chan struct{})}) {
		t.Fatal("stale process was accepted")
	}
	cancel()
	if supervisor.activeRun(run) {
		t.Fatal("canceled process was accepted")
	}
	otherContext, cancelOther := context.WithCancel(context.Background())
	defer cancelOther()
	otherRun := &managedProcess{ctx: otherContext, done: make(chan struct{})}
	supervisor.active = otherRun
	close(otherRun.done)
	if supervisor.activeRun(otherRun) {
		t.Fatal("completed process was accepted")
	}
}

func TestWatchLaunchGraceQueuesOnLauncherThread(t *testing.T) {
	previousMode, previousOutputMode := launcherConfig.CurrentMode, launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode, launcherConfig.CurrentPMode = previousMode, previousOutputMode
	})
	launcherConfig.CurrentMode, launcherConfig.CurrentPMode = CursorDance, Watch

	runtimeDone := make(chan struct{})
	l := &launcher{
		bld:           &builder{},
		runtimeDone:   runtimeDone,
		runtimeEvents: make(chan launcherEvent, 1),
	}
	t.Cleanup(func() {
		l.cancelPendingDanserStart()
		close(runtimeDone)
	})

	clickedAt := time.Now().Add(-2 * time.Second)
	l.scheduleDanserStart(clickedAt)
	pending := l.pendingStart
	if pending == nil || pending.timer == nil {
		t.Fatal("Watch click did not schedule a timer")
	}
	if !pending.clickedAt.Equal(clickedAt) || pending.mode != CursorDance || pending.outputMode != Watch {
		t.Fatalf("pending request lost click-time state: %#v", pending)
	}
	l.scheduleDanserStart(clickedAt.Add(time.Second))
	if l.pendingStart != pending {
		t.Fatal("repeated click replaced the pending launch")
	}

	select {
	case event := <-l.runtimeEvents:
		if event.kind != launcherStartupTaskEvent || event.task == nil {
			t.Fatalf("unexpected Watch timer event: %#v", event)
		}
		// Cancellation after the timer posts but before the main thread drains
		// the task must prevent a child from starting.
		l.cancelPendingDanserStart()
		event.task()
		if l.pendingStart != nil {
			t.Fatal("stale timer task recreated a canceled launch")
		}
	case <-time.After(watchLaunchGracePeriod + 2*time.Second):
		t.Fatal("Watch timer did not post the launcher event")
	}
}

func TestCanceledOrChangedWatchRequestCannotLaunch(t *testing.T) {
	previousMode, previousOutputMode := launcherConfig.CurrentMode, launcherConfig.CurrentPMode
	t.Cleanup(func() {
		launcherConfig.CurrentMode, launcherConfig.CurrentPMode = previousMode, previousOutputMode
	})
	launcherConfig.CurrentMode, launcherConfig.CurrentPMode = CursorDance, Watch

	runtimeDone := make(chan struct{})
	l := &launcher{
		bld:           &builder{},
		runtimeDone:   runtimeDone,
		runtimeEvents: make(chan launcherEvent, 2),
	}
	t.Cleanup(func() {
		l.cancelPendingDanserStart()
		close(runtimeDone)
	})

	l.scheduleDanserStart(time.Now())
	canceled := l.pendingStart
	l.cancelPendingDanserStart()
	l.scheduleDanserStart(time.Now())
	newRequest := l.pendingStart
	l.launchPendingDanserStart(canceled)
	if l.pendingStart != newRequest {
		t.Fatal("stale timer consumed a newer pending launch")
	}

	launcherConfig.CurrentMode = Replay
	l.launchPendingDanserStart(newRequest)
	if l.pendingStart != nil {
		t.Fatal("changed mode remained pending after canceled handoff")
	}
	if l.danserRunning {
		t.Fatal("changed mode started gameplay")
	}
}

func TestDropIgnoredDuringWatchLaunchGrace(t *testing.T) {
	l := &launcher{pendingStart: &pendingDanserStart{}}
	// Replay and archive drops would otherwise change the selected map,
	// even while all ImGui controls are disabled.
	l.handleDrop(gcontext.DropEvent{Names: []string{"another-replay.osr", "new-map.osz"}})
	if l.pendingStart == nil {
		t.Fatal("file drop interrupted the pending Watch launch")
	}
}
