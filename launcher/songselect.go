package launcher

import (
	"cmp"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"slices"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"

	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/database"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/bass"
	"github.com/wieku/danser-go/framework/graphics/texture"
	"github.com/wieku/danser-go/framework/math/animation"
	"github.com/wieku/danser-go/framework/math/mutils"
	"github.com/wieku/danser-go/framework/platform"
	"github.com/wieku/danser-go/framework/qpc"
	"github.com/wieku/danser-go/framework/util"
)

type SortBy int

const (
	Title = SortBy(iota)
	Artist
	Creator
	DateAdded
	Difficulty
)

var sortMethods = []SortBy{Title, Artist, Creator, DateAdded, Difficulty}

func (s SortBy) String() string {
	switch s {
	case Title:
		return "Title"
	case Artist:
		return "Artist"
	case Creator:
		return "Creator"
	case DateAdded:
		return "Date Added"
	case Difficulty:
		return "Difficulty"
	}

	return ""
}

type searchEntry struct {
	entry           *database.BeatmapEntry
	searchKey       string
	titleKey        string
	artistKey       string
	creatorKey      string
	directoryKey    string
	mapKey          string
	md5             string
	artistCreator   string
	difficultyLabel string
	groupIndex      int
}

func newSearchEntry(entry *database.BeatmapEntry) *searchEntry {
	if entry == nil {
		return nil
	}

	return &searchEntry{
		entry:           entry,
		searchKey:       entry.SearchKey(),
		titleKey:        strings.ToLower(entry.Name),
		artistKey:       strings.ToLower(entry.Artist),
		creatorKey:      strings.ToLower(entry.Creator),
		directoryKey:    normalizeSongDirectory(entry.Dir),
		mapKey:          normalizeMapKey(entry.MapKey()),
		md5:             strings.ToLower(entry.MD5),
		artistCreator:   entry.Artist + " // " + entry.Creator,
		difficultyLabel: ">   " + entry.Difficulty,
		groupIndex:      -1,
	}
}

func normalizeSongDirectory(directory string) string {
	return strings.ToLower(filepath.ToSlash(directory))
}

func normalizeMapKey(mapKey string) string {
	return strings.ToLower(filepath.ToSlash(mapKey))
}

type searchEntries []*searchEntry

func (e searchEntries) String(i int) string {
	return e[i].searchKey
}

func (e searchEntries) Len() int {
	return len(e)
}

type songSet struct {
	directory     string
	title         string
	artistCreator string
	groupIndex    int
	entryStart    int
	entryEnd      int
	matchCount    int
	hovered       bool
}

// searchResults keeps all matched entries in one flat array. A set stores a
// half-open range into that array, avoiding one heap allocation and one pointer
// indirection for every set in a broad query.
type searchResults struct {
	sets    []songSet
	entries []*searchEntry
}

func (r searchResults) entriesForSet(index int) []*searchEntry {
	if index < 0 || index >= len(r.sets) {
		return nil
	}

	set := r.sets[index]
	if set.entryStart < 0 || set.entryStart > set.entryEnd || set.entryEnd > len(r.entries) {
		return nil
	}

	return r.entries[set.entryStart:set.entryEnd]
}

func (r searchResults) findSetByGroup(groupIndex int) (int, bool) {
	left, right := 0, len(r.sets)
	for left < right {
		middle := left + (right-left)/2
		if r.sets[middle].groupIndex < groupIndex {
			left = middle + 1
		} else {
			right = middle
		}
	}

	return left, left < len(r.sets) && r.sets[left].groupIndex == groupIndex
}

type searchMatch struct {
	entry    *searchEntry
	setIndex int
}

