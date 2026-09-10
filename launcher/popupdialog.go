package launcher

import (
	"time"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

const (
	popupDialogMinWidth       = float32(280)
	popupDialogMaxWidth       = float32(560)
	popupDialogPreferredWidth = float32(440)

	popupDialogWindowMargin = float32(32)
	popupDialogPadding      = float32(24)
	popupDialogActionGap    = float32(8)
	popupDialogButtonHeight = float32(34)
	popupDialogButtonRadius = float32(4)
	popupDialogCloseSize    = float32(32)
	popupDialogTitleSize    = float32(24)
	popupDialogBodySize     = float32(18)

	popupDialogInitialAlpha = float32(0.2)
	popupDialogOffsetY      = float32(10)

	popupDialogAppearanceDuration    = 200 * time.Millisecond
	popupDialogDisappearanceDuration = 100 * time.Millisecond
)

type popupDialogPhase uint8

const (
	popupDialogPhaseClosed popupDialogPhase = iota
	popupDialogPhaseAppearing
	popupDialogPhaseVisible
	popupDialogPhaseDisappearing
)

type popupDialogTone uint8

const (
	popupDialogToneDefault popupDialogTone = iota
	popupDialogToneInfo
	popupDialogToneWarning
	popupDialogToneDanger
)

type popupDialogProperties struct {
	dismissOnEscape       bool
	dismissOnClickOutside bool
	animateTransition     bool
	scrimColor            imgui.Vec4
}

type popupDialogActionAppearance uint8

const (
	popupDialogActionFilled popupDialogActionAppearance = iota
	popupDialogActionOutlined
)

type popupDialogAction struct {
	label       string
	onClick     func()
	closeWhen   func() bool
	appearance  popupDialogActionAppearance
	keepOpen    bool
	destructive bool
}

type popupDialogDismissInput struct {
	escapePressed        bool
	outsideMouseReleased bool
	outsideMouseButton   imgui.MouseButton
}

type popupDialogPalette struct {
	surface        imgui.Vec4
	contentSurface imgui.Vec4
	text           imgui.Vec4
	secondaryText  imgui.Vec4
	border         imgui.Vec4
	button         imgui.Vec4
	buttonHovered  imgui.Vec4
	buttonActive   imgui.Vec4
}

type popupDialogOption func(*popupDialog)

type popupDialog struct {
	name           string
	title          string
	supportingText string
	content        func()
	validationText func() string

	primaryAction *popupDialogAction
	dismissAction *popupDialogAction
	extraAction   *popupDialogAction

	tone       popupDialogTone
	properties popupDialogProperties

	onDismissRequest func()

	opened               bool
	phase                popupDialogPhase
	transitionStarted    time.Time
	dismissStartProgress float32
	dismissRequestCalled bool
}

var _ iPopup = (*popupDialog)(nil)

func newPopupDialog(
	name string,
	title string,
	options ...popupDialogOption,
) *popupDialog {
	dialog := &popupDialog{
		name:                 name,
		title:                title,
		properties:           defaultPopupDialogProperties(),
		phase:                popupDialogPhaseClosed,
		dismissStartProgress: 1,
	}

	for _, option := range options {
		if option != nil {
			option(dialog)
		}
	}

	return dialog
}

func newPopupDialogAction(label string, onClick func()) popupDialogAction {
	return popupDialogAction{
		label:      label,
		onClick:    onClick,
		appearance: popupDialogActionFilled,
	}
}

func outlinedPopupDialogAction(label string, onClick func()) popupDialogAction {
	action := newPopupDialogAction(label, onClick)
	action.appearance = popupDialogActionOutlined
	return action
}

func destructivePopupDialogAction(label string, onClick func()) popupDialogAction {
	action := newPopupDialogAction(label, onClick)
	action.destructive = true
	return action
}

func keepOpenPopupDialogAction(label string, onClick func()) popupDialogAction {
	action := newPopupDialogAction(label, onClick)
	action.keepOpen = true
	return action
}

func conditionallyClosingPopupDialogAction(
	action popupDialogAction,
	closeWhen func() bool,
) popupDialogAction {
	action.closeWhen = closeWhen
	return action
}

func withPopupDialogSupportingText(text string) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.supportingText = text
	}
}

