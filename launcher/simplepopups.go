package launcher

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/innovationreadytupperware/danser-ee/build"
	"github.com/innovationreadytupperware/danser-ee/framework/graphics/texture"
	"github.com/innovationreadytupperware/danser-ee/framework/platform"
)

func drawSpeedMenu(bld *builder) {
	sliderFloatResetStep("Speed", &bld.speed, 0.1, 3, 0.05, "%.2f")
	imgui.Spacing()

	sliderFloatResetStep("Pitch", &bld.pitch, 0.1, 3, 0.05, "%.2f")
	imgui.Spacing()
}

func drawCDMenu(bld *builder) {
	if imgui.BeginTable("dfa", 2) {
		imgui.TableNextColumn()

		imgui.TextUnformatted("Mirrored cursors:")

		imgui.TableNextColumn()

		imgui.SetNextItemWidth(imgui.TextLineHeight() * 4.5)

		if imgui.InputIntV("##mirrors", &bld.mirrors, 1, 1, 0) {
			if bld.mirrors < 1 {
				bld.mirrors = 1
			}
		}

		imgui.TableNextColumn()

		tagLabel := "Tag cursors:"
		if launcherConfig.CurrentMode == SoloKnockout {
			tagLabel = "Danser participants:"
		}

		imgui.TextUnformatted(tagLabel)

		imgui.TableNextColumn()

		imgui.SetNextItemWidth(imgui.TextLineHeight() * 4.5)

		if imgui.InputIntV("##tags", &bld.tags, 1, 1, 0) {
			if bld.tags < 1 {
				bld.tags = 1
			}
		}

		imgui.EndTable()
	}
}

type outputDialogCopy struct {
	title           string
	supportingText  string
	outputNameLabel string
}

type screenshotTimeFieldState struct {
	text             string
	validationError  string
	validationActive bool
	limitInitialized bool
	limitAvailable   bool
	limit            float64
	focusRequested   bool
}

func outputDialogCopyFor(mode PMode) outputDialogCopy {
	switch mode {
	case Record:
		return outputDialogCopy{
			title:           "Set recording output",
			supportingText:  "Choose the video file name.",
			outputNameLabel: "Video file name",
		}
	case Screenshot:
		return outputDialogCopy{
			title:           "Set screenshot output",
			supportingText:  "Choose the screenshot file name and capture time.",
			outputNameLabel: "Screenshot file name",
		}
	default:
		return outputDialogCopy{
			title:           "Set output",
			supportingText:  "Choose the output file name.",
			outputNameLabel: "File name",
		}
	}
}