type songSelectPopup struct {
	*popup

	bld      *builder
	beatmaps searchEntries

	searchResults    searchResults
	searchScratch    []searchMatch
	groupScratch     []int
	groupByDirectory map[string]int
	groupCount       int
	searchStr        string

	prevMap       *beatmap.BeatMap
	prevEntry     *database.BeatmapEntry
	previewMapKey string
	PreviewedSong *bass.TrackBass
	volume        *animation.Glider
	stopTime      float64
	thumbTex      *texture.TextureSingle
	texRef        *imgui.TextureRef
	lastThumbPath string
	focusTheMap   bool

	comboOpened bool
	scrolling   bool

	materialize          func(*database.BeatmapEntry) (*beatmap.BeatMap, error)
	catalog              *database.CatalogSnapshot
	catalogDirty         bool
	layout               variableHeightLayout
	layoutWidth          float32
	layoutReady          bool
	lastScrollY          float32
	tooltipDisabledUntil float64
}

func newSongSelectPopup(bld *builder, catalog *database.CatalogSnapshot, materialize func(*database.BeatmapEntry) (*beatmap.BeatMap, error)) *songSelectPopup {
	mP := &songSelectPopup{
		popup:       newPopup("Song select", popBig),
		bld:         bld,
		volume:      animation.NewGlider(0),
		materialize: materialize,
	}

	mP.internalDraw = mP.drawSongSelect
	mP.setCloseListener(mP.releaseThumbnail)

	mP.setCatalog(catalog)

	return mP
}

func (m *songSelectPopup) setCatalog(catalog *database.CatalogSnapshot) {
	m.catalog = catalog
	m.catalogDirty = false

	beatmaps := make(searchEntries, 0)
	if catalog != nil {
		beatmaps = make(searchEntries, 0, catalog.Len())
		catalog.ForEach(func(entry *database.BeatmapEntry) bool {
			if searchEntry := newSearchEntry(entry); searchEntry != nil {
				beatmaps = append(beatmaps, searchEntry)
			}
			return true
		})
	}

	m.beatmaps = beatmaps
	m.groupByDirectory = sortMaps(m.beatmaps, launcherConfig.SortMapsBy)
	m.groupCount = len(m.groupByDirectory)
	m.search()
	m.focusTheMap = true
}

func (m *songSelectPopup) updateCatalog(catalog *database.CatalogSnapshot) {
	m.catalog = catalog
	// Reconciliation publishes from a background worker. Rebuilding and sorting
	// a 140k-entry view here would put that worker's main-thread callback on the
	// input path, so defer the rebuild until the selector is opened again.
	m.catalogDirty = true
}

func (m *songSelectPopup) refreshCatalog() {
	if m.catalogDirty {
		m.setCatalog(m.catalog)
	}
}

func (m *songSelectPopup) update() {
	cT := qpc.GetMilliTimeF()

	m.volume.Update(cT)
	if m.PreviewedSong != nil {
		m.PreviewedSong.SetVolumeRelative(m.volume.GetValue() * launcherConfig.PreviewVolume)

		if cT >= m.stopTime {
			m.stopPreview()
		}
	}
}