func withPopupDialogContent(content func()) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.content = content
	}
}

func withPopupDialogValidationText(validationText func() string) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.validationText = validationText
	}
}

func withPopupDialogPrimaryAction(action popupDialogAction) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.primaryAction = new(action)
	}
}

func withPopupDialogTone(tone popupDialogTone) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.tone = tone
	}
}

func withPopupDialogDismissAction(action popupDialogAction) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.dismissAction = new(action)
	}
}

func withPopupDialogExtraAction(action popupDialogAction) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.extraAction = new(action)
	}
}

func withPopupDialogDismissOnEscape(enabled bool) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.properties.dismissOnEscape = enabled
	}
}

func withPopupDialogDismissOnClickOutside(enabled bool) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.properties.dismissOnClickOutside = enabled
	}
}

func withPopupDialogAnimation(enabled bool) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.properties.animateTransition = enabled
	}
}

func withPopupDialogScrimColor(color imgui.Vec4) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.properties.scrimColor = color
	}
}

func withPopupDialogDismissRequest(onDismissRequest func()) popupDialogOption {
	return func(dialog *popupDialog) {
		dialog.onDismissRequest = onDismissRequest
	}
}

func defaultPopupDialogProperties() popupDialogProperties {
	return popupDialogProperties{
		dismissOnEscape:       true,
		dismissOnClickOutside: true,
		animateTransition:     true,
		scrimColor:            vec4(0, 0, 0, 0.6),
	}
}

func (d *popupDialog) open() {
	d.openAt(time.Now())
}

func (d *popupDialog) openAt(now time.Time) {
	d.opened = true
	d.dismissRequestCalled = false
	d.dismissStartProgress = 1
	d.transitionStarted = now

	if d.properties.animateTransition {
		d.phase = popupDialogPhaseAppearing
		return
	}

	d.phase = popupDialogPhaseVisible
}

