package launcher

import (
	"fmt"
	"math"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"

	appUpdate "github.com/innovationreadytupperware/danser-ee/app/update"
	"github.com/innovationreadytupperware/danser-ee/framework/platform"
)

const (
	danserEERepositoryURL = "https://github.com/InnovationReadyTupperware/danser-ee"
	danserEEReleasesURL   = danserEERepositoryURL + "/releases"
	danserGoRepositoryURL = "https://github.com/Wieku/danser-go"
	danserEELicenseURL    = danserEERepositoryURL + "/blob/main/LICENSE"
)

func (l *launcher) openAboutDialog() {
	options := []popupDialogOption{
		withPopupDialogPlainBody(),
		withPopupDialogContent(l.drawAboutContent),
	}
	options = append(options, aboutDeveloperDialogOptions(l)...)

	l.openPopup(newPopupDialog(
		"about",
		"About danser-ee",
		options...,
	))
}

func (l *launcher) drawAboutContent() {
	l.drawAboutIdentity()

	dummyExactY(16)
	imgui.Separator()
	dummyExactY(12)
	l.drawAboutUpdateSection()

	dummyExactY(12)
	imgui.Separator()
	dummyExactY(12)
	drawAboutSectionLabel("Links")
	dummyExactY(4)

	drawAboutLinkRow("about-source", "\uf121", "Source code", danserEERepositoryURL)
	drawAboutLinkRow("about-releases", "\uf019", "Releases & downloads", danserEEReleasesURL)
	drawAboutLinkRow("about-upstream", "\uf126", "Original danser-go", danserGoRepositoryURL)
	drawAboutLinkRow("about-license", "\uf15c", "GPL-3.0 license", danserEELicenseURL)
}

func drawAboutSectionLabel(label string) {
	imgui.PushStyleColorVec4(imgui.ColText, *imgui.StyleColorVec4(imgui.ColTextDisabled))
	imgui.PushFont(Font, 14)
	imgui.TextUnformatted(label)
	imgui.PopFont()
	imgui.PopStyleColor()
}

func (l *launcher) drawAboutIdentity() {
	if imgui.BeginTableV("about-identity", 2, imgui.TableFlagsSizingStretchProp, vec2(-1, 0), -1) {
		imgui.TableSetupColumnV("logo", imgui.TableColumnFlagsWidthFixed, 88, 0)
		imgui.TableSetupColumnV("details", imgui.TableColumnFlagsWidthStretch, 0, 1)

		imgui.TableNextColumn()
		if l.coin != nil && l.coin.Texture.Texture != nil {
			texRef := imgui.NewTextureRefTextureID(imgui.TextureID(l.coin.Texture.Texture.GetID()))
			imgui.Image(*texRef, vec2(76, 76))
			texRef.Destroy()
		}

		imgui.TableNextColumn()
		version, releaseStyle := aboutDisplayedBuildIdentity()
		titleStart := imgui.CursorScreenPos()
		imgui.PushFont(Font, 22)
		titleSize := imgui.CalcTextSize(productDisplayName)
		imgui.TextUnformatted(productDisplayName)
		imgui.PopFont()
		if releaseStyle {
			const versionFontSize = float32(18)
			displayVersion := "v" + version

			imgui.PushFont(Font, versionFontSize)
			versionSize := imgui.CalcTextSize(displayVersion)
			imgui.PopFont()

			versionPosition := titleStart.Add(vec2(
				titleSize.X+4,
				(titleSize.Y-versionSize.Y)/2+1,
			))
			imgui.WindowDrawList().AddTextFontPtr(
				Font,
				versionFontSize,
				versionPosition,
				packColor(*imgui.StyleColorVec4(imgui.ColTextDisabled)),
				displayVersion,
			)
		} else {
			imgui.PushStyleColorVec4(imgui.ColText, *imgui.StyleColorVec4(imgui.ColTextDisabled))
			imgui.PushFont(Font, 15)
			imgui.TextUnformatted(version + " · Development build")
			imgui.PopFont()
			imgui.PopStyleColor()
		}

		imgui.PushFont(Font, 18)
		imgui.PushTextWrapPos()
		imgui.TextUnformatted("An enterprise-grade danser-go fork for visualizing osu!standard maps and replays.")
		imgui.PopTextWrapPos()
		imgui.PopFont()

		imgui.EndTable()
	}
}

