package launcher

import (
	"cmp"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
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

type mapWithName struct {
	name    string
	title   string
	artist  string
	creator string
	dir     string
	entry   *database.BeatmapEntry
}

func newMapWithName(entry *database.BeatmapEntry) *mapWithName {
	return &mapWithName{
		name:    entry.SearchKey(),
		title:   strings.ToLower(entry.Name),
		artist:  strings.ToLower(entry.Artist),
		creator: strings.ToLower(entry.Creator),
		dir:     strings.ToLower(entry.Dir),
		entry:   entry,
	}
}

type beatmapSet struct {
	directory string
	top       float32
	height    float32
	measured  bool
	entries   []*database.BeatmapEntry
	hovered   bool
}

type maps []*mapWithName

func (e maps) String(i int) string {
	return e[i].name
}

func (e maps) Len() int {
	return len(e)
}

type songSelectPopup struct {
	*popup

	bld      *builder
	beatmaps maps

	searchResults []*beatmapSet
	searchStr     string

	prevMap       *beatmap.BeatMap
	prevEntry     *database.BeatmapEntry
	PreviewedSong *bass.TrackBass
	volume        *animation.Glider
	stopTime      float64
	thumbTex      *texture.TextureSingle
	texRef        *imgui.TextureRef
	lastThumbPath string
	drawTex       bool
	focusTheMap   bool

	comboOpened bool
	scrolling   bool

	materialize  func(*database.BeatmapEntry) (*beatmap.BeatMap, error)
	catalog      *database.CatalogSnapshot
	catalogDirty bool
	layoutWidth  float32
	layoutReady  bool
	layoutDirty  bool
}

func newSongSelectPopup(bld *builder, catalog *database.CatalogSnapshot, materialize func(*database.BeatmapEntry) (*beatmap.BeatMap, error)) *songSelectPopup {
	mP := &songSelectPopup{
		popup:       newPopup("Song select", popBig),
		bld:         bld,
		volume:      animation.NewGlider(0),
		materialize: materialize,
	}

	mP.internalDraw = mP.drawSongSelect

	mP.setCatalog(catalog)

	return mP
}

func (m *songSelectPopup) setCatalog(catalog *database.CatalogSnapshot) {
	m.catalog = catalog
	m.catalogDirty = false

	beatmaps := make(maps, 0)
	if catalog != nil {
		beatmaps = make(maps, 0, catalog.Len())
		catalog.ForEach(func(entry *database.BeatmapEntry) bool {
			beatmaps = append(beatmaps, newMapWithName(entry))
			return true
		})
	}

	m.beatmaps = beatmaps
	sortMaps(m.beatmaps, launcherConfig.SortMapsBy)
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
					sortMaps(m.beatmaps, launcherConfig.SortMapsBy)
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
			sortMaps(m.beatmaps, launcherConfig.SortMapsBy)
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

	m.scrolling = handleDragScroll()

	imgui.PushStyleVarVec2(imgui.StyleVarFramePadding, vec2(5, 0))
	listStartY := imgui.CursorPos().Y
	listWidth := imgui.ContentRegionAvail().X
	if !m.layoutReady || m.layoutDirty || math.Abs(float64(listWidth-m.layoutWidth)) > 1 {
		m.prepareLayout(listWidth)
	}

	if m.focusTheMap {
		if m.bld.currentMap != nil {
			currentDirectory := strings.ToLower(m.bld.currentMap.Dir)
			for _, result := range m.searchResults {
				if result.directory != currentDirectory {
					continue
				}
				if slices.ContainsFunc(result.entries, func(entry *database.BeatmapEntry) bool {
					return entryMatchesMap(entry, m.bld.currentMap)
				}) {
					imgui.SetScrollYFloat(result.top)
					break
				}
			}
		}
		m.focusTheMap = false
	}

	startIndex, endIndex := m.visibleRange(imgui.ScrollY(), imgui.ContentRegionAvail().Y)

	if imgui.BeginTableV("bsetstab", 1, imgui.TableFlagsRowBg|imgui.TableFlagsPadOuterX|imgui.TableFlagsBordersH, vec2(-1, 0), -1) {
		imgui.TableSetBgColor(imgui.TableBgTargetRowBg1, packColor(vec4(0.5, 0.5, 0.5, 1)))

		if startIndex > 0 {
			imgui.TableNextColumn()
			currentY := imgui.CursorPos().Y - listStartY
			top := m.totalLayoutHeight()
			if startIndex < len(m.searchResults) {
				top = m.searchResults[startIndex].top
			}
			dummyExactY(max(0, top-currentY))
		}

		for i := startIndex; i < endIndex; i++ {
			b := m.searchResults[i]

			imgui.TableNextColumn()

			isPreviewed := slices.ContainsFunc(b.entries, func(entry *database.BeatmapEntry) bool {
				return m.prevEntry != nil && entry.MapKey() == m.prevEntry.MapKey()
			})

			c1 := imgui.CursorPos().Y

			rId := strconv.Itoa(i)

			imgui.BeginGroup()

			if imgui.BeginTableV("bsetstab"+rId, 2, imgui.TableFlagsSizingStretchProp, vec2(-1, 0), -1) {
				imgui.PushFont(Font, 32)

				imgui.TableSetupColumnV("##hhh"+rId, imgui.TableColumnFlagsWidthStretch, 0, imgui.ID(0))
				imgui.TableSetupColumnV("##hhhg"+rId, imgui.TableColumnFlagsWidthFixed, imgui.FrameHeight()*2+imgui.CurrentStyle().ItemSpacing().X, imgui.ID(1))

				imgui.TableNextColumn()

				imgui.PushTextWrapPos()

				imgui.TextUnformatted(b.entries[0].Name)

				imgui.PopTextWrapPos()

				imgui.PopFont()

				imgui.TableNextColumn()

				if b.hovered {
					imgui.PushFont(Font, 20)

					imgui.PushStyleVarFloat(imgui.StyleVarFrameBorderSize, 0)
					imgui.PushStyleColorVec4(imgui.ColButton, vec4(0, 0, 0, 1))
					imgui.PushStyleColorVec4(imgui.ColButtonActive, vec4(0.2, 0.2, 0.2, 1))
					imgui.PushStyleColorVec4(imgui.ColButtonHovered, vec4(0.4, 0.4, 0.4, 1))

					s := b.entries[0].SetID == 0

					if s {
						imgui.BeginDisabled()
					}

					imgui.PushFont(FontAw, 16)

					imgui.AlignTextToFramePadding()
					if imgui.ButtonV("\uF7A2##"+rId, vec2(imgui.FrameHeight()*2, imgui.FrameHeight()*2)) {
						platform.OpenURL(fmt.Sprintf("https://osu.ppy.sh/s/%d", b.entries[0].SetID))
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
							imgui.TextUnformatted(fmt.Sprintf("https://osu.ppy.sh/s/%d", b.entries[0].SetID))
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
					if imgui.ButtonV(name+"##"+rId, vec2(imgui.FrameHeight()*2, imgui.FrameHeight()*2)) {
						m.stopPreview()

						if name == "\uF04B" {
							m.startPreview(b.entries[0])
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

			imgui.TextUnformatted(fmt.Sprintf("%s // %s", b.entries[0].Artist, b.entries[0].Creator))

			imgui.PushFont(Font, 20)

			for j, entry := range b.entries {
				fDiffName := ">   " + entry.Difficulty

				tSiz := imgui.CalcTextSizeV(fDiffName, false, 0)

				sPos := imgui.CursorScreenPos()

				if imgui.SelectableBoolV(fDiffName+"##"+rId+"s"+strconv.Itoa(j), entryMatchesMap(entry, m.bld.currentMap), 0, vzero()) {
					bMap, ok := m.materializeEntry(entry)
					if !ok {
						continue
					}

					m.bld.setMap(bMap)

					if !isPreviewed && launcherConfig.PreviewSelected {
						m.stopPreview()
						m.startPreview(entry)
					}

					m.opened = false
				}

				if imgui.IsItemHovered() && ImIO.MousePos().X <= sPos.X+tSiz.X {
					m.showMapTooltip(entry)
				}
			}

			imgui.PopFont()

			imgui.SetCursorPos(imgui.CursorPos().Add(vec2(imgui.ContentRegionAvail().X, 0))) //hack to get cell hovering to work
			imgui.Dummy(vzero())

			imgui.EndGroup()

			b.hovered = imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenBlockedByActiveItem | imgui.HoveredFlagsAllowWhenOverlapped)

			c2 := imgui.CursorPos().Y
			measuredHeight := max(0, c2-c1)
			if !b.measured || math.Abs(float64(b.height-measuredHeight)) > 0.5 {
				b.height = measuredHeight
				b.measured = true
				m.layoutDirty = true
			}
		}

		if endIndex < len(m.searchResults) || startIndex == len(m.searchResults) {
			imgui.TableNextColumn()
			currentY := imgui.CursorPos().Y - listStartY
			dummyExactY(max(0, m.totalLayoutHeight()-currentY))
		}

		imgui.EndTable()
	}

	imgui.PopStyleVar()

	imgui.EndChild()

	imgui.WindowDrawList().AddLine(csPos, csPos.Add(vec2(imgui.ContentRegionAvail().X, 0)), packColor(*imgui.StyleColorVec4(imgui.ColSeparator)))
}

func (m *songSelectPopup) prepareLayout(width float32) {
	if math.Abs(float64(width-m.layoutWidth)) > 1 {
		for _, result := range m.searchResults {
			result.measured = false
		}
	}

	top := float32(0)
	for _, result := range m.searchResults {
		if !result.measured {
			result.height = m.estimateSetHeight(result, width)
		}
		result.top = top
		top += result.height
	}

	m.layoutWidth = width
	m.layoutReady = true
	m.layoutDirty = false
}

func (m *songSelectPopup) estimateSetHeight(result *beatmapSet, width float32) float32 {
	if result == nil || len(result.entries) == 0 {
		return 0
	}

	imgui.PushFont(Font, 20)
	buttonSize := imgui.FrameHeight() * 2
	buttonWidth := buttonSize + imgui.CurrentStyle().ItemSpacing().X
	difficultyHeight := imgui.TextLineHeight()
	imgui.PopFont()

	imgui.PushFont(Font, 32)
	titleWidth := max(1, width-buttonWidth)
	titleHeight := imgui.CalcTextSizeV(result.entries[0].Name, false, titleWidth).Y
	if titleHeight <= 0 {
		titleHeight = imgui.TextLineHeight()
	}
	imgui.PopFont()

	imgui.PushFont(Font, 24)
	artistHeight := imgui.TextLineHeight()
	imgui.PopFont()

	return max(titleHeight, buttonSize) + artistHeight + float32(len(result.entries))*difficultyHeight + imgui.CurrentStyle().ItemSpacing().Y*3 + 4
}

func (m *songSelectPopup) visibleRange(scrollY, viewportHeight float32) (int, int) {
	count := len(m.searchResults)
	if count == 0 {
		return 0, 0
	}

	start := sort.Search(count, func(index int) bool {
		result := m.searchResults[index]
		return result.top+result.height >= scrollY
	})
	end := sort.Search(count, func(index int) bool {
		return m.searchResults[index].top > scrollY+viewportHeight
	})

	if start > 0 {
		start--
	}
	if end < count {
		end++
	}
	if end < start {
		end = start
	}

	return start, end
}

func (m *songSelectPopup) totalLayoutHeight() float32 {
	if len(m.searchResults) == 0 {
		return 0
	}

	last := m.searchResults[len(m.searchResults)-1]
	return last.top + last.height
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

func entryMatchesMap(entry *database.BeatmapEntry, bMap *beatmap.BeatMap) bool {
	if entry == nil || bMap == nil {
		return false
	}
	if entry.MD5 != "" && bMap.MD5 != "" && strings.EqualFold(entry.MD5, bMap.MD5) {
		return true
	}

	return strings.EqualFold(entry.MapKey(), strings.ToLower(filepath.ToSlash(filepath.Join(bMap.Dir, bMap.File))))
}

func (m *songSelectPopup) showMapTooltip(entry *database.BeatmapEntry) {
	imgui.PushFont(Font, 24)

	const tgAsp = float32(4.0 / 3)

	imgui.BeginTooltip()

	cPos := imgui.CursorPos()

	thumbPath := filepath.Join(settings.General.GetSongsDir(), entry.Dir, entry.Background)

	if m.lastThumbPath != thumbPath {
		if m.thumbTex != nil {
			m.thumbTex.Dispose()
			m.texRef.Destroy()
			m.thumbTex = nil
		}

		pX, err := texture.NewPixmapFileString(thumbPath)
		if err == nil {
			m.thumbTex = texture.LoadTextureSingle(pX.RGBA(), 4)

			m.texRef = imgui.NewTextureRefTextureID(imgui.TextureID(m.thumbTex.GetID()))

			pX.Dispose()
		}

		m.lastThumbPath = thumbPath
		m.drawTex = false
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
			uvBR.Y = 1 - uvTL.X
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
	imgui.EndTooltip()
}

func (m *songSelectPopup) selectRandom() {
	if len(m.searchResults) == 0 {
		return
	}

	i := rand.Intn(len(m.searchResults))

	entry := m.searchResults[i].entries[len(m.searchResults[i].entries)-1]
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
		m.PreviewedSong = nil
		m.prevMap = nil
		m.prevEntry = nil
	}
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
	}
}

func (m *songSelectPopup) search() {
	m.layoutReady = false
	m.layoutDirty = false
	m.searchResults = searchMapSets(m.beatmaps, m.searchStr)
}

func searchMapSets(beatmaps maps, query string) []*beatmapSet {
	results := make([]*beatmapSet, 0)
	sString := strings.ToLower(query)

	for _, mapName := range beatmaps {
		if sString != "" && !strings.Contains(mapName.name, sString) {
			continue
		}

		entry := mapName.entry
		entryDirectory := mapName.dir
		if len(results) == 0 || results[len(results)-1].directory != entryDirectory {
			results = append(results, &beatmapSet{
				directory: entryDirectory,
				entries:   make([]*database.BeatmapEntry, 0, 1),
			})
		}

		results[len(results)-1].entries = append(results[len(results)-1].entries, entry)
	}

	return results
}

func (m *songSelectPopup) open() {
	if m.catalogDirty {
		m.setCatalog(m.catalog)
	}

	m.focusTheMap = true

	m.popup.open()
}

func sortMaps(bMaps maps, sortBy SortBy) {
	slices.SortStableFunc(bMaps, func(b1, b2 *mapWithName) int {
		entry1 := b1.entry
		entry2 := b2.entry
		var res int

		switch sortBy {
		case Title:
			res = cmp.Compare(b1.title, b2.title)
		case Artist:
			res = cmp.Compare(b1.artist, b2.artist)
		case Creator:
			res = cmp.Compare(b1.creator, b2.creator)
		case DateAdded:
			if b1.dir != b2.dir || mutils.Abs(entry1.LastModified/1000-entry2.LastModified/1000) > 10 {
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

		res = cmp.Compare(b1.dir, b2.dir)

		if !launcherConfig.SortAscending {
			res = -res
		}

		if res != 0 {
			return res
		}

		return cmp.Compare(entry1.Stars, entry2.Stars) // Don't flip grouped difficulties
	})
}