func (m *songSelectPopup) drawSongSelect() {
	imgui.PushFont(Font, 32)

	imgui.SetNextItemWidth(-1)
	if searchBox("##searchpath", &m.searchStr) {
		m.search()
		m.focusTheMap = true
	}

	if !m.scrolling && !m.comboOpened && !imgui.IsAnyItemActive() && !imgui.IsMouseClickedBool(0) {
		imgui.SetKeyboardFocusHereV(-1)
	}

	imgui.PopFont()

	imgui.PushFont(Font, 20)

	if imgui.BeginTableV("sortrandom", 2, 0, vec2(-1, 0), -1) {
		imgui.TableSetupColumnV("##sortrandom1", imgui.TableColumnFlagsWidthStretch, 0, imgui.ID(0))
		imgui.TableSetupColumnV("##sortrandom2", imgui.TableColumnFlagsWidthFixed, 0, imgui.ID(1))

		imgui.TableNextColumn()

		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted("Sort by:")

		imgui.SameLine()

		m.comboOpened = false

		imgui.SetNextItemWidth(150)

		if imgui.BeginCombo("##sortcombo", launcherConfig.SortMapsBy.String()) {
			m.comboOpened = true

			for _, s := range sortMethods {
				if imgui.SelectableBoolV(s.String(), s == launcherConfig.SortMapsBy, 0, vzero()) && s != launcherConfig.SortMapsBy {
					launcherConfig.SortMapsBy = s
					m.groupByDirectory = sortMaps(m.beatmaps, launcherConfig.SortMapsBy)
					m.groupCount = len(m.groupByDirectory)
					m.search()
					m.focusTheMap = true
					saveLauncherConfig()
				}
			}

			imgui.EndCombo()
		}

		imgui.SameLine()

		imgui.PushFont(FontAw, 20)

		sDir := "\uF882"
		if launcherConfig.SortAscending {
			sDir = "\uF15D"
		}

		if imgui.Button(sDir) {
			launcherConfig.SortAscending = !launcherConfig.SortAscending
			m.groupByDirectory = sortMaps(m.beatmaps, launcherConfig.SortMapsBy)
			m.groupCount = len(m.groupByDirectory)
			m.search()
			m.focusTheMap = true
			saveLauncherConfig()
		}

		imgui.PopFont()

		imgui.TableNextColumn()

		if imgui.Button("Random") {
			m.selectRandom()
		}

		imgui.EndTable()
	}

	imgui.PopFont()

	csPos := imgui.CursorScreenPos()

	imgui.BeginChildStr("##bsets")

	previousScrollY := m.lastScrollY
	m.scrolling = handleDragScroll()

	imgui.PushStyleVarVec2(imgui.StyleVarFramePadding, vec2(5, 0))
	listStartY := imgui.CursorPos().Y
	listWidth := imgui.ContentRegionAvail().X
	if !m.layoutReady || math.Abs(float64(listWidth-m.layoutWidth)) > 1 {
		m.rebuildLayout(listWidth)
	}

	if m.focusTheMap {
		if m.bld.currentMap != nil {
			currentDirectory := normalizeSongDirectory(m.bld.currentMap.Dir)
			if groupIndex, ok := m.groupByDirectory[currentDirectory]; ok {
				resultIndex, found := m.searchResults.findSetByGroup(groupIndex)
				if found {
					currentMap := newMapIdentity(m.bld.currentMap)
					for _, entry := range m.searchResults.entriesForSet(resultIndex) {
						if entry.matches(currentMap) {
							imgui.SetScrollYFloat(m.layout.top(resultIndex))
							break
						}
					}
				}
			}
		}
		m.focusTheMap = false
	}

	scrollY := imgui.ScrollY()
	now := qpc.GetMilliTimeF()
	if m.scrolling || math.Abs(float64(scrollY-previousScrollY)) > 0.01 {
		// Dear ImGui's item-tooltip delay is useful for ordinary hover, but it
		// does not know that this list's thumbnail load touches the disk and
		// uploads a texture. Keep that work out of the scroll path explicitly.
		m.tooltipDisabledUntil = now + 150
	}
	tooltipAllowed := now >= m.tooltipDisabledUntil
	currentMap := newMapIdentity(m.bld.currentMap)

	startIndex, endIndex := m.layout.visibleRange(scrollY, imgui.ContentRegionAvail().Y)
	var scrollCorrection float32

	if imgui.BeginTableV("bsetstab", 1, imgui.TableFlagsRowBg|imgui.TableFlagsPadOuterX|imgui.TableFlagsBordersH, vec2(-1, 0), -1) {
		imgui.TableSetBgColor(imgui.TableBgTargetRowBg1, packColor(vec4(0.5, 0.5, 0.5, 1)))

		if startIndex > 0 {
			imgui.TableNextColumn()
			currentY := imgui.CursorPos().Y - listStartY
			dummyExactY(max(0, m.layout.top(startIndex)-currentY))
		}

		previewKey := m.previewMapKey

		for i := startIndex; i < endIndex; i++ {
			b := &m.searchResults.sets[i]
			entries := m.searchResults.entriesForSet(i)
			if len(entries) == 0 {
				continue
			}

			imgui.TableNextColumn()

			isPreviewed := false
			if previewKey != "" {
				for _, entry := range entries {
					if entry.mapKey == previewKey {
						isPreviewed = true
						break
					}
				}
			}

			c1 := imgui.CursorPos().Y

			imgui.PushIDInt(int32(i))

			imgui.BeginGroup()

			if imgui.BeginTableV("bsetstab", 2, imgui.TableFlagsSizingStretchProp, vec2(-1, 0), -1) {
				imgui.PushFont(Font, 32)

				imgui.TableSetupColumnV("##title", imgui.TableColumnFlagsWidthStretch, 0, imgui.ID(0))
				imgui.TableSetupColumnV("##actions", imgui.TableColumnFlagsWidthFixed, imgui.FrameHeight()*2+imgui.CurrentStyle().ItemSpacing().X, imgui.ID(1))

				imgui.TableNextColumn()

				imgui.PushTextWrapPos()

				imgui.TextUnformatted(b.title)

				imgui.PopTextWrapPos()

				imgui.PopFont()

				imgui.TableNextColumn()

				if b.hovered {
					imgui.PushFont(Font, 20)

					imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
					imgui.PushStyleColorVec4(imgui.ColButton, vec4(0, 0, 0, 1))
					imgui.PushStyleColorVec4(imgui.ColButtonActive, vec4(0.2, 0.2, 0.2, 1))
					imgui.PushStyleColorVec4(imgui.ColButtonHovered, vec4(0.4, 0.4, 0.4, 1))

					s := entries[0].entry.SetID == 0

					if s {
						imgui.BeginDisabled()
					}

					imgui.PushFont(FontAw, 16)

					imgui.AlignTextToFramePadding()
					if imgui.ButtonV("\uF7A2", vec2(imgui.FrameHeight()*2, imgui.FrameHeight()*2)) {
						platform.OpenURL(fmt.Sprintf("https://osu.ppy.sh/s/%d", entries[0].entry.SetID))
					}

					if s {
						imgui.EndDisabled()
					}

					imgui.PopFont()

					if imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenDisabled) {
						imgui.BeginTooltip()

						if s {
							imgui.TextUnformatted("Not available")
						} else {
							imgui.TextUnformatted(fmt.Sprintf("https://osu.ppy.sh/s/%d", entries[0].entry.SetID))
						}

						imgui.EndTooltip()
					}

					imgui.SameLine()

					name := "\uF04B"
					if isPreviewed {
						name = "\uF04D"
					}

					imgui.PushFont(FontAw, 16)

					imgui.AlignTextToFramePadding()
					if imgui.ButtonV(name, vec2(imgui.FrameHeight()*2, imgui.FrameHeight()*2)) {
						m.stopPreview()

						if name == "\uF04B" {
							m.startPreview(entries[0].entry)
						}
					}

					imgui.PopFont()

					if imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenDisabled) {
						imgui.BeginTooltip()

						if isPreviewed {
							imgui.TextUnformatted("Stop preview")
						} else {
							imgui.TextUnformatted("Play preview")
						}

						imgui.EndTooltip()
					}

					imgui.PopStyleVar()
					imgui.PopStyleColor()
					imgui.PopStyleColor()
					imgui.PopStyleColor()

					imgui.PopFont()
				}

				imgui.EndTable()
			}

			imgui.TextUnformatted(b.artistCreator)

			imgui.PushFont(Font, 20)

			for j, entry := range entries {
				imgui.PushIDInt(int32(j))

				tSiz := imgui.CalcTextSizeV(entry.difficultyLabel, false, 0)

				sPos := imgui.CursorScreenPos()

				if imgui.SelectableBoolV(entry.difficultyLabel, entry.matches(currentMap), 0, vzero()) {
					if bMap, ok := m.materializeEntry(entry.entry); ok {
						m.bld.setMap(bMap)

						if !isPreviewed && launcherConfig.PreviewSelected {
							m.stopPreview()
							m.startPreview(entry.entry)
						}

						m.opened = false
					}
				}

				if tooltipAllowed && imgui.IsItemHovered() && ImIO.MousePos().X <= sPos.X+tSiz.X && imgui.BeginItemTooltip() {
					m.drawMapTooltip(entry.entry)
					imgui.EndTooltip()
				}

				imgui.PopID()
			}

			imgui.PopFont()

			imgui.SetCursorPos(imgui.CursorPos().Add(vec2(imgui.ContentRegionAvail().X, 0))) //hack to get cell hovering to work
			imgui.Dummy(vzero())

			imgui.EndGroup()

			b.hovered = imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenBlockedByActiveItem | imgui.HoveredFlagsAllowWhenOverlapped)

			c2 := imgui.CursorPos().Y
			measuredHeight := max(0, c2-c1)
			previousBottom := m.layout.top(i) + m.layout.height(i)
			if delta := m.layout.update(i, measuredHeight); delta != 0 && previousBottom <= scrollY {
				scrollCorrection += delta
			}

			imgui.PopID()
		}

		if endIndex < len(m.searchResults.sets) || startIndex == len(m.searchResults.sets) {
			imgui.TableNextColumn()
			currentY := imgui.CursorPos().Y - listStartY
			dummyExactY(max(0, m.layout.total()-currentY))
		}

		imgui.EndTable()
	}

	imgui.PopStyleVar()

	if scrollCorrection != 0 {
		imgui.SetScrollYFloat(max(0, scrollY+scrollCorrection))
	}
	m.lastScrollY = imgui.ScrollY()

	imgui.EndChild()

	imgui.WindowDrawList().AddLine(csPos, csPos.Add(vec2(imgui.ContentRegionAvail().X, 0)), packColor(*imgui.StyleColorVec4(imgui.ColSeparator)))
}

