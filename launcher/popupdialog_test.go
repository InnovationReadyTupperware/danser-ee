package launcher

import (
	"testing"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
)

func TestPopupDialogDefaultProperties(t *testing.T) {
	properties := defaultPopupDialogProperties()
	if !properties.dismissOnEscape {
		t.Fatal("dismissOnEscape = false, want true")
	}
	if !properties.dismissOnClickOutside {
		t.Fatal("dismissOnClickOutside = false, want true")
	}
	if !properties.animateTransition {
		t.Fatal("animateTransition = false, want true")
	}
	if properties.scrimColor != (imgui.Vec4{W: 0.6}) {
		t.Fatalf("scrimColor = %#v, want 60%% black", properties.scrimColor)
	}
}

func TestPopupDialogActionAppearances(t *testing.T) {
	filled := newPopupDialogAction("Apply", nil)
	if filled.appearance != popupDialogActionFilled {
		t.Fatalf("newPopupDialogAction() appearance = %v, want filled", filled.appearance)
	}

	outlined := outlinedPopupDialogAction("Cancel", nil)
	if outlined.appearance != popupDialogActionOutlined {
		t.Fatalf("outlinedPopupDialogAction() appearance = %v, want outlined", outlined.appearance)
	}

	destructive := destructivePopupDialogAction("Delete", nil)
	if !destructive.destructive {
		t.Fatal("destructivePopupDialogAction() destructive = false, want true")
	}
}

func TestPopupDialogOptionsApplyReusableBehavior(t *testing.T) {
	scrim := imgui.Vec4{X: 0.1, Y: 0.2, Z: 0.3, W: 0.4}
	dialog := newPopupDialog(
		"options",
		"Options",
		withPopupDialogTone(popupDialogToneWarning),
		withPopupDialogExtraAction(newPopupDialogAction("Help", nil)),
		withPopupDialogDismissOnEscape(false),
		withPopupDialogDismissOnClickOutside(false),
		withPopupDialogScrimColor(scrim),
	)

	if dialog.tone != popupDialogToneWarning {
		t.Fatalf("tone = %v, want warning", dialog.tone)
	}
	if dialog.extraAction == nil || dialog.extraAction.label != "Help" {
		t.Fatalf("extraAction = %#v, want Help action", dialog.extraAction)
	}
	if dialog.properties.dismissOnEscape {
		t.Fatal("dismissOnEscape = true, want false")
	}
	if dialog.properties.dismissOnClickOutside {
		t.Fatal("dismissOnClickOutside = true, want false")
	}
	if dialog.properties.scrimColor != scrim {
		t.Fatalf("scrimColor = %#v, want %#v", dialog.properties.scrimColor, scrim)
	}
}

func TestOutputDialogCopyFor(t *testing.T) {
	tests := []struct {
		name string
		mode PMode
		want outputDialogCopy
	}{
		{
			name: "recording",
			mode: Record,
			want: outputDialogCopy{
				title:           "Set recording output",
				supportingText:  "Choose the video file name.",
				outputNameLabel: "Video file name",
			},
		},
		{
			name: "screenshot",
			mode: Screenshot,
			want: outputDialogCopy{
				title:           "Set screenshot output",
				supportingText:  "Choose the screenshot file name and capture time.",
				outputNameLabel: "Screenshot file name",
			},
		},
		{
			name: "fallback",
			mode: Watch,
			want: outputDialogCopy{
				title:           "Set output",
				supportingText:  "Choose the output file name.",
				outputNameLabel: "File name",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := outputDialogCopyFor(test.mode); got != test.want {
				t.Fatalf("outputDialogCopyFor(%v) = %#v, want %#v", test.mode, got, test.want)
			}
		})
	}
}

func TestPopupDialogActionsAreOptional(t *testing.T) {
	dialog := newPopupDialog("no-actions", "No actions")
	if dialog.hasActions() {
		t.Fatal("hasActions() = true, want false")
	}

	dialog = newPopupDialog(
		"primary-action",
		"Primary action",
		withPopupDialogPrimaryAction(newPopupDialogAction("Apply", nil)),
	)
	if !dialog.hasActions() {
		t.Fatal("hasActions() = false with primary action")
	}
}

