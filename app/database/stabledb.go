package database

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	stableDBMinVersion      = 20120101
	stableDBMaxVersion      = 20260711
	stableDBMaxMapCount     = 2_000_000
	stableDBMaxListCount    = 1_000_000
	stableDBMaxStringBytes  = 32 << 20
	stableDBMaxFileBytes    = 2 << 30
	windowsTicksToUnixEpoch = 621355968000000000
)

// stableDatabasePath returns only the conventional adjacent osu!.db path.
// The accelerator must never search the user's Songs tree to find its source.
func stableDatabasePath() string {
	return filepath.Join(filepath.Dir(songsDir), "osu!.db")
}

// loadStableDatabase returns provisional catalog entries. It is intentionally
// independent from the danser database so malformed Stable data can be
// rejected without affecting the existing catalog.
func loadStableDatabase() ([]*BeatmapEntry, error) {
	file, err := os.Open(stableDatabasePath())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() <= 0 || info.Size() > stableDBMaxFileBytes {
		return nil, fmt.Errorf("osu!.db has an unsupported size: %d", info.Size())
	}

	return parseStableDatabase(file, info.Size())
}

func parseStableDatabase(source io.Reader, size int64) ([]*BeatmapEntry, error) {
	if source == nil {
		return nil, errors.New("nil osu!.db source")
	}
	if size <= 0 || size > stableDBMaxFileBytes {
		return nil, fmt.Errorf("osu!.db has an unsupported size: %d", size)
	}

	reader := &stableDBReader{reader: io.LimitReader(source, size), remaining: size}
	version, err := reader.readUint32()
	if err != nil {
		return nil, fmt.Errorf("read osu!.db version: %w", err)
	}
	if int(version) < stableDBMinVersion || int(version) > stableDBMaxVersion {
		return nil, fmt.Errorf("unsupported osu!.db version: %d", version)
	}

	if _, err = reader.readUint32(); err != nil { // Folder count.
		return nil, fmt.Errorf("read osu!.db folder count: %w", err)
	}
	if _, err = reader.readByte(); err != nil { // Account unlocked.
		return nil, fmt.Errorf("read osu!.db account state: %w", err)
	}
	if _, err = reader.readUint64(); err != nil { // Unlock date.
		return nil, fmt.Errorf("read osu!.db unlock date: %w", err)
	}
	if _, err = reader.readString(); err != nil { // Player name.
		return nil, fmt.Errorf("read osu!.db player name: %w", err)
	}

	mapCount, err := reader.readCount(stableDBMaxMapCount)
	if err != nil {
		return nil, fmt.Errorf("read osu!.db map count: %w", err)
	}

	entries := make([]*BeatmapEntry, 0, mapCount)
	seenPaths := make(map[string]struct{}, mapCount)
	for i := 0; i < mapCount; i++ {
		entry, err := reader.readStableEntry(int(version))
		if err != nil {
			return nil, fmt.Errorf("read osu!.db map %d: %w", i, err)
		}

		key := entry.MapKey()
		if _, exists := seenPaths[key]; exists {
			return nil, fmt.Errorf("duplicate osu!.db map path: %s", key)
		}
		seenPaths[key] = struct{}{}
		entries = append(entries, entry)
	}

	if _, err = reader.readUint32(); err != nil { // User permissions.
		return nil, fmt.Errorf("read osu!.db permissions: %w", err)
	}
	if reader.remaining != 0 {
		return nil, fmt.Errorf("osu!.db contains %d unexpected trailing bytes", reader.remaining)
	}

	return entries, nil
}

type stableDBReader struct {
	reader    io.Reader
	remaining int64
}

func (r *stableDBReader) readByte() (byte, error) {
	var value [1]byte
	if err := r.readFull(value[:]); err != nil {
		return 0, err
	}
	return value[0], nil
}

