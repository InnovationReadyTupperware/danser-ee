package database

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
)

const catalogShardCount = 256

// BeatmapEntry is the immutable metadata needed by the launcher and song
// selector. Runtime beatmap state is deliberately not stored here: parsing
// objects, timings, samples, and render state for every installed map would
// make a large library expensive to load and would allow gameplay code to
// mutate data visible to search workers.
type BeatmapEntry struct {
	Dir               string
	File              string
	Name              string
	NameUnicode       string
	Artist            string
	ArtistUnicode     string
	Creator           string
	Difficulty        string
	Source            string
	Tags              string
	Audio             string
	Background        string
	MD5               string
	Mode              int64
	SetID             int64
	ID                int64
	LastModified      int64
	FileSize          int64
	TimeAdded         int64
	PlayCount         int64
	LastPlayed        int64
	PreviewTime       int64
	LocalOffset       int
	SampleSet         int
	Circles           int
	Sliders           int
	Spinners          int
	Length            int
	StarsVersion      int
	MetadataState     MetadataState
	Stars             float64
	MinBPM            float64
	MaxBPM            float64
	CircleSize        float64
	ApproachRate      float64
	HealthDrain       float64
	OverallDifficulty float64
	SliderMult        float64
	SliderTickRate    float64
	StackLeniency     float64

	pathKey   string
	searchKey string
}

// MetadataState describes how much of an entry has been read from its source
// file. Stable's osu!.db can provide useful catalog metadata, but exact .osu
// fields are still loaded lazily before gameplay.
type MetadataState uint8

const (
	MetadataComplete MetadataState = iota
	MetadataFromStableDatabase
	MetadataInvalid
	MetadataMissing
)

// MapKey returns the case-insensitive source-relative identity of the map.
// The relative path is kept separately from the key so display and gameplay
// paths retain the casing used by the source filesystem.
func (e *BeatmapEntry) MapKey() string {
	if e == nil {
		return ""
	}

	if e.pathKey != "" {
		return e.pathKey
	}

	return catalogPathKey(e.Dir, e.File)
}

// SearchKey returns the normalized string used by song-select filtering.
// It is calculated once per entry instead of once per query or comparison.
func (e *BeatmapEntry) SearchKey() string {
	if e == nil {
		return ""
	}

	if e.searchKey != "" {
		return e.searchKey
	}

	return catalogSearchKey(e)
}

// NewBeatMap materializes a runtime beatmap from catalog metadata. Callers
// that need timings or hitobjects must still parse the backing .osu file.
func (e *BeatmapEntry) NewBeatMap() *beatmap.BeatMap {
	bMap := beatmap.NewBeatMap()
	bMap.Dir = e.Dir
	bMap.File = e.File
	bMap.Name = e.Name
	bMap.NameUnicode = e.NameUnicode
	bMap.Artist = e.Artist
	bMap.ArtistUnicode = e.ArtistUnicode
	bMap.Creator = e.Creator
	bMap.Difficulty = e.Difficulty
	bMap.Source = e.Source
	bMap.Tags = e.Tags
	bMap.Audio = e.Audio
	bMap.Bg = e.Background
	bMap.MD5 = e.MD5
	bMap.Mode = e.Mode
	bMap.SetID = e.SetID
	bMap.ID = e.ID
	bMap.LastModified = e.LastModified
	bMap.FileSize = e.FileSize
	bMap.TimeAdded = e.TimeAdded
	bMap.PlayCount = e.PlayCount
	bMap.LastPlayed = e.LastPlayed
	bMap.PreviewTime = e.PreviewTime
	bMap.LocalOffset = e.LocalOffset
	bMap.Stars = e.Stars
	bMap.StarsVersion = e.StarsVersion
	bMap.MinBPM = e.MinBPM
	bMap.MaxBPM = e.MaxBPM
	bMap.Circles = e.Circles
	bMap.Sliders = e.Sliders
	bMap.Spinners = e.Spinners
	bMap.Length = e.Length
	bMap.SliderMultiplier = e.SliderMult
	bMap.StackLeniency = e.StackLeniency
	bMap.Diff.SetCS(e.CircleSize)
	bMap.Diff.SetAR(e.ApproachRate)
	bMap.Diff.SetHP(e.HealthDrain)
	bMap.Diff.SetOD(e.OverallDifficulty)
	bMap.Timings.SliderMult = e.SliderMult
	bMap.Timings.TickRate = e.SliderTickRate
	bMap.Timings.BaseSet = e.SampleSet

	return bMap
}