func (d *popupDialog) draw() {
	now := time.Now()
	d.updateTransition(now)
	if d.phase == popupDialogPhaseClosed || !d.opened {
		return
	}

	popupName := "##popup-dialog-" + d.name
	if !imgui.IsPopupOpenStr(popupName) {
		imgui.OpenPopupStr(popupName)
		imgui.SetNextWindowFocus()
	}

	progress := d.appearanceProgress(now)
	contentAlpha := popupDialogContentAlpha(progress)
	palette := currentPopupDialogPalette()
	windowWidth := float32(settings.Graphics.WindowWidth)
	windowHeight := float32(settings.Graphics.WindowHeight)
	dialogWidth := popupDialogWidth(windowWidth)
	maxHeight := max(float32(1), windowHeight-popupDialogWindowMargin*2)

	imgui.SetNextWindowSizeConstraints(vec2(dialogWidth, 0), vec2(dialogWidth, maxHeight))
	imgui.SetNextWindowPosV(
		vec2(windowWidth/2, windowHeight/2+popupDialogTranslationY(progress)),
		imgui.CondAlways,
		vec2(0.5, 0.5),
	)

	surfaceColor := palette.surface
	surfaceColor.W *= contentAlpha
	borderColor := palette.border
	borderColor.W *= 0.45 * contentAlpha
	scrimColor := d.properties.scrimColor
	scrimColor.W *= contentAlpha

	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, vec2(popupDialogPadding, popupDialogPadding))
	imgui.PushStyleVarFloat(imgui.StyleVarPopupRounding, 12)
	imgui.PushStyleVarFloat(imgui.StyleVarPopupBorderSize, 0.5)
	imgui.PushStyleColorVec4(imgui.ColPopupBg, surfaceColor)
	imgui.PushStyleColorVec4(imgui.ColBorder, borderColor)
	imgui.PushStyleColorVec4(imgui.ColModalWindowDimBg, scrimColor)

	flags := imgui.WindowFlagsNoCollapse |
		imgui.WindowFlagsNoResize |
		imgui.WindowFlagsAlwaysAutoResize |
		imgui.WindowFlagsNoMove |
		imgui.WindowFlagsNoTitleBar |
		imgui.WindowFlagsNoSavedSettings

	if imgui.BeginPopupModalV(popupName, &d.opened, flags) {
		imgui.PushStyleVarFloat(imgui.StyleVarAlpha, contentAlpha)
		d.drawContent(windowHeight, palette)

		if d.phase == popupDialogPhaseAppearing || d.phase == popupDialogPhaseVisible {
			hovered := imgui.IsWindowHoveredV(
				imgui.HoveredFlagsRootAndChildWindows|
					imgui.HoveredFlagsAllowWhenBlockedByActiveItem|
					imgui.HoveredFlagsAllowWhenBlockedByPopup,
			) || openedAbove

			input := popupDialogDismissInput{
				escapePressed:        imgui.IsKeyPressedBool(imgui.KeyEscape),
				outsideMouseReleased: !hovered && imgui.IsMouseReleased(imgui.MouseButtonLeft),
				outsideMouseButton:   imgui.MouseButtonLeft,
			}
			if shouldDismissPopupDialog(d.properties, input) {
				d.requestDismissAt(now)
			}
		}

		openedAbove = true
		if d.phase == popupDialogPhaseClosed || !d.opened {
			imgui.CloseCurrentPopup()
		}

		imgui.PopStyleVar()
		imgui.EndPopup()
	}

	imgui.PopStyleColorV(3)
	imgui.PopStyleVarV(3)

	if !d.opened && d.phase != popupDialogPhaseClosed {
		d.phase = popupDialogPhaseClosed
	}
}

func (d *popupDialog) shouldClose() bool {
	return d.phase == popupDialogPhaseClosed
}

func (d *popupDialog) dismissible() bool {
	return d.properties.dismissOnEscape || d.properties.dismissOnClickOutside
}

func (d *popupDialog) showCloseButton() bool {
	return d.dismissible() && d.dismissAction == nil
}

func (d *popupDialog) drawContent(windowHeight float32, palette popupDialogPalette) {
	d.drawHeader(palette)

	if d.supportingText != "" {
		dummyExactY(4)
		imgui.PushFont(Font, popupDialogBodySize)
		imgui.PushStyleColorVec4(imgui.ColText, palette.secondaryText)
		imgui.PushTextWrapPos()
		imgui.TextUnformattedV(d.supportingText)
		imgui.PopTextWrapPos()
		imgui.PopStyleColor()
		imgui.PopFont()
	}

	if d.content != nil {
		dummyExactY(16)
		d.drawBody(windowHeight, palette)
	}

	if d.validationText != nil {
		if validationText := d.validationText(); validationText != "" {
			dummyExactY(8)
			d.drawValidationText(validationText)
		}
	}

	if d.hasActions() {
		dummyExactY(24)
		d.drawActions(palette)
	}
}

func (d *popupDialog) drawValidationText(text string) {
	_, errorColor := popupDialogToneAppearance(
		popupDialogToneDanger,
		*imgui.StyleColorVec4(imgui.ColText),
	)

	imgui.PushStyleColorVec4(imgui.ColText, errorColor)
	imgui.PushFont(FontAw, 14)
	imgui.TextUnformatted("\uf06a")
	imgui.PopFont()
	imgui.SameLineV(0, 6)
	imgui.PushFont(Font, 15)
	imgui.PushTextWrapPos()
	imgui.TextUnformatted(text)
	imgui.PopTextWrapPos()
	imgui.PopFont()
	imgui.PopStyleColor()
}