func (m *songSelectPopup) rebuildLayout(width float32) {
	style := imgui.CurrentStyle()
	spacingY := style.ItemSpacing().Y

	imgui.PushFont(Font, 20)
	buttonSize := imgui.FrameHeight() * 2
	buttonWidth := buttonSize + style.ItemSpacing().X
	difficultyHeight := imgui.TextLineHeight()
	imgui.PopFont()

	imgui.PushFont(Font, 24)
	artistHeight := imgui.TextLineHeight()
	imgui.PopFont()

	titleWidth := max(1, width-buttonWidth)
	imgui.PushFont(Font, 32)
	m.layout.rebuild(len(m.searchResults.sets), func(index int) float32 {
		set := &m.searchResults.sets[index]
		titleHeight := imgui.CalcTextSizeV(set.title, false, titleWidth).Y
		if titleHeight <= 0 {
			titleHeight = imgui.TextLineHeight()
		}

		return max(titleHeight, buttonSize) + artistHeight + float32(set.entryEnd-set.entryStart)*difficultyHeight + spacingY*3 + 4
	})
	imgui.PopFont()

	m.layoutWidth = width
	m.layoutReady = true
}

func (m *songSelectPopup) materializeEntry(entry *database.BeatmapEntry) (*beatmap.BeatMap, bool) {
	if entry == nil {
		return nil, false
	}

	if m.materialize == nil {
		return entry.NewBeatMap(), true
	}

	bMap, err := m.materialize(entry)
	if err != nil {
		showMessage(mError, "Failed to load map %q/%q: %s", entry.Dir, entry.File, err)
		return nil, false
	}

	return bMap, true
}