// NewBeatmapEntry extracts catalog metadata from a parsed beatmap.
func NewBeatmapEntry(bMap *beatmap.BeatMap) *BeatmapEntry {
	if bMap == nil {
		return nil
	}

	entry := &BeatmapEntry{
		Dir:               bMap.Dir,
		File:              bMap.File,
		Name:              bMap.Name,
		NameUnicode:       bMap.NameUnicode,
		Artist:            bMap.Artist,
		ArtistUnicode:     bMap.ArtistUnicode,
		Creator:           bMap.Creator,
		Difficulty:        bMap.Difficulty,
		Source:            bMap.Source,
		Tags:              bMap.Tags,
		Audio:             bMap.Audio,
		Background:        bMap.Bg,
		MD5:               bMap.MD5,
		Mode:              bMap.Mode,
		SetID:             bMap.SetID,
		ID:                bMap.ID,
		LastModified:      bMap.LastModified,
		FileSize:          bMap.FileSize,
		TimeAdded:         bMap.TimeAdded,
		PlayCount:         bMap.PlayCount,
		LastPlayed:        bMap.LastPlayed,
		PreviewTime:       bMap.PreviewTime,
		LocalOffset:       bMap.LocalOffset,
		SampleSet:         bMap.Timings.BaseSet,
		Circles:           bMap.Circles,
		Sliders:           bMap.Sliders,
		Spinners:          bMap.Spinners,
		Length:            bMap.Length,
		StarsVersion:      bMap.StarsVersion,
		Stars:             bMap.Stars,
		MinBPM:            bMap.MinBPM,
		MaxBPM:            bMap.MaxBPM,
		CircleSize:        bMap.Diff.GetBaseCS(),
		ApproachRate:      bMap.Diff.GetBaseAR(),
		HealthDrain:       bMap.Diff.GetBaseHP(),
		OverallDifficulty: bMap.Diff.GetBaseOD(),
		SliderMult:        bMap.SliderMultiplier,
		SliderTickRate:    bMap.Timings.TickRate,
		StackLeniency:     bMap.StackLeniency,
		MetadataState:     MetadataComplete,
	}
	entry.prepare()

	return entry
}

func (e *BeatmapEntry) prepare() {
	if e == nil {
		return
	}

	e.pathKey = catalogPathKey(e.Dir, e.File)
	e.searchKey = catalogSearchKey(e)
}

type catalogShard struct {
	entries []*BeatmapEntry
}

// CatalogSnapshot is a read-only view of catalog metadata. Snapshots share
// unchanged shards; applying a small delta copies only the fixed shard table
// and the shards that contain changed entries.
type CatalogSnapshot struct {
	generation uint64
	shards     [catalogShardCount]*catalogShard
	count      int
}