func (d *popupDialog) drawHeader(palette popupDialogPalette) {
	start := imgui.CursorPos()
	available := imgui.ContentRegionAvail().X
	titleWidth := available
	if d.showCloseButton() {
		titleWidth -= popupDialogCloseSize + popupDialogActionGap
	}

	glyph, toneColor := popupDialogToneAppearance(d.tone, palette.text)
	if glyph != "" {
		imgui.PushFont(FontAw, 18)
		imgui.PushStyleColorVec4(imgui.ColText, toneColor)
		imgui.TextUnformattedV(glyph)
		imgui.PopStyleColor()
		imgui.PopFont()
		imgui.SameLineV(0, 8)
	}

	imgui.PushFont(Font, popupDialogTitleSize)
	imgui.PushStyleColorVec4(imgui.ColText, palette.text)
	imgui.PushTextWrapPosV(start.X + titleWidth)
	imgui.TextUnformattedV(d.title)
	imgui.PopTextWrapPos()
	imgui.PopStyleColor()
	imgui.PopFont()
	afterTitleY := imgui.CursorPos().Y

	if !d.showCloseButton() {
		return
	}

	imgui.SetCursorPos(vec2(start.X+available-popupDialogCloseSize, start.Y))
	if d.drawCloseButton(palette) {
		d.requestDismissAt(time.Now())
	}
	imgui.SetCursorPos(vec2(start.X, max(afterTitleY, start.Y+popupDialogCloseSize)))
}

func (d *popupDialog) drawCloseButton(palette popupDialogPalette) bool {
	hovered := palette.buttonHovered
	hovered.W *= 0.7
	active := palette.buttonActive
	active.W *= 0.85

	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, popupDialogButtonRadius)
	imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
	imgui.PushStyleColorVec4(imgui.ColButton, vec4(0, 0, 0, 0))
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, hovered)
	imgui.PushStyleColorVec4(imgui.ColButtonActive, active)
	imgui.PushStyleColorVec4(imgui.ColText, palette.text)
	imgui.PushFont(FontAw, 15)
	pressed := imgui.ButtonV("\uf00d##popup-dialog-close-"+d.name, vec2(popupDialogCloseSize, popupDialogCloseSize))
	imgui.PopFont()
	imgui.PopStyleColorV(4)
	imgui.PopStyleVarV(2)
	return pressed
}

func (d *popupDialog) drawBody(windowHeight float32, palette popupDialogPalette) {
	maxBodyHeight := min(float32(320), max(float32(120), windowHeight*0.45))

	imgui.PushStyleVarFloat(imgui.StyleVarChildRounding, 8)
	imgui.PushStyleVarFloat(imgui.StyleVarChildBorderSize, 1)
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, vec2(14, 14))
	imgui.PushStyleColorVec4(imgui.ColChildBg, palette.contentSurface)
	imgui.PushStyleColorVec4(imgui.ColBorder, palette.border)

	imgui.SetNextWindowSizeConstraints(
		vec2(0, 0),
		vec2(imgui.ContentRegionAvail().X, maxBodyHeight),
	)
	imgui.BeginChildStrV(
		"##popup-dialog-body-"+d.name,
		vec2(0, 0),
		imgui.ChildFlagsAutoResizeY|imgui.ChildFlagsAlwaysUseWindowPadding,
		imgui.WindowFlagsNoSavedSettings,
	)
	d.content()
	imgui.EndChild()

	imgui.PopStyleColorV(2)
	imgui.PopStyleVarV(3)
}

