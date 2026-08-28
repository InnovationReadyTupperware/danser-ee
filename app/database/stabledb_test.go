package database

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestParseStableDatabaseCurrentFixture(t *testing.T) {
	data := buildStableDatabaseFixture()

	entries, err := parseStableDatabase(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parseStableDatabase() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("parseStableDatabase() returned %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Dir != "Set" || entry.File != "map.osu" {
		t.Fatalf("entry path = %q/%q, want Set/map.osu", entry.Dir, entry.File)
	}
	if entry.Name != "Title" || entry.Artist != "Artist" || entry.Creator != "Creator" {
		t.Fatalf("entry metadata was not decoded: %+v", entry)
	}
	if entry.ID != 42 || entry.Mode != 0 || entry.Length != 1234 || entry.PreviewTime != 567 {
		t.Fatalf("entry scalar metadata was not decoded: %+v", entry)
	}
	if entry.LastModified != 1 || entry.LastPlayed != 2 {
		t.Fatalf("entry timestamps = %d/%d, want 1/2", entry.LastModified, entry.LastPlayed)
	}
	if entry.MinBPM != 120 || entry.MaxBPM != 120 {
		t.Fatalf("entry BPM = %f/%f, want 120/120", entry.MinBPM, entry.MaxBPM)
	}
	if entry.MetadataState != MetadataFromStableDatabase || entry.FileSize != -1 {
		t.Fatalf("entry source state = %d/%d, want Stable/-1", entry.MetadataState, entry.FileSize)
	}
}

func TestParseStableDatabaseRejectsTrailingData(t *testing.T) {
	data := append(buildStableDatabaseFixture(), 0x01)

	if _, err := parseStableDatabase(bytes.NewReader(data), int64(len(data))); err == nil {
		t.Fatal("parseStableDatabase() accepted trailing data")
	}
}

func TestParseStableDatabaseOldFormat(t *testing.T) {
	data := buildStableDatabaseFixtureVersion(20140608)

	entries, err := parseStableDatabase(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parseStableDatabase() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("parseStableDatabase() returned %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.ApproachRate != 9 || entry.CircleSize != 4 || entry.HealthDrain != 5 || entry.OverallDifficulty != 6 {
		t.Fatalf("old difficulty values = %v/%v/%v/%v, want 9/4/5/6", entry.ApproachRate, entry.CircleSize, entry.HealthDrain, entry.OverallDifficulty)
	}
	if entry.Stars != -1 {
		t.Fatalf("old database unexpectedly supplied star rating %v", entry.Stars)
	}
}

func TestStableDBReaderRejectsOversizedULEB128(t *testing.T) {
	reader := &stableDBReader{
		reader:    bytes.NewReader(bytes.Repeat([]byte{0x80}, 10)),
		remaining: 10,
	}

	if _, err := reader.readULEB128(); err == nil {
		t.Fatal("readULEB128() accepted an oversized value")
	}
}

func buildStableDatabaseFixture() []byte {
	return buildStableDatabaseFixtureVersion(stableDBMaxVersion)
}

func buildStableDatabaseFixtureVersion(version int) []byte {
	var data bytes.Buffer
	writeStableUint32(&data, uint32(version))
	writeStableUint32(&data, 1)
	data.WriteByte(1)
	writeStableUint64(&data, 0)
	writeStableString(&data, "player")
	writeStableUint32(&data, 1)

	if version < 20191106 {
		writeStableUint32(&data, 0)
	}

	writeStableString(&data, "Artist")
	writeStableString(&data, "")
	writeStableString(&data, "Title")
	writeStableString(&data, "")
	writeStableString(&data, "Creator")
	writeStableString(&data, "Difficulty")
	writeStableString(&data, "audio.mp3")
	writeStableString(&data, "0123456789abcdef0123456789abcdef")
	writeStableString(&data, "map.osu")
	data.WriteByte(0)
	writeStableUint16(&data, 10)
	writeStableUint16(&data, 20)
	writeStableUint16(&data, 1)
	writeStableUint64(&data, windowsTicksToUnixEpoch+10_000)
	if version < 20140609 {
		data.Write([]byte{9, 4, 5, 6})
	} else {
		writeStableFloat32(&data, 9)
		writeStableFloat32(&data, 4)
		writeStableFloat32(&data, 5)
		writeStableFloat32(&data, 6)
	}
	writeStableFloat64(&data, 1.4)
	if version >= 20140609 {
		writeStableEmptyRatings(&data)
		writeStableEmptyRatings(&data)
		writeStableEmptyRatings(&data)
		writeStableEmptyRatings(&data)
	}
	writeStableUint32(&data, 100)
	writeStableUint32(&data, 1234)
	writeStableUint32(&data, 567)
	writeStableUint32(&data, 1)
	writeStableFloat64(&data, 500)
	writeStableFloat64(&data, 0)
	data.WriteByte(0)
	writeStableUint32(&data, 0)
	writeStableUint32(&data, 42)
	writeStableUint32(&data, 0)
	data.Write([]byte{0, 0, 0, 0})
	writeStableUint16(&data, 12)
	writeStableFloat32(&data, 0.7)
	data.WriteByte(0)
	writeStableString(&data, "source")
	writeStableString(&data, "tag")
	writeStableUint16(&data, 0)
	writeStableString(&data, "")
	data.WriteByte(0)
	writeStableUint64(&data, windowsTicksToUnixEpoch+20_000)
	data.WriteByte(0)
	writeStableString(&data, "Set")
	writeStableUint64(&data, 0)
	data.Write([]byte{0, 0, 0, 0, 0})
	if version < 20140609 {
		writeStableUint16(&data, 0)
		writeStableUint32(&data, 0)
	}
	data.WriteByte(0)
	writeStableUint32(&data, 0)

	return data.Bytes()
}

func writeStableEmptyRatings(data *bytes.Buffer) {
	data.WriteByte(0x08)
	writeStableUint32(data, 0)
}

func writeStableString(data *bytes.Buffer, value string) {
	if value == "" {
		data.WriteByte(0)
		return
	}

	data.WriteByte(0x0b)
	writeStableULEB128(data, uint64(len(value)))
	data.WriteString(value)
}

func writeStableULEB128(data *bytes.Buffer, value uint64) {
	for value >= 0x80 {
		data.WriteByte(byte(value) | 0x80)
		value >>= 7
	}
	data.WriteByte(byte(value))
}

func writeStableUint16(data *bytes.Buffer, value uint16) {
	var encoded [2]byte
	binary.LittleEndian.PutUint16(encoded[:], value)
	data.Write(encoded[:])
}

func writeStableUint32(data *bytes.Buffer, value uint32) {
	var encoded [4]byte
	binary.LittleEndian.PutUint32(encoded[:], value)
	data.Write(encoded[:])
}

func writeStableUint64(data *bytes.Buffer, value uint64) {
	var encoded [8]byte
	binary.LittleEndian.PutUint64(encoded[:], value)
	data.Write(encoded[:])
}

func writeStableFloat32(data *bytes.Buffer, value float32) {
	writeStableUint32(data, math.Float32bits(value))
}

func writeStableFloat64(data *bytes.Buffer, value float64) {
	writeStableUint64(data, math.Float64bits(value))
}