type mapIdentity struct {
	mapKey string
	md5    string
}

func newMapIdentity(bMap *beatmap.BeatMap) mapIdentity {
	if bMap == nil {
		return mapIdentity{}
	}

	return mapIdentity{
		mapKey: normalizeMapKey(filepath.Join(bMap.Dir, bMap.File)),
		md5:    strings.ToLower(bMap.MD5),
	}
}

func (entry *searchEntry) matches(identity mapIdentity) bool {
	if entry == nil || identity.mapKey == "" && identity.md5 == "" {
		return false
	}
	if entry.md5 != "" && identity.md5 != "" && entry.md5 == identity.md5 {
		return true
	}

	return entry.mapKey == identity.mapKey
}

func (m *songSelectPopup) drawMapTooltip(entry *database.BeatmapEntry) {
	imgui.PushFont(Font, 24)

	const tgAsp = float32(4.0 / 3)

	cPos := imgui.CursorPos()

	thumbPath := filepath.Join(settings.General.GetSongsDir(), entry.Dir, entry.Background)

	if m.lastThumbPath != thumbPath {
		m.releaseThumbnail()

		pX, err := texture.NewPixmapFileString(thumbPath)
		if err == nil {
			m.thumbTex = texture.LoadTextureSingle(pX.RGBA(), 4)

			m.texRef = imgui.NewTextureRefTextureID(imgui.TextureID(m.thumbTex.GetID()))

			pX.Dispose()
		}

		m.lastThumbPath = thumbPath
	}

	if m.thumbTex != nil {
		uvTL := vec2(0, 0)
		uvBR := vec2(1, 1)

		asp := float32(m.thumbTex.GetWidth()) / float32(m.thumbTex.GetHeight())

		if asp > tgAsp {
			uvTL.X = (1 - tgAsp/asp) / 2
			uvBR.X = 1 - uvTL.X
		} else {
			uvTL.Y = (1 - asp/tgAsp) / 2
			uvBR.Y = 1 - uvTL.Y
		}

		imgui.ImageWithBgV(*m.texRef, vec2(200*tgAsp, 200), uvTL, uvBR, imgui.Vec4{}, imgui.Vec4{X: 1, Y: 1, Z: 1, W: 0.3})
	}

	imgui.SetCursorPos(cPos)

	sR := "N/A"
	if entry.Stars >= 0 {
		sR = mutils.FormatWOZeros(entry.Stars, 2)
	}

	bpm := fmt.Sprintf("%.0f", entry.MinBPM)
	if math.Abs(entry.MinBPM-entry.MaxBPM) > 0.01 {
		bpm = fmt.Sprintf("%.0f - %.0f", entry.MinBPM, entry.MaxBPM)
	}

	if imgui.BeginTableV("btooltip", 4, imgui.TableFlagsSizingStretchProp|imgui.TableFlagsNoClip, vec2(200.0*tgAsp, 0), -1) {
		imgui.TableSetupColumnV("btooltip1", imgui.TableColumnFlagsWidthFixed, 0, imgui.ID(0))
		imgui.TableSetupColumnV("btooltip2", imgui.TableColumnFlagsWidthStretch, 0, imgui.ID(1))
		imgui.TableSetupColumnV("btooltip3", imgui.TableColumnFlagsWidthFixed, 0, imgui.ID(2))
		imgui.TableSetupColumnV("btooltip4", imgui.TableColumnFlagsWidthFixed, imgui.CalcTextSizeV("9.9", false, 0).X, imgui.ID(3))

		tRow := func(text string, text2 string) {
			textColumn(text)
			textColumn(text2)
		}

		tRow("Stars: ", sR)
		tRow("", "")

		tRow("Objects: ", fmt.Sprintf("%d", entry.Circles+entry.Sliders+entry.Spinners))
		tRow("AR: ", mutils.FormatWOZeros(entry.ApproachRate, 2))

		tRow("Circles: ", fmt.Sprintf("%d", entry.Circles))
		tRow("OD: ", mutils.FormatWOZeros(entry.OverallDifficulty, 2))

		tRow("Sliders: ", fmt.Sprintf("%d", entry.Sliders))
		tRow("CS: ", mutils.FormatWOZeros(entry.CircleSize, 2))

		tRow("Spinners: ", fmt.Sprintf("%d", entry.Spinners))
		tRow("HP: ", mutils.FormatWOZeros(entry.HealthDrain, 2))

		tRow("BPM: ", bpm)
		tRow("", "")

		tRow("Length: ", util.FormatSeconds(entry.Length/1000))
		tRow("", "")

		imgui.EndTable()
	}

	imgui.PopFont()
}