func (d *popupDialog) drawActions(palette popupDialogPalette) {
	primaryWidth := float32(0)
	dismissWidth := float32(0)
	extraWidth := float32(0)
	widths := make([]float32, 0, 3)
	hasLeadingAction := d.extraAction != nil
	if d.primaryAction != nil {
		primaryWidth = d.actionWidth(*d.primaryAction)
	}

	if d.extraAction != nil {
		extraWidth = d.actionWidth(*d.extraAction)
		widths = append(widths, extraWidth)
	}
	if d.dismissAction != nil {
		dismissWidth = d.actionWidth(*d.dismissAction)
		widths = append(widths, dismissWidth)
	}
	if d.primaryAction != nil {
		widths = append(widths, primaryWidth)
	}

	available := imgui.ContentRegionAvail().X
	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, popupDialogButtonRadius)

	if popupDialogUseVerticalActions(available, widths, hasLeadingAction) {
		d.drawVerticalActions(available, palette)
		imgui.PopStyleVar()
		return
	}

	startX := imgui.CursorPosX()
	if d.extraAction != nil {
		if d.drawActionButton(*d.extraAction, "extra", extraWidth, palette) {
			d.activateActionAt(*d.extraAction, time.Now())
		}
		imgui.SameLineV(0, 0)
	}

	rightWidth := primaryWidth
	if d.dismissAction != nil {
		if rightWidth > 0 {
			rightWidth += popupDialogActionGap
		}
		rightWidth += dismissWidth
	}
	if rightWidth > 0 {
		imgui.SetCursorPosX(startX + available - rightWidth)
	}

	if d.dismissAction != nil {
		if d.drawActionButton(*d.dismissAction, "dismiss", dismissWidth, palette) {
			d.activateActionAt(*d.dismissAction, time.Now())
		}
		if d.primaryAction != nil {
			imgui.SameLineV(0, popupDialogActionGap)
		}
	}
	if d.primaryAction != nil && d.drawActionButton(*d.primaryAction, "primary", primaryWidth, palette) {
		d.activateActionAt(*d.primaryAction, time.Now())
	}

	imgui.PopStyleVar()
}

func (d *popupDialog) drawVerticalActions(available float32, palette popupDialogPalette) {
	drawn := false
	if d.primaryAction != nil {
		if d.drawActionButton(*d.primaryAction, "primary", available, palette) {
			d.activateActionAt(*d.primaryAction, time.Now())
		}
		drawn = true
	}

	if d.dismissAction != nil {
		if drawn {
			dummyExactY(popupDialogActionGap)
		}
		if d.drawActionButton(*d.dismissAction, "dismiss", available, palette) {
			d.activateActionAt(*d.dismissAction, time.Now())
		}
		drawn = true
	}

	if d.extraAction != nil {
		if drawn {
			dummyExactY(popupDialogActionGap)
		}
		if d.drawActionButton(*d.extraAction, "extra", available, palette) {
			d.activateActionAt(*d.extraAction, time.Now())
		}
	}
}

func (d *popupDialog) hasActions() bool {
	return d.primaryAction != nil || d.dismissAction != nil || d.extraAction != nil
}

func (d *popupDialog) drawActionButton(
	action popupDialogAction,
	idSuffix string,
	width float32,
	palette popupDialogPalette,
) bool {
	label := action.label + "##popup-dialog-" + d.name + "-" + idSuffix
	if action.appearance == popupDialogActionOutlined {
		return d.drawOutlinedActionButton(label, width, palette)
	}

	base := popupDialogBlendColor(palette.button, palette.buttonHovered, 0.55)
	hovered := palette.buttonHovered
	active := palette.buttonActive
	base.W = 1
	hovered.W = 1
	active.W = 1
	if action.destructive {
		base = vec4(0.88, 0.22, 0.25, 1)
		hovered = popupDialogScaleColor(base, 1.12)
		active = popupDialogScaleColor(base, 0.84)
	}

	imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
	imgui.PushStyleColorVec4(imgui.ColButton, base)
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, hovered)
	imgui.PushStyleColorVec4(imgui.ColButtonActive, active)
	imgui.PushStyleColorVec4(imgui.ColText, palette.text)
	pressed := imgui.ButtonV(label, vec2(width, popupDialogButtonHeight))
	imgui.PopStyleColorV(4)
	imgui.PopStyleVar()
	return pressed
}

