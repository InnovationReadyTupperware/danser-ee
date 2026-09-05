package database

import (
	"context"
	"database/sql"
	"testing"
)

func TestUpsertCatalogEntriesPreservesLocalState(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	previousDatabase := dbFile
	dbFile = database
	defer func() { dbFile = previousDatabase }()

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
		metadataState INTEGER DEFAULT 0,
		UNIQUE(dir COLLATE NOCASE, file COLLATE NOCASE)
	)`)
	if err != nil {
		t.Fatal(err)
	}

	first := &BeatmapEntry{
		Dir:         "set",
		File:        "map.osu",
		Name:        "before",
		Mode:        0,
		TimeAdded:   10,
		PlayCount:   4,
		LastPlayed:  20,
		LocalOffset: 7,
	}
	if err := upsertCatalogEntries([]*BeatmapEntry{first}); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	second := &BeatmapEntry{
		Dir:          "SET",
		File:         "MAP.OSU",
		Name:         "after",
		Mode:         0,
		TimeAdded:    999,
		PlayCount:    999,
		LastPlayed:   999,
		LocalOffset:  999,
		LastModified: 30,
		FileSize:     40,
	}
	if err := upsertCatalogEntries([]*BeatmapEntry{second}); err != nil {
		t.Fatalf("replacement upsert: %v", err)
	}

	var name string
	var dateAdded, playCount, lastPlayed, localOffset int64
	if err := database.QueryRow(`SELECT title, dateAdded, playCount, lastPlayed, localOffset
		FROM beatmaps WHERE dir = ? COLLATE NOCASE AND file = ? COLLATE NOCASE`, first.Dir, first.File).
		Scan(&name, &dateAdded, &playCount, &lastPlayed, &localOffset); err != nil {
		t.Fatal(err)
	}

	if name != "after" || dateAdded != 10 || playCount != 4 || lastPlayed != 20 || localOffset != 7 {
		t.Fatalf("upsert changed preserved state: %q %d %d %d %d", name, dateAdded, playCount, lastPlayed, localOffset)
	}
}

func TestEnsureCatalogLocationIndexRepairsCaseVariants(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	previousDatabase := dbFile
	dbFile = database
	defer func() { dbFile = previousDatabase }()

	if _, err = database.Exec(`CREATE TABLE beatmaps (dir TEXT, file TEXT);
		INSERT INTO beatmaps (dir, file) VALUES ('Set', 'map.osu'), ('set', 'MAP.OSU');`); err != nil {
		t.Fatal(err)
	}

	if err = ensureCatalogLocationIndex(); err != nil {
		t.Fatalf("ensureCatalogLocationIndex() error = %v", err)
	}

	var count int
	if err = database.QueryRow("SELECT COUNT(*) FROM beatmaps").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("case-variant rows = %d, want 1", count)
	}

	var unique int
	if err = database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_beatmaps_location_nocase'").Scan(&unique); err != nil {
		t.Fatal(err)
	}
	if unique != 1 {
		t.Fatal("case-insensitive location index was not created")
	}
}

func TestEnsureCatalogColumnsCanBeRetried(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	previousDatabase := dbFile
	dbFile = database
	defer func() { dbFile = previousDatabase }()

	if _, err = database.Exec("CREATE TABLE beatmaps (dir TEXT, file TEXT)"); err != nil {
		t.Fatal(err)
	}

	if err = ensureCatalogColumns(); err != nil {
		t.Fatalf("first ensureCatalogColumns() error = %v", err)
	}
	if err = ensureCatalogColumns(); err != nil {
		t.Fatalf("retry ensureCatalogColumns() error = %v", err)
	}

	rows, err := database.Query("PRAGMA table_info(beatmaps)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var (
			columnID   int
			columnName string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err = rows.Scan(&columnID, &columnName, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns[columnName] = true
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !columns["fileSize"] || !columns["metadataState"] {
		t.Fatalf("catalog columns = %#v, want fileSize and metadataState", columns)
	}
}

func TestInvalidCatalogEntriesStayHidden(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	previousDatabase := dbFile
	dbFile = database
	defer func() { dbFile = previousDatabase }()

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
		metadataState INTEGER DEFAULT 0,
		UNIQUE(dir COLLATE NOCASE, file COLLATE NOCASE)
	)`)
	if err != nil {
		t.Fatal(err)
	}

	valid := &BeatmapEntry{
		Dir:           "set",
		File:          "map.osu",
		Name:          "playable",
		Mode:          0,
		Stars:         5,
		MetadataState: MetadataComplete,
	}
	tombstone := &BeatmapEntry{
		Dir:           "broken",
		File:          "broken.osu",
		Mode:          0,
		Stars:         -1,
		LastModified:  123,
		FileSize:      456,
		MetadataState: MetadataInvalid,
	}
	if err := upsertCatalogEntries([]*BeatmapEntry{valid, tombstone}); err != nil {
		t.Fatalf("upsert tombstone fixtures: %v", err)
	}

	// Tombstones for unparseable files must never surface in song select.
	if entries := loadCatalogEntries(); len(entries) != 1 || entries[0].File != "map.osu" {
		t.Fatalf("catalog entries = %#v, want only the playable map", entries)
	}

	// The fingerprint must still be visible so the next scan skips the file
	// without reparsing it.
	_, fingerprints, complete := getLastModifiedContext(context.Background())
	if !complete {
		t.Fatal("fingerprint load was marked incomplete")
	}
	cached, ok := fingerprints[catalogPathKey("broken", "broken.osu")]
	if !ok {
		t.Fatal("tombstone fingerprint is missing")
	}
	if cached.fingerprint.modified != 123 || cached.fingerprint.size != 456 ||
		cached.fingerprint.metadataState != MetadataInvalid {
		t.Fatalf("tombstone fingerprint = %+v, want recorded values", cached.fingerprint)
	}
}