func TestPopupDialogDismissible(t *testing.T) {
	tests := []struct {
		name       string
		properties popupDialogProperties
		want       bool
	}{
		{name: "default", properties: defaultPopupDialogProperties(), want: true},
		{name: "escape only", properties: popupDialogProperties{dismissOnEscape: true}, want: true},
		{name: "outside click only", properties: popupDialogProperties{dismissOnClickOutside: true}, want: true},
		{name: "non-dismissible"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dialog := popupDialog{properties: test.properties}
			if got := dialog.dismissible(); got != test.want {
				t.Fatalf("dismissible() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestPopupDialogCloseButtonIsHiddenWhenDismissActionExists(t *testing.T) {
	dialog := newPopupDialog(
		"dismiss-action",
		"Dismiss action",
		withPopupDialogDismissAction(outlinedPopupDialogAction("Cancel", nil)),
	)

	if dialog.showCloseButton() {
		t.Fatal("showCloseButton() = true with explicit dismiss action")
	}

	dialog = newPopupDialog("environmental-dismiss", "Environmental dismiss")
	if !dialog.showCloseButton() {
		t.Fatal("showCloseButton() = false without explicit dismiss action")
	}
}

func TestPopupDialogBlendColor(t *testing.T) {
	from := imgui.Vec4{X: 0, Y: 0.2, Z: 0.4, W: 0.6}
	to := imgui.Vec4{X: 1, Y: 0.8, Z: 0.6, W: 1}

	if got := popupDialogBlendColor(from, to, 0.5); got != (imgui.Vec4{X: 0.5, Y: 0.5, Z: 0.5, W: 0.8}) {
		t.Fatalf("popupDialogBlendColor() = %#v, want midpoint", got)
	}
	if got := popupDialogBlendColor(from, to, -1); got != from {
		t.Fatalf("popupDialogBlendColor() below range = %#v, want from color", got)
	}
	if got := popupDialogBlendColor(from, to, 2); got != to {
		t.Fatalf("popupDialogBlendColor() above range = %#v, want to color", got)
	}
}

func TestShouldDismissPopupDialog(t *testing.T) {
	tests := []struct {
		name       string
		properties popupDialogProperties
		input      popupDialogDismissInput
		want       bool
	}{
		{
			name:       "escape enabled",
			properties: defaultPopupDialogProperties(),
			input:      popupDialogDismissInput{escapePressed: true},
			want:       true,
		},
		{
			name: "escape disabled",
			properties: popupDialogProperties{
				dismissOnClickOutside: true,
			},
			input: popupDialogDismissInput{escapePressed: true},
		},
		{
			name:       "primary release outside",
			properties: defaultPopupDialogProperties(),
			input: popupDialogDismissInput{
				outsideMouseReleased: true,
				outsideMouseButton:   imgui.MouseButtonLeft,
			},
			want: true,
		},
		{
			name:       "secondary release outside",
			properties: defaultPopupDialogProperties(),
			input: popupDialogDismissInput{
				outsideMouseReleased: true,
				outsideMouseButton:   imgui.MouseButtonRight,
			},
		},
		{
			name: "outside click disabled",
			properties: popupDialogProperties{
				dismissOnEscape: true,
			},
			input: popupDialogDismissInput{
				outsideMouseReleased: true,
				outsideMouseButton:   imgui.MouseButtonLeft,
			},
		},
		{
			name:       "no input",
			properties: defaultPopupDialogProperties(),
		},
		{
			name: "fully non-dismissible",
			input: popupDialogDismissInput{
				escapePressed:        true,
				outsideMouseReleased: true,
				outsideMouseButton:   imgui.MouseButtonLeft,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldDismissPopupDialog(test.properties, test.input); got != test.want {
				t.Fatalf("shouldDismissPopupDialog() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestPopupDialogTransitionMath(t *testing.T) {
	tests := []struct {
		name      string
		progress  float32
		wantEase  float32
		wantAlpha float32
		wantY     float32
	}{
		{name: "start", progress: 0, wantEase: 0, wantAlpha: 0.2, wantY: 10},
		{name: "half", progress: 0.5, wantEase: 0.75, wantAlpha: 0.6, wantY: 5},
		{name: "end", progress: 1, wantEase: 1, wantAlpha: 1, wantY: 0},
		{name: "below range", progress: -1, wantEase: 0, wantAlpha: 0.2, wantY: 10},
		{name: "above range", progress: 2, wantEase: 1, wantAlpha: 1, wantY: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := popupDialogEaseOut(test.progress); got != test.wantEase {
				t.Fatalf("popupDialogEaseOut(%v) = %v, want %v", test.progress, got, test.wantEase)
			}
			if got := popupDialogContentAlpha(test.progress); got != test.wantAlpha {
				t.Fatalf("popupDialogContentAlpha(%v) = %v, want %v", test.progress, got, test.wantAlpha)
			}
			if got := popupDialogTranslationY(test.progress); got != test.wantY {
				t.Fatalf("popupDialogTranslationY(%v) = %v, want %v", test.progress, got, test.wantY)
			}
		})
	}
}

func TestPopupDialogDurationProgress(t *testing.T) {
	tests := []struct {
		name     string
		elapsed  time.Duration
		duration time.Duration
		want     float32
	}{
		{name: "before start", elapsed: -time.Millisecond, duration: time.Second, want: 0},
		{name: "start", duration: time.Second, want: 0},
		{name: "half", elapsed: 500 * time.Millisecond, duration: time.Second, want: 0.5},
		{name: "end", elapsed: time.Second, duration: time.Second, want: 1},
		{name: "past end", elapsed: 2 * time.Second, duration: time.Second, want: 1},
		{name: "zero duration", duration: 0, want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := popupDialogDurationProgress(test.elapsed, test.duration); got != test.want {
				t.Fatalf("popupDialogDurationProgress() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPopupDialogActionLayout(t *testing.T) {
	tests := []struct {
		name       string
		available  float32
		widths     []float32
		hasLeading bool
		want       bool
	}{
		{name: "one action fits", available: 280, widths: []float32{100}},
		{name: "two actions fit", available: 280, widths: []float32{90, 100}},
		{name: "two actions overflow", available: 180, widths: []float32{90, 100}, want: true},
		{name: "three actions fit", available: 420, widths: []float32{100, 90, 100}, hasLeading: true},
		{name: "three actions reserve leading gap", available: 320, widths: []float32{100, 90, 100}, hasLeading: true, want: true},
		{name: "no actions", available: 100},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := popupDialogUseVerticalActions(test.available, test.widths, test.hasLeading); got != test.want {
				t.Fatalf("popupDialogUseVerticalActions() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestPopupDialogWidthBounds(t *testing.T) {
	tests := []struct {
		name    string
		window  float32
		wantMin float32
		wantMax float32
	}{
		{name: "wide window", window: 1920, wantMin: 280, wantMax: 560},
		{name: "medium window", window: 500, wantMin: 280, wantMax: 436},
		{name: "narrow window", window: 250, wantMin: 186, wantMax: 186},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotMin, gotMax := popupDialogWidthBounds(test.window)
			if gotMin != test.wantMin || gotMax != test.wantMax {
				t.Fatalf("popupDialogWidthBounds(%v) = (%v, %v), want (%v, %v)", test.window, gotMin, gotMax, test.wantMin, test.wantMax)
			}
		})
	}
}

func TestPopupDialogWidthUsesDesktopPreferredSizeWithinBounds(t *testing.T) {
	tests := []struct {
		name   string
		window float32
		want   float32
	}{
		{name: "wide window", window: 1920, want: 440},
		{name: "medium window", window: 500, want: 436},
		{name: "narrow window", window: 250, want: 186},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := popupDialogWidth(test.window); got != test.want {
				t.Fatalf("popupDialogWidth(%v) = %v, want %v", test.window, got, test.want)
			}
		})
	}
}

func TestPopupDialogDismissRequestFiresOnce(t *testing.T) {
	now := time.Unix(0, 0)
	requests := 0
	dialog := newPopupDialog(
		"dismiss-once",
		"Dismiss once",
		withPopupDialogPrimaryAction(newPopupDialogAction("OK", nil)),
		withPopupDialogDismissRequest(func() { requests++ }),
	)
	dialog.openAt(now)
	dialog.phase = popupDialogPhaseVisible

	dialog.requestDismissAt(now)
	dialog.requestDismissAt(now.Add(time.Millisecond))

	if requests != 1 {
		t.Fatalf("dismiss requests = %d, want 1", requests)
	}
	if dialog.phase != popupDialogPhaseDisappearing {
		t.Fatalf("phase = %v, want disappearing", dialog.phase)
	}
}

func TestPopupDialogActionsDismissUnlessKeptOpen(t *testing.T) {
	now := time.Unix(0, 0)
	clicks := 0
	dismissRequests := 0
	dialog := newPopupDialog(
		"actions",
		"Actions",
		withPopupDialogPrimaryAction(newPopupDialogAction("OK", nil)),
		withPopupDialogAnimation(false),
		withPopupDialogDismissRequest(func() { dismissRequests++ }),
	)
	dialog.openAt(now)

	dialog.activateActionAt(keepOpenPopupDialogAction("Refresh", func() { clicks++ }), now)
	if clicks != 1 {
		t.Fatalf("keep-open clicks = %d, want 1", clicks)
	}
	if dialog.phase != popupDialogPhaseVisible || !dialog.opened {
		t.Fatalf("keep-open action closed dialog: phase=%v opened=%t", dialog.phase, dialog.opened)
	}

	dialog.activateActionAt(newPopupDialogAction("Done", func() { clicks++ }), now)
	if clicks != 2 {
		t.Fatalf("total clicks = %d, want 2", clicks)
	}
	if dialog.phase != popupDialogPhaseClosed || dialog.opened {
		t.Fatalf("normal action did not close dialog: phase=%v opened=%t", dialog.phase, dialog.opened)
	}
	if dismissRequests != 0 {
		t.Fatalf("action invoked onDismissRequest %d times, want 0", dismissRequests)
	}
}

func TestPopupDialogActionIsIgnoredOnceDismissalStarts(t *testing.T) {
	now := time.Unix(0, 0)
	clicks := 0
	dialog := newPopupDialog(
		"double-action",
		"Double action",
		withPopupDialogPrimaryAction(newPopupDialogAction("Apply", nil)),
	)
	dialog.openAt(now)
	dialog.phase = popupDialogPhaseVisible
	action := newPopupDialogAction("Apply", func() { clicks++ })

	dialog.activateActionAt(action, now)
	dialog.activateActionAt(action, now.Add(time.Millisecond))

	if clicks != 1 {
		t.Fatalf("action clicks = %d, want 1 while disappearing", clicks)
	}
}

func TestPopupDialogAppearanceBecomesVisibleAfterComposeDuration(t *testing.T) {
	start := time.Unix(0, 0)
	dialog := newPopupDialog(
		"appearance",
		"Appearance",
		withPopupDialogPrimaryAction(newPopupDialogAction("OK", nil)),
	)
	dialog.openAt(start)

	dialog.updateTransition(start.Add(popupDialogAppearanceDuration - time.Nanosecond))
	if dialog.phase != popupDialogPhaseAppearing {
		t.Fatalf("phase before appearance duration = %v, want appearing", dialog.phase)
	}

	dialog.updateTransition(start.Add(popupDialogAppearanceDuration))
	if dialog.phase != popupDialogPhaseVisible {
		t.Fatalf("phase at appearance duration = %v, want visible", dialog.phase)
	}
}

func TestPopupDialogDisappearanceScalesFromCurrentProgress(t *testing.T) {
	start := time.Unix(0, 0)
	dialog := newPopupDialog(
		"partial",
		"Partial",
		withPopupDialogPrimaryAction(newPopupDialogAction("OK", nil)),
	)
	dialog.openAt(start)

	dismissAt := start.Add(popupDialogAppearanceDuration / 2)
	wantStart := popupDialogEaseOut(0.5)
	dialog.beginDismissAt(dismissAt, false)

	if dialog.dismissStartProgress != wantStart {
		t.Fatalf("dismissStartProgress = %v, want %v", dialog.dismissStartProgress, wantStart)
	}
	wantDuration := time.Duration(float64(popupDialogDisappearanceDuration) * float64(wantStart))
	if got := popupDialogDisappearanceDurationFor(dialog.dismissStartProgress); got != wantDuration {
		t.Fatalf("disappearance duration = %v, want %v", got, wantDuration)
	}

	dialog.updateTransition(dismissAt.Add(wantDuration))
	if dialog.phase != popupDialogPhaseClosed || dialog.opened {
		t.Fatalf("completed disappearance = phase %v opened %t, want closed", dialog.phase, dialog.opened)
	}
}