func (d *popupDialog) drawOutlinedActionButton(label string, width float32, palette popupDialogPalette) bool {
	base := palette.button
	base.W *= 0.35
	hovered := palette.buttonHovered
	hovered.W *= 0.55
	active := palette.buttonActive
	active.W *= 0.7
	border := palette.border
	border.W = max(border.W, float32(0.65))

	imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 1)
	imgui.PushStyleColorVec4(imgui.ColButton, base)
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, hovered)
	imgui.PushStyleColorVec4(imgui.ColButtonActive, active)
	imgui.PushStyleColorVec4(imgui.ColText, palette.text)
	imgui.PushStyleColorVec4(imgui.ColBorder, border)
	pressed := imgui.ButtonV(label, vec2(width, popupDialogButtonHeight))
	imgui.PopStyleColorV(5)
	imgui.PopStyleVar()
	return pressed
}

func (d *popupDialog) actionWidth(action popupDialogAction) float32 {
	return max(float32(80), imgui.CalcTextSizeV(action.label, false, 0).X+24)
}

func (d *popupDialog) activateActionAt(action popupDialogAction, now time.Time) {
	if d.phase == popupDialogPhaseClosed ||
		d.phase == popupDialogPhaseDisappearing {
		return
	}

	if action.onClick != nil {
		action.onClick()
	}
	if action.keepOpen || action.closeWhen != nil && !action.closeWhen() {
		return
	}

	d.beginDismissAt(now, false)
}

func (d *popupDialog) requestDismissAt(now time.Time) {
	d.beginDismissAt(now, true)
}

func (d *popupDialog) beginDismissAt(now time.Time, notify bool) {
	if d.phase == popupDialogPhaseClosed || d.phase == popupDialogPhaseDisappearing {
		return
	}

	if notify && !d.dismissRequestCalled {
		d.dismissRequestCalled = true
		if d.onDismissRequest != nil {
			d.onDismissRequest()
		}
	}

	if !d.properties.animateTransition {
		d.phase = popupDialogPhaseClosed
		d.opened = false
		return
	}

	progress := d.appearanceProgress(now)
	if progress <= 0 {
		d.phase = popupDialogPhaseClosed
		d.opened = false
		return
	}

	d.dismissStartProgress = progress
	d.transitionStarted = now
	d.phase = popupDialogPhaseDisappearing
}

func (d *popupDialog) updateTransition(now time.Time) {
	switch d.phase {
	case popupDialogPhaseAppearing:
		if !d.properties.animateTransition || now.Sub(d.transitionStarted) >= popupDialogAppearanceDuration {
			d.phase = popupDialogPhaseVisible
		}
	case popupDialogPhaseDisappearing:
		duration := popupDialogDisappearanceDurationFor(d.dismissStartProgress)
		if !d.properties.animateTransition || duration <= 0 || now.Sub(d.transitionStarted) >= duration {
			d.phase = popupDialogPhaseClosed
			d.opened = false
		}
	}
}

func (d *popupDialog) appearanceProgress(now time.Time) float32 {
	switch d.phase {
	case popupDialogPhaseAppearing:
		return popupDialogEaseOut(popupDialogDurationProgress(now.Sub(d.transitionStarted), popupDialogAppearanceDuration))
	case popupDialogPhaseVisible:
		return 1
	case popupDialogPhaseDisappearing:
		duration := popupDialogDisappearanceDurationFor(d.dismissStartProgress)
		if duration <= 0 {
			return 0
		}

		progress := popupDialogEaseOut(popupDialogDurationProgress(now.Sub(d.transitionStarted), duration))
		return (1 - progress) * d.dismissStartProgress
	default:
		return 0
	}
}

func popupDialogWidthBounds(windowWidth float32) (float32, float32) {
	maxWidth := min(popupDialogMaxWidth, max(float32(1), windowWidth-popupDialogWindowMargin*2))
	return min(popupDialogMinWidth, maxWidth), maxWidth
}

func popupDialogWidth(windowWidth float32) float32 {
	minWidth, maxWidth := popupDialogWidthBounds(windowWidth)
	return min(maxWidth, max(minWidth, popupDialogPreferredWidth))
}