// NewCatalogSnapshot builds the initial snapshot. It is intentionally the
// only operation that needs to ingest every entry at once.
func NewCatalogSnapshot(entries []*BeatmapEntry) *CatalogSnapshot {
	snapshot := &CatalogSnapshot{}

	// Database rows are not ordered for the catalog's shard keys. Build each
	// shard with append and sort it once instead of inserting every row into a
	// growing slice. The difference is material for a cold cache containing
	// roughly 140,000 maps: repeated insertion shifts pointers for every row.
	for _, entry := range entries {
		if entry == nil {
			continue
		}

		entry.prepare()
		shardIndex := catalogShardFor(entry.MapKey())
		shard := snapshot.shards[shardIndex]
		if shard == nil {
			shard = &catalogShard{}
			snapshot.shards[shardIndex] = shard
		}
		shard.entries = append(shard.entries, entry)
	}

	for _, shard := range snapshot.shards {
		if shard == nil {
			continue
		}

		sort.SliceStable(shard.entries, func(i, j int) bool {
			return shard.entries[i].MapKey() < shard.entries[j].MapKey()
		})

		// The durable catalog has a case-insensitive unique location index, but
		// retain last-entry-wins behavior for callers constructing snapshots
		// directly and for recovery from older databases.
		unique := shard.entries[:0]
		for _, entry := range shard.entries {
			if len(unique) > 0 && unique[len(unique)-1].MapKey() == entry.MapKey() {
				unique[len(unique)-1] = entry
				continue
			}
			unique = append(unique, entry)
		}
		shard.entries = unique
		snapshot.count += len(unique)
	}

	return snapshot
}

// Generation identifies the snapshot used by asynchronous consumers.
func (s *CatalogSnapshot) Generation() uint64 {
	if s == nil {
		return 0
	}

	return s.generation
}

// Len reports the number of active entries without materializing a slice.
func (s *CatalogSnapshot) Len() int {
	if s == nil {
		return 0
	}

	return s.count
}

// ForEach visits all active entries. The callback can return false to stop.
func (s *CatalogSnapshot) ForEach(fn func(*BeatmapEntry) bool) {
	if s == nil || fn == nil {
		return
	}

	for _, shard := range s.shards {
		if shard == nil {
			continue
		}

		for _, entry := range shard.entries {
			if !fn(entry) {
				return
			}
		}
	}
}

// Lookup returns an entry by its canonical source-relative key.
func (s *CatalogSnapshot) Lookup(key string) *BeatmapEntry {
	if s == nil {
		return nil
	}

	key = normalizeCatalogKey(key)
	shard := s.shards[catalogShardFor(key)]
	if shard == nil {
		return nil
	}

	index := sort.Search(len(shard.entries), func(i int) bool {
		return shard.entries[i].MapKey() >= key
	})
	if index < len(shard.entries) && shard.entries[index].MapKey() == key {
		return shard.entries[index]
	}

	return nil
}

// ApplyDelta returns a snapshot that shares all untouched catalog shards.
func (s *CatalogSnapshot) ApplyDelta(delta CatalogDelta) *CatalogSnapshot {
	if s == nil {
		s = &CatalogSnapshot{}
	}
	if delta.empty() {
		return s
	}

	next := *s
	next.generation++
	var changed [catalogShardCount]bool

	for _, key := range delta.Removals {
		key = normalizeCatalogKey(key)
		shardIndex := catalogShardFor(key)
		if !changed[shardIndex] {
			next.shards[shardIndex] = cloneCatalogShard(next.shards[shardIndex])
			changed[shardIndex] = true
		}

		next.removeFromShard(shardIndex, key)
	}

	for _, entry := range delta.Upserts {
		if entry == nil {
			continue
		}

		entry.prepare()
		shardIndex := catalogShardFor(entry.MapKey())
		if !changed[shardIndex] {
			next.shards[shardIndex] = cloneCatalogShard(next.shards[shardIndex])
			changed[shardIndex] = true
		}

		if next.replaceInShard(shardIndex, entry) {
			continue
		}

		next.insertIntoShard(shardIndex, entry)
		next.count++
	}

	next.recount()
	return &next
}

// CatalogDelta describes a bounded catalog update. Keys in Removals must be
// canonical source-relative paths.
type CatalogDelta struct {
	Upserts  []*BeatmapEntry
	Removals []string
}

func (d CatalogDelta) empty() bool {
	return len(d.Upserts) == 0 && len(d.Removals) == 0
}

