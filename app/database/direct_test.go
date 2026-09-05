package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

const directTestOsu = `osu file format v14

[General]
AudioFilename: audio.mp3
Mode: 0

[Metadata]
Title:Direct Test
Artist:Test Artist
Creator:Test Creator
Version:Hard

[Difficulty]
HPDrainRate:5
CircleSize:4
OverallDifficulty:7
ApproachRate:9
SliderMultiplier:1.4
SliderTickRate:1

[TimingPoints]
1000,500,4,2,1,60,1,0

[HitObjects]
256,192,1000,1,0,0:0:0:0:
256,192,2000,1,0,0:0:0:0:
`

func setupDirectSongsDir(t *testing.T, modeLine string) string {
	t.Helper()

	root := t.TempDir()
	setDir := filepath.Join(root, "12345 Artist - Title")
	if err := os.MkdirAll(setDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := directTestOsu
	if modeLine != "" {
		content = "osu file format v14\n\n[General]\n" + modeLine + "\n\n[Metadata]\nTitle:Direct Test\nArtist:Test Artist\nCreator:Test Creator\nVersion:Hard\n\n[TimingPoints]\n1000,500,4,2,1,60,1,0\n"
	}
	if err := os.WriteFile(filepath.Join(setDir, "map.osu"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	previousSongsDir := songsDir
	songsDir = root
	t.Cleanup(func() { songsDir = previousSongsDir })

	previousSettingsDir := settings.General.OsuSongsDir
	settings.General.OsuSongsDir = root
	t.Cleanup(func() { settings.General.OsuSongsDir = previousSettingsDir })

	return filepath.Join("12345 Artist - Title", "map.osu")
}

func TestLoadDirectBeatmapParsesWithoutCatalogScan(t *testing.T) {
	relativePath := setupDirectSongsDir(t, "")

	bMap, err := LoadDirectBeatmap(relativePath)
	if err != nil {
		t.Fatalf("LoadDirectBeatmap() error = %v", err)
	}

	if bMap.Name != "Direct Test" || bMap.Artist != "Test Artist" || bMap.Difficulty != "Hard" {
		t.Fatalf("direct beatmap metadata = %q/%q/%q, want Direct Test/Test Artist/Hard", bMap.Name, bMap.Artist, bMap.Difficulty)
	}
	if bMap.MD5 == "" {
		t.Fatal("direct beatmap is missing its content hash")
	}
	if bMap.LastModified <= 0 || bMap.FileSize <= 0 {
		t.Fatalf("direct beatmap fingerprint = %d/%d, want positive modification time and size", bMap.LastModified, bMap.FileSize)
	}
	if bMap.File != "map.osu" {
		t.Fatalf("direct beatmap file = %q, want map.osu", bMap.File)
	}
}

func TestLoadDirectBeatmapRejectsUnsafePaths(t *testing.T) {
	setupDirectSongsDir(t, "")

	for _, path := range []string{"", "../outside.osu", "set/../../outside.osu", "set/..", "/absolute/map.osu", "C:/songs/map.osu"} {
		if _, err := LoadDirectBeatmap(path); err == nil {
			t.Fatalf("LoadDirectBeatmap(%q) succeeded, want a path validation error", path)
		}
	}
}

func TestLoadDirectBeatmapRejectsMissingFiles(t *testing.T) {
	relativePath := setupDirectSongsDir(t, "")
	missing := filepath.Join(filepath.Dir(relativePath), "absent.osu")

	if _, err := LoadDirectBeatmap(missing); err == nil {
		t.Fatalf("LoadDirectBeatmap(%q) succeeded, want a read error", missing)
	}
}

func TestLoadDirectBeatmapRejectsUnsupportedModes(t *testing.T) {
	relativePath := setupDirectSongsDir(t, "Mode: 1")

	if _, err := LoadDirectBeatmap(relativePath); err == nil {
		t.Fatalf("LoadDirectBeatmap(%q) succeeded for a non-standard mode, want an error", relativePath)
	}
}

func TestLoadDirectBeatmapRequiresInitialization(t *testing.T) {
	previous := songsDir
	songsDir = ""
	t.Cleanup(func() { songsDir = previous })

	if _, err := LoadDirectBeatmap("set/map.osu"); err == nil {
		t.Fatal("LoadDirectBeatmap() succeeded without initialization, want an error")
	}
}

func setupDirectLookupDatabase(t *testing.T) {
	t.Helper()

	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	previousDatabase := dbFile
	dbFile = database
	t.Cleanup(func() { dbFile = previousDatabase })

	_, err = database.Exec(`CREATE TABLE beatmaps (
		dir TEXT, file TEXT, lastModified INTEGER, title TEXT, titleUnicode TEXT,
		artist TEXT, artistUnicode TEXT, creator TEXT, version TEXT, source TEXT,
		tags TEXT, cs REAL, ar REAL, sliderMultiplier REAL, sliderTickRate REAL,
		audioFile TEXT, previewTime INTEGER, sampleSet INTEGER, stackLeniency REAL,
		mode INTEGER, bg TEXT, md5 TEXT, dateAdded INTEGER, playCount INTEGER,
		lastPlayed INTEGER, hpdrain REAL, od REAL, stars REAL DEFAULT -1,
		bpmMin REAL, bpmMax REAL, circles INTEGER, sliders INTEGER, spinners INTEGER,
		endTime INTEGER, setID INTEGER, mapID INTEGER, starsVersion INTEGER DEFAULT 0,
		localOffset INTEGER DEFAULT 0, fileSize INTEGER DEFAULT 0,
		metadataState INTEGER DEFAULT 0
	)`)
	if err != nil {
		t.Fatal(err)
	}

	entry := &BeatmapEntry{
		Dir:         "12345 Artist - Title",
		File:        "map.osu",
		Name:        "Direct Test",
		Mode:        0,
		MD5:         "lookup-md5",
		TimeAdded:   11,
		PlayCount:   3,
		LastPlayed:  22,
		LocalOffset: 5,
	}
	if err := upsertCatalogEntries([]*BeatmapEntry{entry}); err != nil {
		t.Fatalf("seed lookup row: %v", err)
	}
}

func TestLookupBeatmapEntryFindsCommittedRow(t *testing.T) {
	setupDirectLookupDatabase(t)

	entry := LookupBeatmapEntry("12345 artist - title", "MAP.OSU")
	if entry == nil {
		t.Fatal("LookupBeatmapEntry() = nil, want the seeded row")
	}
	if entry.MD5 != "lookup-md5" || entry.PlayCount != 3 || entry.LocalOffset != 5 {
		t.Fatalf("lookup row = md5 %q plays %d offset %d, want lookup-md5/3/5", entry.MD5, entry.PlayCount, entry.LocalOffset)
	}
}

func TestLookupBeatmapEntryMissesWithoutFailing(t *testing.T) {
	setupDirectLookupDatabase(t)

	if entry := LookupBeatmapEntry("12345 Artist - Title", "absent.osu"); entry != nil {
		t.Fatalf("LookupBeatmapEntry() = %#v, want nil for a missing row", entry)
	}
	if entry := LookupBeatmapEntry("12345 Artist - Title", ""); entry != nil {
		t.Fatalf("LookupBeatmapEntry() = %#v, want nil for an empty filename", entry)
	}
}

func TestLookupBeatmapEntryWithoutDatabase(t *testing.T) {
	previousDatabase := dbFile
	dbFile = nil
	t.Cleanup(func() { dbFile = previousDatabase })

	if entry := LookupBeatmapEntry("set", "map.osu"); entry != nil {
		t.Fatalf("LookupBeatmapEntry() = %#v, want nil without a database", entry)
	}
}