func popupDialogUseVerticalActions(available float32, widths []float32, hasLeadingAction bool) bool {
	if len(widths) == 0 {
		return false
	}

	total := float32(0)
	for _, width := range widths {
		total += width
	}
	total += popupDialogActionGap * float32(len(widths)-1)
	if hasLeadingAction && len(widths) > 1 {
		total += 24
	}

	return total > available
}

func shouldDismissPopupDialog(properties popupDialogProperties, input popupDialogDismissInput) bool {
	if input.escapePressed && properties.dismissOnEscape {
		return true
	}

	return properties.dismissOnClickOutside &&
		input.outsideMouseReleased &&
		input.outsideMouseButton == imgui.MouseButtonLeft
}

func popupDialogDurationProgress(elapsed, duration time.Duration) float32 {
	if duration <= 0 || elapsed >= duration {
		return 1
	}
	if elapsed <= 0 {
		return 0
	}

	return float32(float64(elapsed) / float64(duration))
}

func popupDialogEaseOut(progress float32) float32 {
	progress = max(float32(0), min(float32(1), progress))
	return -progress * (progress - 2)
}

func popupDialogContentAlpha(progress float32) float32 {
	progress = max(float32(0), min(float32(1), progress))
	return popupDialogInitialAlpha + (1-popupDialogInitialAlpha)*progress
}

func popupDialogTranslationY(progress float32) float32 {
	progress = max(float32(0), min(float32(1), progress))
	return popupDialogOffsetY * (1 - progress)
}

func popupDialogDisappearanceDurationFor(startProgress float32) time.Duration {
	startProgress = max(float32(0), min(float32(1), startProgress))
	return time.Duration(float64(popupDialogDisappearanceDuration) * float64(startProgress))
}

func currentPopupDialogPalette() popupDialogPalette {
	surface := *imgui.StyleColorVec4(imgui.ColPopupBg)
	surface.W = max(surface.W, float32(0.96))
	contentSurface := *imgui.StyleColorVec4(imgui.ColFrameBg)
	contentSurface.W = max(contentSurface.W, float32(0.8))
	text := *imgui.StyleColorVec4(imgui.ColText)
	text.W = 1
	secondaryText := *imgui.StyleColorVec4(imgui.ColTextDisabled)
	secondaryText.W = 1
	border := *imgui.StyleColorVec4(imgui.ColBorder)
	border.W *= 0.55

	return popupDialogPalette{
		surface:        surface,
		contentSurface: contentSurface,
		text:           text,
		secondaryText:  secondaryText,
		border:         border,
		button:         *imgui.StyleColorVec4(imgui.ColButton),
		buttonHovered:  *imgui.StyleColorVec4(imgui.ColButtonHovered),
		buttonActive:   *imgui.StyleColorVec4(imgui.ColButtonActive),
	}
}

func popupDialogToneAppearance(tone popupDialogTone, neutral imgui.Vec4) (string, imgui.Vec4) {
	switch tone {
	case popupDialogToneInfo:
		return "\uf05a", neutral
	case popupDialogToneWarning:
		return "\uf071", vec4(1, 0.72, 0.22, 1)
	case popupDialogToneDanger:
		return "\uf06a", vec4(0.96, 0.32, 0.35, 1)
	default:
		return "", vec4(0, 0, 0, 0)
	}
}

func popupDialogBlendColor(from, to imgui.Vec4, amount float32) imgui.Vec4 {
	amount = max(float32(0), min(float32(1), amount))
	return imgui.Vec4{
		X: from.X + (to.X-from.X)*amount,
		Y: from.Y + (to.Y-from.Y)*amount,
		Z: from.Z + (to.Z-from.Z)*amount,
		W: from.W + (to.W-from.W)*amount,
	}
}

func popupDialogScaleColor(color imgui.Vec4, scale float32) imgui.Vec4 {
	color.X = min(float32(1), color.X*scale)
	color.Y = min(float32(1), color.Y*scale)
	color.Z = min(float32(1), color.Z*scale)
	return color
}