// releaseThumbnail tears down both sides of the native texture ownership
// boundary. The ImGui reference must not outlive the OpenGL texture it names,
// and keeping either object after the popup closes would retain the last map
// background until process shutdown.
func (m *songSelectPopup) releaseThumbnail() {
	if m.texRef != nil {
		m.texRef.Destroy()
		m.texRef = nil
	}
	if m.thumbTex != nil {
		m.thumbTex.Dispose()
		m.thumbTex = nil
	}

	m.lastThumbPath = ""
}

func (m *songSelectPopup) selectRandom() {
	if len(m.searchResults.sets) == 0 {
		return
	}

	i := rand.Intn(len(m.searchResults.sets))

	entries := m.searchResults.entriesForSet(i)
	if len(entries) == 0 {
		return
	}

	entry := entries[len(entries)-1].entry
	bMap, ok := m.materializeEntry(entry)
	if !ok {
		return
	}

	m.bld.setMap(bMap)
	m.focusTheMap = true

	if launcherConfig.PreviewSelected {
		m.stopPreview()
		m.startPreview(entry)
	}
}

func (m *songSelectPopup) selectNewest() {
	if len(m.beatmaps) == 0 {
		return
	}

	lastTimeStamp := m.beatmaps[0].entry.TimeAdded
	selectEntry := m.beatmaps[0].entry

	for _, mapName := range m.beatmaps {
		tStamp := max(mapName.entry.TimeAdded, mapName.entry.LastModified)

		if tStamp > lastTimeStamp {
			lastTimeStamp = tStamp
			selectEntry = mapName.entry
		}
	}

	selectMap, ok := m.materializeEntry(selectEntry)
	if !ok {
		return
	}

	m.bld.setMap(selectMap)
	m.focusTheMap = true

	if launcherConfig.PreviewSelected {
		m.stopPreview()
		m.startPreview(selectEntry)
	}
}