func (s *CatalogSnapshot) insert(entry *BeatmapEntry) {
	if entry == nil {
		return
	}

	entry.prepare()
	shardIndex := catalogShardFor(entry.MapKey())
	if s.replaceInShard(shardIndex, entry) {
		return
	}

	s.insertIntoShard(shardIndex, entry)
	s.count++
}

func (s *CatalogSnapshot) insertIntoShard(shardIndex int, entry *BeatmapEntry) {
	shard := s.shards[shardIndex]
	if shard == nil {
		shard = &catalogShard{}
		s.shards[shardIndex] = shard
	}

	index := sort.Search(len(shard.entries), func(i int) bool {
		return shard.entries[i].MapKey() >= entry.MapKey()
	})
	shard.entries = append(shard.entries, nil)
	copy(shard.entries[index+1:], shard.entries[index:])
	shard.entries[index] = entry
}

func (s *CatalogSnapshot) replaceInShard(shardIndex int, entry *BeatmapEntry) bool {
	shard := s.shards[shardIndex]
	if shard == nil {
		return false
	}

	index := sort.Search(len(shard.entries), func(i int) bool {
		return shard.entries[i].MapKey() >= entry.MapKey()
	})
	if index >= len(shard.entries) || shard.entries[index].MapKey() != entry.MapKey() {
		return false
	}

	shard.entries[index] = entry
	return true
}

func (s *CatalogSnapshot) removeFromShard(shardIndex int, key string) {
	shard := s.shards[shardIndex]
	if shard == nil {
		return
	}

	index := sort.Search(len(shard.entries), func(i int) bool {
		return shard.entries[i].MapKey() >= key
	})
	if index >= len(shard.entries) || shard.entries[index].MapKey() != key {
		return
	}

	copy(shard.entries[index:], shard.entries[index+1:])
	shard.entries = shard.entries[:len(shard.entries)-1]
}

func (s *CatalogSnapshot) recount() {
	count := 0
	for _, shard := range s.shards {
		if shard != nil {
			count += len(shard.entries)
		}
	}
	s.count = count
}

func cloneCatalogShard(shard *catalogShard) *catalogShard {
	if shard == nil {
		return &catalogShard{}
	}

	entries := make([]*BeatmapEntry, len(shard.entries))
	copy(entries, shard.entries)
	return &catalogShard{entries: entries}
}

func catalogPathKey(dir, file string) string {
	value := filepath.Join(dir, file)
	// Stable stores Windows separators even when a catalog is inspected by a
	// Linux build. Normalize both separator forms before hashing so the source
	// identity remains portable across platforms.
	return normalizeCatalogKey(value)
}

func normalizeCatalogKey(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	if value == "" {
		return ""
	}
	value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))

	return strings.ToLower(value)
}

func isAbsoluteCatalogPath(value string) bool {
	normalized := strings.ReplaceAll(value, "\\", "/")
	return filepath.IsAbs(value) || strings.HasPrefix(normalized, "/") ||
		(len(normalized) >= 2 && normalized[1] == ':')
}

func catalogShardFor(key string) int {
	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return int(hash & (catalogShardCount - 1))
}

func catalogSearchKey(entry *BeatmapEntry) string {
	var builder strings.Builder
	builder.Grow(len(entry.Artist) + len(entry.Name) + len(entry.Difficulty) + len(entry.Creator) + 32)
	builder.WriteString(entry.Artist)
	builder.WriteString(" - ")
	builder.WriteString(entry.Name)
	builder.WriteString(" [")
	builder.WriteString(entry.Difficulty)
	builder.WriteString("] by ")
	builder.WriteString(entry.Creator)
	builder.WriteByte(' ')
	builder.WriteString(strconv.FormatInt(entry.SetID, 10))
	builder.WriteByte(' ')
	builder.WriteString(strconv.FormatInt(entry.ID, 10))
	return strings.ToLower(builder.String())
}