func drawAboutLinkRow(id, icon, label, url string) {
	const (
		rowHeight   = float32(40)
		edgeInset   = float32(12)
		iconSize    = float32(14)
		labelSize   = float32(17)
		chevronSize = float32(11)
	)

	start := imgui.CursorScreenPos()
	width := imgui.ContentRegionAvail().X
	hovered := *imgui.StyleColorVec4(imgui.ColButtonHovered)
	hovered.W *= 0.45
	active := *imgui.StyleColorVec4(imgui.ColButtonActive)
	active.W *= 0.55

	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, 8)
	imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
	imgui.PushStyleColorVec4(imgui.ColButton, vec4(0, 0, 0, 0))
	imgui.PushStyleColorVec4(imgui.ColButtonHovered, hovered)
	imgui.PushStyleColorVec4(imgui.ColButtonActive, active)
	pressed := imgui.ButtonV("##"+id, vec2(width, rowHeight))
	imgui.PopStyleColorV(3)
	imgui.PopStyleVarV(2)

	drawList := imgui.WindowDrawList()
	textColor := packColor(*imgui.StyleColorVec4(imgui.ColText))
	secondaryColor := packColor(*imgui.StyleColorVec4(imgui.ColTextDisabled))
	imgui.PushFont(FontAw, chevronSize)
	chevronWidth := imgui.CalcTextSize("\uf054").X
	imgui.PopFont()

	drawList.AddTextFontPtr(FontAw, iconSize, start.Add(vec2(edgeInset, (rowHeight-iconSize)/2)), textColor, icon)
	drawList.AddTextFontPtr(Font, labelSize, start.Add(vec2(36, (rowHeight-labelSize)/2)), textColor, label)
	drawList.AddTextFontPtr(FontAw, chevronSize, start.Add(vec2(width-edgeInset-chevronWidth, (rowHeight-chevronSize)/2)), secondaryColor, "\uf054")

	if pressed {
		platform.OpenURL(url)
	}
}

type aboutUpdateAction uint8

const (
	aboutUpdateActionNone aboutUpdateAction = iota
	aboutUpdateActionCheck
	aboutUpdateActionRelease
)

type aboutUpdatePresentation struct {
	icon       string
	loading    bool
	title      string
	secondary  string
	action     aboutUpdateAction
	actionText string
}

func (l *launcher) drawAboutUpdateSection() {
	snapshot := appUpdate.Default().Snapshot()
	presentation := aboutUpdatePresentationFor(snapshot, time.Now(), aboutAutomaticUpdateChecksEnabled())
	drawAboutUpdateHeader(presentation.secondary)
	dummyExactY(4)
	l.drawAboutUpdateRow(snapshot, presentation)
}

func drawAboutUpdateHeader(secondary string) {
	const metadataFontSize = float32(14)

	imgui.PushFont(Font, metadataFontSize)
	metadataWidth := imgui.CalcTextSize(secondary).X
	imgui.PopFont()

	if !imgui.BeginTableV("about-update-header", 2, imgui.TableFlagsSizingStretchProp, vec2(-1, 0), -1) {
		return
	}
	defer imgui.EndTable()

	imgui.TableSetupColumnV("label", imgui.TableColumnFlagsWidthStretch, 0, 0)
	imgui.TableSetupColumnV("metadata", imgui.TableColumnFlagsWidthFixed, metadataWidth, 1)

	imgui.TableNextColumn()
	drawAboutSectionLabel("Updates")

	imgui.TableNextColumn()
	imgui.PushStyleColorVec4(imgui.ColText, *imgui.StyleColorVec4(imgui.ColTextDisabled))
	imgui.PushFont(Font, metadataFontSize)
	imgui.TextUnformatted(secondary)
	imgui.PopFont()
	imgui.PopStyleColor()
}

func (l *launcher) drawAboutUpdateRow(snapshot appUpdate.Snapshot, presentation aboutUpdatePresentation) {
	const (
		actionWidth = float32(112)
		rowHeight   = float32(40)
	)

	if !imgui.BeginTableV("about-update-row", 2, imgui.TableFlagsSizingStretchProp, vec2(-1, rowHeight), -1) {
		return
	}
	defer imgui.EndTable()

	imgui.TableSetupColumnV("status", imgui.TableColumnFlagsWidthStretch, 0, 0)
	imgui.TableSetupColumnV("action", imgui.TableColumnFlagsWidthFixed, actionWidth, 1)

	imgui.TableNextColumn()
	imgui.AlignTextToFramePadding()
	l.drawAboutUpdateStatus(presentation)

	imgui.TableNextColumn()
	if presentation.action == aboutUpdateActionNone {
		return
	}
	dummyExactY(4)
	imgui.PushFont(Font, 20)
	pressed := imgui.ButtonV(presentation.actionText, vec2(-1, 32))
	imgui.PopFont()
	if !pressed {
		return
	}

	if presentation.action == aboutUpdateActionRelease && snapshot.ReleaseURL != "" {
		platform.OpenURL(snapshot.ReleaseURL)
		return
	}

	l.startAboutUpdateCheck()
}

func (l *launcher) drawAboutUpdateStatus(presentation aboutUpdatePresentation) {
	if presentation.loading {
		drawAboutLoadingIndicator()
	} else {
		imgui.PushStyleColorVec4(imgui.ColText, *imgui.StyleColorVec4(imgui.ColTextDisabled))
		imgui.PushFont(FontAw, 15)
		imgui.TextUnformatted(presentation.icon)
		imgui.PopFont()
		imgui.PopStyleColor()
	}

	imgui.SameLineV(0, 8)
	imgui.PushFont(Font, 18)
	imgui.TextUnformatted(presentation.title)
	imgui.PopFont()
}