func setupRemoveCatalogLocationDatabase(t *testing.T) {
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
		metadataState INTEGER DEFAULT 0,
		UNIQUE(dir COLLATE NOCASE, file COLLATE NOCASE)
	)`)
	if err != nil {
		t.Fatal(err)
	}

	seed := []*BeatmapEntry{
		{Dir: "gone", File: "map.osu", Name: "Ghost", Mode: 0},
		{Dir: "kept", File: "map.osu", Name: "Kept", Mode: 0},
	}
	if err := upsertCatalogEntries(seed); err != nil {
		t.Fatalf("seed removal fixtures: %v", err)
	}
}

func TestRemoveCatalogLocationDeletesOneRow(t *testing.T) {
	setupRemoveCatalogLocationDatabase(t)

	if err := RemoveCatalogLocation("GONE", "MAP.OSU"); err != nil {
		t.Fatalf("RemoveCatalogLocation() error = %v", err)
	}

	entries := loadCatalogEntries()
	if len(entries) != 1 || entries[0].Name != "Kept" {
		t.Fatalf("catalog entries = %#v, want only the kept map", entries)
	}
}

func TestRemoveCatalogLocationMissesWithoutFailing(t *testing.T) {
	setupRemoveCatalogLocationDatabase(t)

	if err := RemoveCatalogLocation("gone", "absent.osu"); err != nil {
		t.Fatalf("RemoveCatalogLocation() error = %v, want nil for a missing row", err)
	}
	if entries := loadCatalogEntries(); len(entries) != 2 {
		t.Fatalf("catalog entries = %d, want both rows after a miss", len(entries))
	}
}

func TestRemoveCatalogLocationRejectsBadInput(t *testing.T) {
	setupRemoveCatalogLocationDatabase(t)

	if err := RemoveCatalogLocation("gone", ""); err == nil {
		t.Fatal("RemoveCatalogLocation() succeeded with an empty filename, want an error")
	}

	previousDatabase := dbFile
	dbFile = nil
	t.Cleanup(func() { dbFile = previousDatabase })

	if err := RemoveCatalogLocation("gone", "map.osu"); err == nil {
		t.Fatal("RemoveCatalogLocation() succeeded without a database, want an error")
	}
}

func TestNotifyCatalogDeltaPublishesOnlyChanges(t *testing.T) {
	entry := &BeatmapEntry{Dir: "set", File: "map.osu"}
	var received []CatalogDelta
	listener := func(delta CatalogDelta) {
		received = append(received, delta)
	}

	notifyCatalogDelta(listener, CatalogDelta{})
	notifyCatalogDelta(nil, CatalogDelta{Upserts: []*BeatmapEntry{entry}})
	notifyCatalogDelta(listener, CatalogDelta{Upserts: []*BeatmapEntry{entry}})
	notifyCatalogDelta(listener, CatalogDelta{Removals: []string{"removed/map.osu"}})

	if len(received) != 2 {
		t.Fatalf("listener received %d deltas, want 2", len(received))
	}
	if len(received[0].Upserts) != 1 || received[0].Upserts[0] != entry {
		t.Fatalf("upsert delta = %#v, want entry", received[0])
	}
	if len(received[1].Removals) != 1 || received[1].Removals[0] != "removed/map.osu" {
		t.Fatalf("removal delta = %#v, want removed map key", received[1])
	}
}