func drawRecordMenu(
	bld *builder,
	sourceMode Mode,
	mode PMode,
	dialogCopy outputDialogCopy,
	screenshotTimeState *screenshotTimeFieldState,
	focusOutputName bool,
) {
	if imgui.BeginTable("rfa", 2) {
		imgui.TableSetupColumnV("c1rfa", imgui.TableColumnFlagsWidthFixed, 0, imgui.ID(0))
		imgui.TableSetupColumnV("c2rfa", imgui.TableColumnFlagsWidthFixed, imgui.TextLineHeight()*7, imgui.ID(1))

		imgui.TableNextColumn()

		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted(dialogCopy.outputNameLabel)

		imgui.TableNextColumn()

		imgui.SetNextItemWidth(-1)
		flags := imgui.InputTextFlagsCallbackCharFilter
		if focusOutputName {
			imgui.SetKeyboardFocusHere()
			flags |= imgui.InputTextFlagsAutoSelectAll
		}

		inputTextV("##oname", &bld.outputName, flags, imguiPathFilter)

		if mode == Screenshot {
			imgui.TableNextColumn()

			imgui.AlignTextToFramePadding()
			imgui.TextUnformatted("Screenshot time")

			imgui.TableNextColumn()

			if imgui.BeginTableV("rrfa", 2, 0, vec2(-1, 0), -1) {
				imgui.TableSetupColumnV("c1rrfa", imgui.TableColumnFlagsWidthStretch, 0, imgui.ID(0))
				imgui.TableSetupColumnV("c2rrfa", imgui.TableColumnFlagsWidthFixed, imgui.CalcTextSizeV("s", false, 0).X+imgui.CurrentStyle().CellPadding().X*2, imgui.ID(1))

				imgui.TableNextColumn()

				imgui.SetNextItemWidth(-1)

				if screenshotTimeState == nil {
					panic("screenshot time state is required in screenshot mode")
				}

				maxTime, available := screenshotTimeLimit(bld, sourceMode)
				screenshotTimeState.syncLimit(maxTime, available)
				if !available {
					imgui.BeginDisabled()
				}

				if screenshotTimeState.validationError != "" {
					_, errorColor := popupDialogToneAppearance(
						popupDialogToneDanger,
						*imgui.StyleColorVec4(imgui.ColBorder),
					)
					imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 1)
					imgui.PushStyleColorVec4(imgui.ColBorder, errorColor)
				}

				flags := imgui.InputTextFlagsEnterReturnsTrue
				if available && screenshotTimeState.focusRequested {
					imgui.SetKeyboardFocusHere()
					flags |= imgui.InputTextFlagsAutoSelectAll
					screenshotTimeState.focusRequested = false
				}

				hint := ""
				if available {
					hint = screenshotTimePlaceholder(maxTime)
				}

				enterPressed := imgui.InputTextWithHint(
					"##sstime",
					hint,
					&screenshotTimeState.text,
					flags,
					nil,
				)
				edited := imgui.IsItemEdited()
				deactivatedAfterEdit := imgui.IsItemDeactivatedAfterEdit()

				if screenshotTimeState.validationError != "" {
					imgui.PopStyleColor()
					imgui.PopStyleVar()
				}

				if !available {
					imgui.EndDisabled()
					if imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenDisabled) {
						imgui.SetTooltip(screenshotTimeUnavailableTooltip(sourceMode))
					}
				} else {
					if edited && screenshotTimeState.validationActive {
						screenshotTimeState.revalidate(maxTime)
					}
					if enterPressed || deactivatedAfterEdit {
						if parsed, valid := screenshotTimeState.commit(maxTime); valid {
							bld.ssTime = parsed
						}
					}
				}

				imgui.TableNextColumn()

				imgui.AlignTextToFramePadding()
				imgui.TextUnformatted("s")

				imgui.EndTable()
			}

		}

		imgui.EndTable()
	}
}

func normalizeScreenshotTimeInput(text string) string {
	if strings.HasPrefix(text, ".") {
		return "0" + text
	}

	return text
}

func formatScreenshotTime(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', 3, 32)
}

func formatScreenshotTimeLimit(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func screenshotTimePlaceholder(maxValue float64) string {
	if maxValue <= 0 {
		return ""
	}

	return "0-" + formatScreenshotTimeLimit(maxValue)
}

func screenshotTimeUnavailableTooltip(mode Mode) string {
	switch mode {
	case Replay:
		return "Select a replay to set the screenshot time."
	case Knockout:
		return "Select at least one replay to set the screenshot time."
	default:
		return "Select a map to set the screenshot time."
	}
}

func newScreenshotTimeFieldState(value float32) *screenshotTimeFieldState {
	return &screenshotTimeFieldState{text: formatScreenshotTime(value)}
}

func (state *screenshotTimeFieldState) validate(maxValue float64) (float32, bool) {
	if state == nil {
		return 0, false
	}

	state.validationActive = true
	parsed, validationError := validateScreenshotTimeInput(state.text, maxValue)
	state.validationError = validationError
	if validationError != "" {
		return 0, false
	}

	return parsed, true
}

func (state *screenshotTimeFieldState) revalidate(maxValue float64) {
	if state == nil || !state.validationActive {
		return
	}

	_, state.validationError = validateScreenshotTimeInput(state.text, maxValue)
}

func (state *screenshotTimeFieldState) commit(maxValue float64) (float32, bool) {
	parsed, valid := state.validate(maxValue)
	if !valid {
		return 0, false
	}

	parsed = float32(math.Round(float64(parsed)*1000) / 1000)
	state.text = formatScreenshotTime(parsed)
	return parsed, true
}

func (state *screenshotTimeFieldState) syncLimit(maxValue float64, available bool) {
	if state == nil {
		return
	}

	changed := !state.limitInitialized ||
		state.limitAvailable != available ||
		available && state.limit != maxValue

	state.limitInitialized = true
	state.limitAvailable = available
	state.limit = maxValue
	if !changed {
		return
	}

	if !available {
		state.validationError = ""
		return
	}

	_, validationError := validateScreenshotTimeInput(state.text, maxValue)
	state.validationError = validationError
	if validationError != "" {
		state.validationActive = true
	}
}

func screenshotTimeLimit(bld *builder, mode Mode) (float64, bool) {
	if bld == nil || bld.currentMap == nil || bld.currentMap.Length <= 0 {
		return 0, false
	}

	switch mode {
	case Replay:
		if bld.currentReplay == nil {
			return 0, false
		}
	case Knockout:
		if bld.numKnockoutReplays() == 0 {
			return 0, false
		}
	}

	return float64(bld.currentMap.Length) / 1000, true
}

func validateScreenshotTimeInput(text string, maxValue float64) (float32, string) {
	text = normalizeScreenshotTimeInput(text)
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, "Enter a time in seconds"
	}

	if validationError := validateScreenshotTimeValue(parsed, maxValue); validationError != "" {
		return 0, validationError
	}

	return float32(parsed), ""
}