func drawAboutLoadingIndicator() {
	const (
		indicatorSize = float32(16)
		radius        = float32(5)
		thickness     = float32(2)
	)

	start := imgui.CursorScreenPos()
	lineHeight := imgui.TextLineHeight()
	imgui.Dummy(vec2(indicatorSize, lineHeight))

	center := start.Add(vec2(indicatorSize/2, lineHeight/2))
	rotation := float32(math.Mod(imgui.Time()*4.5, 2*math.Pi))
	drawList := imgui.WindowDrawList()
	drawList.PathArcToV(center, radius, rotation, rotation+1.5*math.Pi, 20)
	drawList.PathStrokeV(
		packColor(*imgui.StyleColorVec4(imgui.ColTextDisabled)),
		thickness,
		imgui.DrawFlagsNone,
	)
}

func aboutUpdatePresentationFor(snapshot appUpdate.Snapshot, now time.Time, automaticChecksEnabled bool) aboutUpdatePresentation {
	if snapshot.Checking {
		secondary := aboutLastCheckedText(snapshot.LastAttempt, now)
		if !automaticChecksEnabled {
			secondary = "Automatic update checks are disabled"
		}
		return aboutUpdatePresentation{
			loading:   true,
			title:     "Checking for updates…",
			secondary: secondary,
		}
	}

	displaySecondary := func(fallback string) string {
		if !automaticChecksEnabled {
			return "Automatic update checks are disabled"
		}
		return fallback
	}

	switch snapshot.Status {
	case appUpdate.StatusDisabled:
		return aboutUpdatePresentation{
			icon:      "\uf05a",
			title:     "Development build",
			secondary: "Automatic update checks are disabled",
		}
	case appUpdate.StatusUpToDate:
		return aboutUpdatePresentation{
			icon:       "\uf058",
			title:      "Up to date",
			secondary:  displaySecondary(aboutLastCheckedText(snapshot.LastAttempt, now)),
			action:     aboutUpdateActionCheck,
			actionText: "Check now",
		}
	case appUpdate.StatusUpdateAvailable:
		title := "Update available"
		if snapshot.LatestVersion != "" {
			title = fmt.Sprintf("%s is available", snapshot.LatestVersion)
		}
		action := aboutUpdateActionCheck
		actionText := "Check now"
		if snapshot.ReleaseURL != "" {
			action = aboutUpdateActionRelease
			actionText = "View release"
		}
		return aboutUpdatePresentation{
			icon:       "\uf019",
			title:      title,
			secondary:  displaySecondary(aboutLastCheckedText(snapshot.LastAttempt, now)),
			action:     action,
			actionText: actionText,
		}
	case appUpdate.StatusNoReleases:
		return aboutUpdatePresentation{
			icon:       "\uf05a",
			title:      "No published releases",
			secondary:  displaySecondary(aboutLastCheckedText(snapshot.LastAttempt, now)),
			action:     aboutUpdateActionCheck,
			actionText: "Try again",
		}
	case appUpdate.StatusFailed:
		secondary := "No successful checks yet"
		if !snapshot.LastSuccessfulCheck.IsZero() {
			secondary = "Last successful check " + aboutRelativeTime(now, snapshot.LastSuccessfulCheck)
		} else if !snapshot.LastAttempt.IsZero() {
			secondary = aboutLastCheckedText(snapshot.LastAttempt, now)
		}
		return aboutUpdatePresentation{
			icon:       "\uf06a",
			title:      "Couldn't check for updates",
			secondary:  displaySecondary(secondary),
			action:     aboutUpdateActionCheck,
			actionText: "Try again",
		}
	default:
		return aboutUpdatePresentation{
			icon:       "\uf05a",
			title:      "Not checked yet",
			secondary:  displaySecondary("No previous checks"),
			action:     aboutUpdateActionCheck,
			actionText: "Check now",
		}
	}
}

func aboutLastCheckedText(checked, now time.Time) string {
	if checked.IsZero() {
		return "No previous checks"
	}
	return "Last checked " + aboutRelativeTime(now, checked)
}

func aboutRelativeTime(now, then time.Time) string {
	if then.IsZero() || !now.After(then) {
		return "just now"
	}

	elapsed := now.Sub(then)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < 2*time.Minute:
		return "1 minute ago"
	case elapsed < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(elapsed/time.Minute))
	case elapsed < 2*time.Hour:
		return "1 hour ago"
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(elapsed/time.Hour))
	case elapsed < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%d days ago", int(elapsed/(24*time.Hour)))
	}
}

func (l *launcher) startAboutUpdateCheck() {
	service := appUpdate.Default()
	snapshot := service.Snapshot()
	if snapshot.Checking || snapshot.Status == appUpdate.StatusDisabled {
		return
	}

	l.runBackgroundTask(func() {
		service.Check(l.runtimeContext, appUpdate.Manual)
	})
}