func (r *stableDBReader) readUint16() (uint16, error) {
	var value [2]byte
	if err := r.readFull(value[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(value[:]), nil
}

func (r *stableDBReader) readUint32() (uint32, error) {
	var value [4]byte
	if err := r.readFull(value[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(value[:]), nil
}

func (r *stableDBReader) readUint64() (uint64, error) {
	var value [8]byte
	if err := r.readFull(value[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(value[:]), nil
}

func (r *stableDBReader) readFloat32() (float32, error) {
	value, err := r.readUint32()
	return math.Float32frombits(value), err
}

func (r *stableDBReader) readFloat64() (float64, error) {
	value, err := r.readUint64()
	return math.Float64frombits(value), err
}

func (r *stableDBReader) readFull(data []byte) error {
	if int64(len(data)) > r.remaining {
		return io.ErrUnexpectedEOF
	}
	if _, err := io.ReadFull(r.reader, data); err != nil {
		return err
	}
	r.remaining -= int64(len(data))
	return nil
}

func (r *stableDBReader) readString() (string, error) {
	marker, err := r.readByte()
	if err != nil {
		return "", err
	}
	if marker == 0 {
		return "", nil
	}
	if marker != 0x0b {
		return "", fmt.Errorf("invalid string marker 0x%02x", marker)
	}

	length, err := r.readULEB128()
	if err != nil {
		return "", err
	}
	if length > stableDBMaxStringBytes || int64(length) > r.remaining {
		return "", fmt.Errorf("string length %d is too large", length)
	}

	data := make([]byte, int(length))
	if err = r.readFull(data); err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", errors.New("string is not valid UTF-8")
	}

	return string(data), nil
}

func (r *stableDBReader) readULEB128() (uint64, error) {
	var value uint64
	for index := 0; index < 10; index++ {
		part, err := r.readByte()
		if err != nil {
			return 0, err
		}
		if index == 9 && (part > 1 || part&0x80 != 0) {
			return 0, errors.New("ULEB128 value is too large")
		}

		shift := uint(index * 7)
		value |= uint64(part&0x7f) << shift
		if part&0x80 == 0 {
			return value, nil
		}
	}

	return 0, errors.New("ULEB128 value is too large")
}

func (r *stableDBReader) readCount(maximum int) (int, error) {
	value, err := r.readUint32()
	if err != nil {
		return 0, err
	}
	if value > uint32(maximum) {
		return 0, fmt.Errorf("count %d exceeds limit %d", value, maximum)
	}

	return int(value), nil
}

func (r *stableDBReader) readByteOrFloat(version int) (float64, error) {
	if version < 20140609 {
		value, err := r.readByte()
		return float64(value), err
	}

	value, err := r.readFloat32()
	return float64(value), err
}

func (r *stableDBReader) readStableEntry(version int) (*BeatmapEntry, error) {
	if version < 20191106 {
		if _, err := r.readUint32(); err != nil { // Entry size.
			return nil, err
		}
	}

	artist, err := r.readString()
	if err != nil {
		return nil, err
	}
	artistUnicode, err := r.readString()
	if err != nil {
		return nil, err
	}
	title, err := r.readString()
	if err != nil {
		return nil, err
	}
	titleUnicode, err := r.readString()
	if err != nil {
		return nil, err
	}
	creator, err := r.readString()
	if err != nil {
		return nil, err
	}
	difficultyName, err := r.readString()
	if err != nil {
		return nil, err
	}
	audio, err := r.readString()
	if err != nil {
		return nil, err
	}
	md5, err := r.readString()
	if err != nil {
		return nil, err
	}
	osuFile, err := r.readString()
	if err != nil {
		return nil, err
	}
	if _, err = r.readByte(); err != nil { // Ranked status.
		return nil, err
	}
	circles, err := r.readUint16()
	if err != nil {
		return nil, err
	}
	sliders, err := r.readUint16()
	if err != nil {
		return nil, err
	}
	spinners, err := r.readUint16()
	if err != nil {
		return nil, err
	}
	lastModified, err := r.readUint64()
	if err != nil {
		return nil, err
	}
	approachRate, err := r.readByteOrFloat(version)
	if err != nil {
		return nil, err
	}
	circleSize, err := r.readByteOrFloat(version)
	if err != nil {
		return nil, err
	}
	healthDrain, err := r.readByteOrFloat(version)
	if err != nil {
		return nil, err
	}
	overallDifficulty, err := r.readByteOrFloat(version)
	if err != nil {
		return nil, err
	}
	if _, err = r.readFloat64(); err != nil { // Slider velocity.
		return nil, err
	}

	stars := -1.0
	if version >= 20140609 {
		stars, err = r.readStandardStars(version)
		if err != nil {
			return nil, err
		}
		for i := 0; i < 3; i++ {
			if err = r.skipStarRatings(version); err != nil {
				return nil, err
			}
		}
	}

	if _, err = r.readUint32(); err != nil { // Drain time.
		return nil, err
	}
	totalTime, err := r.readUint32()
	if err != nil {
		return nil, err
	}
	previewTime, err := r.readUint32()
	if err != nil {
		return nil, err
	}
	minBPM, maxBPM, err := r.readTimingPoints()
	if err != nil {
		return nil, err
	}
	if _, err = r.readUint32(); err != nil { // Difficulty ID.
		return nil, err
	}
	mapID, err := r.readUint32()
	if err != nil {
		return nil, err
	}
	if _, err = r.readUint32(); err != nil { // Thread ID.
		return nil, err
	}
	for i := 0; i < 4; i++ {
		if _, err = r.readByte(); err != nil { // Grades.
			return nil, err
		}
	}
	localOffset, err := r.readUint16()
	if err != nil {
		return nil, err
	}
	stackLeniency, err := r.readFloat32()
	if err != nil {
		return nil, err
	}
	mode, err := r.readByte()
	if err != nil {
		return nil, err
	}
	source, err := r.readString()
	if err != nil {
		return nil, err
	}
	tags, err := r.readString()
	if err != nil {
		return nil, err
	}
	if _, err = r.readUint16(); err != nil { // Online offset.
		return nil, err
	}
	if _, err = r.readString(); err != nil { // Font.
		return nil, err
	}
	if _, err = r.readByte(); err != nil { // Unplayed.
		return nil, err
	}
	lastPlayed, err := r.readUint64()
	if err != nil {
		return nil, err
	}
	if _, err = r.readByte(); err != nil { // osz2.
		return nil, err
	}
	folder, err := r.readString()
	if err != nil {
		return nil, err
	}
	if _, err = r.readUint64(); err != nil { // Repository check time.
		return nil, err
	}
	for i := 0; i < 5; i++ {
		if _, err = r.readByte(); err != nil { // Ignore skin, storyboard, video, visual override.
			return nil, err
		}
	}
	if version < 20140609 {
		if _, err = r.readUint16(); err != nil {
			return nil, err
		}
	}
	if version < 20140609 {
		if _, err = r.readUint32(); err != nil { // Legacy second modification time.
			return nil, err
		}
	}
	if _, err = r.readByte(); err != nil { // Mania scroll speed.
		return nil, err
	}

	if err = validateStablePath(folder, osuFile); err != nil {
		return nil, err
	}

	entry := &BeatmapEntry{
		Dir:               filepath.ToSlash(folder),
		File:              osuFile,
		Name:              title,
		NameUnicode:       titleUnicode,
		Artist:            artist,
		ArtistUnicode:     artistUnicode,
		Creator:           creator,
		Difficulty:        difficultyName,
		Source:            source,
		Tags:              tags,
		Audio:             audio,
		MD5:               strings.ToLower(md5),
		Mode:              int64(mode),
		ID:                int64(mapID),
		LastModified:      windowsTicksToUnixMillis(lastModified),
		LastPlayed:        windowsTicksToUnixMillis(lastPlayed),
		LocalOffset:       int(localOffset),
		Circles:           int(circles),
		Sliders:           int(sliders),
		Spinners:          int(spinners),
		Length:            int(totalTime),
		PreviewTime:       int64(previewTime),
		Stars:             stars,
		MinBPM:            minBPM,
		MaxBPM:            maxBPM,
		CircleSize:        circleSize,
		ApproachRate:      approachRate,
		HealthDrain:       healthDrain,
		OverallDifficulty: overallDifficulty,
		StackLeniency:     float64(stackLeniency),
		SliderMult:        1,
		SliderTickRate:    1,
		SampleSet:         1,
		FileSize:          -1,
		MetadataState:     MetadataFromStableDatabase,
	}
	entry.prepare()

	return entry, nil
}

func (r *stableDBReader) readStandardStars(version int) (float64, error) {
	count, err := r.readTaggedCount()
	if err != nil {
		return -1, err
	}

	stars := -1.0
	for i := 0; i < count; i++ {
		marker, err := r.readByte()
		if err != nil {
			return -1, err
		}
		if marker != 0x08 {
			return -1, fmt.Errorf("invalid star-rating mod marker 0x%02x", marker)
		}
		mods, err := r.readUint32()
		if err != nil {
			return -1, err
		}
		valueMarker, err := r.readByte()
		if err != nil {
			return -1, err
		}

		var value float64
		switch {
		case version >= 20250107 && valueMarker == 0x0c:
			parsed, readErr := r.readFloat32()
			value = float64(parsed)
			err = readErr
		case version < 20250107 && valueMarker == 0x0d:
			value, err = r.readFloat64()
		default:
			err = fmt.Errorf("invalid star-rating value marker 0x%02x", valueMarker)
		}
		if err != nil {
			return -1, err
		}
		if mods == 0 {
			stars = value
		}
	}

	return stars, nil
}

func (r *stableDBReader) skipStarRatings(version int) error {
	count, err := r.readTaggedCount()
	if err != nil {
		return err
	}

	for i := 0; i < count; i++ {
		marker, err := r.readByte()
		if err != nil {
			return err
		}
		if marker != 0x08 {
			return fmt.Errorf("invalid star-rating mod marker 0x%02x", marker)
		}
		if _, err = r.readUint32(); err != nil {
			return err
		}
		valueMarker, err := r.readByte()
		if err != nil {
			return err
		}
		switch {
		case version >= 20250107 && valueMarker == 0x0c:
			_, err = r.readFloat32()
		case version < 20250107 && valueMarker == 0x0d:
			_, err = r.readFloat64()
		default:
			return fmt.Errorf("invalid star-rating value marker 0x%02x", valueMarker)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *stableDBReader) readTaggedCount() (int, error) {
	marker, err := r.readByte()
	if err != nil {
		return 0, err
	}
	if marker != 0x08 {
		return 0, fmt.Errorf("invalid tagged count marker 0x%02x", marker)
	}

	return r.readCount(stableDBMaxListCount)
}

func (r *stableDBReader) readTimingPoints() (float64, float64, error) {
	count, err := r.readCount(stableDBMaxListCount)
	if err != nil {
		return 0, 0, err
	}

	minBPM := math.Inf(1)
	maxBPM := 0.0
	for i := 0; i < count; i++ {
		bpm, readErr := r.readFloat64()
		if readErr != nil {
			return 0, 0, readErr
		}
		if _, readErr = r.readFloat64(); readErr != nil {
			return 0, 0, readErr
		}
		inherited, readErr := r.readByte()
		if readErr != nil {
			return 0, 0, readErr
		}
		if inherited == 0 && bpm > 0 && !math.IsNaN(bpm) && !math.IsInf(bpm, 0) {
			beatmapBPM := 60000 / bpm
			minBPM = min(minBPM, beatmapBPM)
			maxBPM = max(maxBPM, beatmapBPM)
		}
	}
	if math.IsInf(minBPM, 1) {
		minBPM = 0
	}

	return minBPM, maxBPM, nil
}

func validateStablePath(folder, osuFile string) error {
	if isAbsoluteCatalogPath(folder) || isAbsoluteCatalogPath(osuFile) {
		return errors.New("osu!.db contains an absolute map path")
	}
	if strings.ContainsAny(osuFile, `/\\`) || !strings.EqualFold(filepath.Ext(osuFile), ".osu") {
		return fmt.Errorf("invalid .osu filename %q", osuFile)
	}

	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(folder, "\\", "/")))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		if folder != "" {
			return fmt.Errorf("invalid map folder %q", folder)
		}
	}

	return nil
}

func windowsTicksToUnixMillis(ticks uint64) int64 {
	if ticks <= windowsTicksToUnixEpoch {
		return 0
	}

	return int64((ticks - windowsTicksToUnixEpoch) / 10_000)
}