func validateScreenshotTimeValue(value float64, maxValue float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "Enter a time in seconds"
	}

	if maxValue <= 0 || value < 0 || value > maxValue {
		return fmt.Sprintf(
			"Enter a time between 0 and %s seconds",
			formatScreenshotTimeLimit(maxValue),
		)
	}

	return ""
}

func screenshotTimeNeedsConfiguration(bld *builder, mode Mode) bool {
	maxTime, available := screenshotTimeLimit(bld, mode)
	if !available {
		return false
	}

	value := bld.ssTime
	return math.IsNaN(float64(value)) ||
		math.IsInf(float64(value), 0) ||
		value < 0 ||
		value > float32(maxTime)
}

func drawAbout(dTex texture.Texture) {
	centerTable("about1", -1, func() {
		texRef := imgui.NewTextureRefTextureID(imgui.TextureID(dTex.GetID()))
		defer texRef.Destroy()

		imgui.Image(*texRef, vec2(100, 100))
	})

	centerTable("about2", -1, func() {
		imgui.TextUnformatted("danser-ee " + build.Version)
	})

	centerTable("about3", -1, func() {
		if imgui.Button("Check for updates") {
			checkForUpdates(true)
		}
	})

	imgui.Dummy(vec2(1, imgui.FrameHeight()))

	centerTable("about4.1", -1, func() {
		imgui.TextUnformatted("Advanced visualisation multi-tool")
	})

	centerTable("about4.2", -1, func() {
		imgui.TextUnformatted("for osu!")
	})

	imgui.Dummy(vec2(1, imgui.FrameHeight()))

	if imgui.BeginTableV("about5", 3, imgui.TableFlagsSizingStretchSame, vec2(-1, 0), -1) {
		imgui.TableNextColumn()

		centerTable("aboutgithub", -1, func() {
			if imgui.Button("GitHub") {
				platform.OpenURL("https://github.com/InnovationReadyTupperware/danser-ee")
			}
		})

		imgui.TableNextColumn()

		centerTable("aboutdonate", -1, func() {
			if imgui.Button("Donate") {
				platform.OpenURL("https://wieku.me/donate")
			}
		})

		imgui.TableNextColumn()

		centerTable("aboutdiscord", -1, func() {
			if imgui.Button("Discord") {
				platform.OpenURL("https://wieku.me/lair")
			}
		})

		imgui.EndTable()
	}
}

func drawLauncherConfig() {
	imgui.PushStyleVarVec2(imgui.StyleVarCellPadding, vec2(imgui.CurrentStyle().CellPadding().X, 10))

	checkboxOption("Check for updates on startup", &launcherConfig.CheckForUpdates)

	checkboxOption("Load latest replay on startup", &launcherConfig.LoadLatestReplay)

	checkboxOption("Skip library rescan.\nFinds new sets only, won't detect\ndeleted/updated maps!", &launcherConfig.SkipMapUpdate)

	checkboxOption("Load changes in Songs folder automatically", &launcherConfig.AutoRefreshDB)

	checkboxOption("Show JSON paths in config editor", &launcherConfig.ShowJSONPaths)

	checkboxOption("Show exported videos/images\nin explorer", &launcherConfig.ShowFileAfter)

	checkboxOption("Preview selected maps", &launcherConfig.PreviewSelected)

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Preview volume")

	volume := int32(launcherConfig.PreviewVolume * 100)

	imgui.PushFont(Font, 16)

	imgui.SetNextItemWidth(-1)

	if sliderIntSlide("##previewvolume", &volume, 0, 100, "%d%%", 0) {
		launcherConfig.PreviewVolume = float64(volume) / 100
	}

	imgui.PopFont()

	imgui.PopStyleVar()
}