func (m *songSelectPopup) stopPreview() {
	if m.PreviewedSong != nil {
		m.PreviewedSong.Stop()
	}

	m.PreviewedSong = nil
	m.prevMap = nil
	m.prevEntry = nil
	m.previewMapKey = ""
}

func (m *songSelectPopup) startPreview(entry *database.BeatmapEntry) {
	bMap, ok := m.materializeEntry(entry)
	if !ok {
		return
	}

	cT := qpc.GetMilliTimeF()

	var track *bass.TrackBass
	if fPath, err2 := bMap.GetAudioFile(); err2 == nil {
		track = bass.NewTrack(fPath)
	}

	if track != nil {
		beatmap.ParseTimingPointsAndPauses(bMap)

		prevTime := float64(bMap.PreviewTime)
		if prevTime < 0 {
			prevTime = float64(bMap.Length) * 0.4
		}

		track.SetPosition(prevTime / 1000)
		track.PlayV(0)
		m.PreviewedSong = track

		m.volume.Reset()
		m.volume.AddEventS(cT, cT+1000, 0, 1)
		m.volume.AddEventS(cT+9000, cT+10000, 1, 0)
		m.stopTime = cT + 10000
		m.prevMap = bMap
		m.prevEntry = entry
		m.previewMapKey = normalizeMapKey(entry.MapKey())
	}
}

func (m *songSelectPopup) search() {
	m.layoutReady = false
	if len(m.beatmaps) > 0 && m.groupCount == 0 {
		m.groupByDirectory = assignSearchGroupIndices(m.beatmaps)
		m.groupCount = len(m.groupByDirectory)
	}
	m.searchResults, m.searchScratch, m.groupScratch = searchMapSetsWithScratch(m.beatmaps, m.searchStr, m.searchScratch, m.groupCount, m.groupScratch)
}

func searchMapSets(beatmaps searchEntries, query string) searchResults {
	groupByDir := assignSearchGroupIndices(beatmaps)
	results, _, _ := searchMapSetsWithScratch(beatmaps, query, nil, len(groupByDir), nil)
	return results
}

func assignSearchGroupIndices(beatmaps searchEntries) map[string]int {
	// Group IDs are assigned after sorting and reused for every query until the
	// sort order changes. This keeps query-time grouping on integer indexes
	// instead of rebuilding a string map for every keystroke.
	groupByDir := make(map[string]int, max(1, len(beatmaps)/4))
	for _, entry := range beatmaps {
		if entry == nil {
			continue
		}

		groupIndex, ok := groupByDir[entry.directoryKey]
		if !ok {
			groupIndex = len(groupByDir)
			groupByDir[entry.directoryKey] = groupIndex
		}
		entry.groupIndex = groupIndex
	}

	return groupByDir
}

