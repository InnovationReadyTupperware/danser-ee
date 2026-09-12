//go:build !danser_release

package launcher

import (
	"time"

	"github.com/AllenDang/cimgui-go/imgui"

	appUpdate "github.com/innovationreadytupperware/danser-ee/app/update"
	"github.com/innovationreadytupperware/danser-ee/build"
)

const aboutUpdateSimulationDelay = 2 * time.Second

var aboutSimulateReleaseIdentity bool
var aboutSimulateAutomaticChecksDisabled bool

type aboutUpdateSimulation struct {
	label  string
	result appUpdate.SimulatedResult
}

var aboutUpdateSimulations = []aboutUpdateSimulation{
	{
		label: "Latest release",
		result: appUpdate.SimulatedResult{
			Status: appUpdate.StatusUpToDate,
		},
	},
	{
		label: "Update available",
		result: appUpdate.SimulatedResult{
			Status:        appUpdate.StatusUpdateAvailable,
			LatestVersion: "2.4.0",
			ReleaseURL:    danserEEReleasesURL,
		},
	},
	{
		label: "No published releases",
		result: appUpdate.SimulatedResult{
			Status: appUpdate.StatusNoReleases,
			Error:  "no releases published for this repository",
		},
	},
	{
		label: "Network failure",
		result: appUpdate.SimulatedResult{
			Status: appUpdate.StatusFailed,
			Error:  "simulated network failure",
		},
	},
}

func aboutDeveloperDialogOptions(l *launcher) []popupDialogOption {
	return []popupDialogOption{
		withPopupDialogHeaderAccessory(vec2(popupDialogCloseSize, popupDialogCloseSize), l.drawAboutDeveloperMenu),
	}
}

func (l *launcher) drawAboutDeveloperMenu(palette popupDialogPalette) {
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
	if imgui.ButtonV("\uf013##about-update-simulator", vec2(popupDialogCloseSize, popupDialogCloseSize)) {
		imgui.OpenPopupStr("##about-update-simulator-menu")
	}
	hoveredButton := imgui.IsItemHovered()
	imgui.PopFont()
	imgui.PopStyleColorV(4)
	imgui.PopStyleVarV(2)
	if hoveredButton {
		imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, vec2(8, 8))
		imgui.BeginTooltip()
		imgui.PushFont(Font, 20)
		imgui.TextUnformatted("Developer tools")
		imgui.PopFont()
		imgui.EndTooltip()
		imgui.PopStyleVar()
	}

	if !imgui.BeginPopupV("##about-update-simulator-menu", imgui.WindowFlagsNoSavedSettings) {
		return
	}
	defer imgui.EndPopup()

	imgui.TextDisabled("Simulate update check")
	imgui.Separator()
	if imgui.MenuItemBoolV("Automatic checks disabled", "", aboutSimulateAutomaticChecksDisabled, true) {
		aboutSimulateAutomaticChecksDisabled = !aboutSimulateAutomaticChecksDisabled
	}
	imgui.Separator()
	for _, simulation := range aboutUpdateSimulations {
		if imgui.MenuItemBool(simulation.label) {
			l.startAboutUpdateSimulation(simulation)
		}
	}
	imgui.Separator()
	imgui.TextDisabled("Simulate build identity")
	imgui.Separator()
	if imgui.MenuItemBoolV("Release build 2.3.1", "", aboutSimulateReleaseIdentity, true) {
		aboutSimulateReleaseIdentity = !aboutSimulateReleaseIdentity
	}
	imgui.Separator()
	if imgui.MenuItemBool("Reset") {
		aboutSimulateReleaseIdentity = false
		aboutSimulateAutomaticChecksDisabled = false
		appUpdate.Default().ResetSimulation()
	}
}

func aboutDisplayedBuildIdentity() (version string, releaseStyle bool) {
	if aboutSimulateReleaseIdentity {
		return "2.3.1", true
	}
	if build.IsRelease() {
		return build.Version, true
	}
	return build.Version, false
}

func aboutAutomaticUpdateChecksEnabled() bool {
	if aboutSimulateAutomaticChecksDisabled {
		return false
	}
	return launcherConfig.CheckForUpdates
}

func (l *launcher) startAboutUpdateSimulation(simulation aboutUpdateSimulation) {
	l.runBackgroundTask(func() {
		appUpdate.Default().Simulate(l.runtimeContext, simulation.result, aboutUpdateSimulationDelay)
	})
}