// searchMapSetsWithScratch expects groupIndex values to have been assigned by
// assignSearchGroupIndices. The popup maintains that invariant across sort
// changes so this function can keep its query path allocation-light.
func searchMapSetsWithScratch(beatmaps searchEntries, query string, scratch []searchMatch, groupCount int, groupScratch []int) (searchResults, []searchMatch, []int) {
	query = strings.ToLower(query)
	if groupCount == 0 && len(beatmaps) > 0 {
		groupCount = len(assignSearchGroupIndices(beatmaps))
	}
	groupHint := min(groupCount, max(1, len(beatmaps)/4))

	results := searchResults{
		sets: make([]songSet, 0, groupHint),
	}
	matches := scratch[:0]
	if cap(groupScratch) < groupCount {
		groupScratch = make([]int, groupCount)
	} else {
		groupScratch = groupScratch[:groupCount]
		clear(groupScratch)
	}

	for _, entry := range beatmaps {
		if entry == nil || query != "" && !strings.Contains(entry.searchKey, query) {
			continue
		}

		// Zero means that the group has not appeared in this query. Store the
		// result index plus one so result index zero remains distinguishable.
		setIndex := groupScratch[entry.groupIndex]
		if setIndex == 0 {
			setIndex = len(results.sets) + 1
			groupScratch[entry.groupIndex] = setIndex
			setIndex--
			results.sets = append(results.sets, songSet{
				directory:     entry.directoryKey,
				title:         entry.entry.Name,
				artistCreator: entry.artistCreator,
				groupIndex:    entry.groupIndex,
			})
		} else {
			setIndex--
		}

		results.sets[setIndex].matchCount++
		matches = append(matches, searchMatch{entry: entry, setIndex: setIndex})
	}

	if len(matches) > 0 {
		results.entries = make([]*searchEntry, len(matches))
		groupOffsets := make([]int, len(results.sets))
		cursor := 0
		for index := range results.sets {
			set := &results.sets[index]
			set.entryStart = cursor
			set.entryEnd = set.entryStart + set.matchCount
			groupOffsets[index] = set.entryStart
			cursor = set.entryEnd
		}

		for _, match := range matches {
			position := groupOffsets[match.setIndex]
			results.entries[position] = match.entry
			groupOffsets[match.setIndex]++
		}
	}

	// The scratch entries contain pointers into the current catalog. Clear
	// them before retaining the capacity so a later catalog publication can be
	// reclaimed even if the selector itself remains alive.
	clear(matches)
	return results, matches[:0], groupScratch
}

func (m *songSelectPopup) open() {
	if m.catalogDirty {
		m.setCatalog(m.catalog)
	}

	m.focusTheMap = true

	m.popup.open()
}

func sortMaps(bMaps searchEntries, sortBy SortBy) map[string]int {
	slices.SortStableFunc(bMaps, func(b1, b2 *searchEntry) int {
		entry1 := b1.entry
		entry2 := b2.entry
		var res int

		switch sortBy {
		case Title:
			res = cmp.Compare(b1.titleKey, b2.titleKey)
		case Artist:
			res = cmp.Compare(b1.artistKey, b2.artistKey)
		case Creator:
			res = cmp.Compare(b1.creatorKey, b2.creatorKey)
		case DateAdded:
			if b1.directoryKey != b2.directoryKey || mutils.Abs(entry1.LastModified/1000-entry2.LastModified/1000) > 10 {
				res = cmp.Compare(entry1.LastModified/1000, entry2.LastModified/1000)
			} else {
				res = 0
			}
		case Difficulty:
			res = cmp.Compare(entry1.Stars, entry2.Stars)
		}

		if !launcherConfig.SortAscending {
			res = -res
		}

		if res != 0 {
			return res
		}

		res = cmp.Compare(b1.directoryKey, b2.directoryKey)

		if !launcherConfig.SortAscending {
			res = -res
		}

		if res != 0 {
			return res
		}

		return cmp.Compare(entry1.Stars, entry2.Stars) // Don't flip grouped difficulties
	})

	return assignSearchGroupIndices(bMaps)
}
