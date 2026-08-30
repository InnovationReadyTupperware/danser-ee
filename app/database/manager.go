package database

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
	"github.com/innovationreadytupperware/danser-ee/app/rulesets/osu/performance/pp250306"
	"github.com/innovationreadytupperware/danser-ee/app/settings"
	"github.com/innovationreadytupperware/danser-ee/app/utils"
	"github.com/innovationreadytupperware/danser-ee/framework/env"
	"github.com/innovationreadytupperware/danser-ee/framework/files"
	"github.com/innovationreadytupperware/danser-ee/framework/goroutines"
	"github.com/innovationreadytupperware/danser-ee/framework/math/mutils"
	"github.com/innovationreadytupperware/danser-ee/framework/util"
)

var dbFile *sql.DB

const databaseVersion = 20260828

var currentPreVersion = databaseVersion
var currentSchemaPreVersion = databaseVersion
var catalogSourceMatches = true

type mapLocation struct {
	dir  string
	file string
}

func (location mapLocation) key() string {
	return catalogPathKey(location.dir, location.file)
}

type cachedMap struct {
	location    mapLocation
	fingerprint fileFingerprint
	localState  catalogLocalState
}

type fileFingerprint struct {
	modified      int64
	size          int64
	metadataState MetadataState
}

type catalogLocalState struct {
	timeAdded   int64
	playCount   int64
	lastPlayed  int64
	localOffset int
}

type modMap struct {
	location    mapLocation
	fingerprint fileFingerprint
	localState  catalogLocalState
}

var migrations []Migration

var songsDir string

var difficultyCalc = pp250306.NewDifficultyCalculator()

func Init() (err error) {
	log.Println("DatabaseManager: Initializing database...")
	defer func() {
		if err != nil {
			// A partially initialized manager must not leave a connection behind
			// for the next launcher reload or hide the original failure.
			Close()
		}
	}()

	songsDir, err = filepath.Abs(settings.General.GetSongsDir())
	if err != nil {
		return fmt.Errorf("invalid song path given: %s", settings.General.GetSongsDir())
	}

	// Do not make the source directory a prerequisite for opening danser's
	// catalog. Cache-first startup must still expose the last known-good rows
	// when a removable drive is disconnected or the directory is temporarily
	// unavailable. The reconciliation scanner performs the authoritative
	// source check later and refuses cleanup after an incomplete scan.
	if info, statErr := os.Stat(songsDir); statErr != nil {
		log.Printf("DatabaseManager: Songs directory is unavailable; cached catalog may still be used: %v", statErr)
	} else if !info.IsDir() {
		log.Printf("DatabaseManager: Songs path is not a directory; cached catalog may still be used: %s", songsDir)
	}

	migrations = []Migration{
		&M20181111{},
		&M20201027{},
		&M20201112{},
		&M20201117{},
		&M20201118{},
		&M20210104{},
		&M20210326{},
		&M20210423{},
		&M20220605{},
		&M20220622{},
		&M20260828{},
	}

	dbFile, err = sql.Open("sqlite3", filepath.Join(env.DataDir(), "danser.db"))
	if err != nil {
		return err
	}

	var beatmapsTableExisted bool
	if err = dbFile.QueryRow("SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'beatmaps')").Scan(&beatmapsTableExisted); err != nil {
		return fmt.Errorf("cannot inspect beatmap catalog table: %w", err)
	}

	dbFile.SetMaxOpenConns(1)
	dbFile.SetMaxIdleConns(1)

	_, err = dbFile.Exec(`
		PRAGMA busy_timeout = 5000;
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		CREATE TABLE IF NOT EXISTS beatmaps (dir TEXT, file TEXT, lastModified INTEGER, title TEXT, titleUnicode TEXT, artist TEXT, artistUnicode TEXT, creator TEXT, version TEXT, source TEXT, tags TEXT, cs REAL, ar REAL, sliderMultiplier REAL, sliderTickRate REAL, audioFile TEXT, previewTime INTEGER, sampleSet INTEGER, stackLeniency REAL, mode INTEGER, bg TEXT, md5 TEXT, dateAdded INTEGER, playCount INTEGER, lastPlayed INTEGER, hpdrain REAL, od REAL, stars REAL DEFAULT -1, bpmMin REAL, bpmMax REAL, circles INTEGER, sliders INTEGER, spinners INTEGER, endTime INTEGER, setID INTEGER, mapID INTEGER, starsVersion INTEGER DEFAULT 0, localOffset INTEGER DEFAULT 0, fileSize INTEGER DEFAULT 0, metadataState INTEGER DEFAULT 0);
		CREATE INDEX IF NOT EXISTS idx ON beatmaps (dir, file);
		CREATE TABLE IF NOT EXISTS info (key TEXT NOT NULL UNIQUE, value TEXT);
	`)

	if err != nil {
		dbFile.Close()
		dbFile = nil
		return err
	}

	dataVersionExists := false
	schemaVersionExists := false
	storedSongsDir := ""
	if beatmapsTableExisted {
		currentPreVersion = 0
		currentSchemaPreVersion = 0
	} else {
		currentPreVersion = databaseVersion
		currentSchemaPreVersion = databaseVersion
	}
	catalogSourceMatches = true

	res, err := dbFile.Query("SELECT key, value FROM info")
	if err != nil {
		dbFile.Close()
		dbFile = nil
		return err
	}
	defer res.Close()

	for res.Next() {
		var key, value string

		err = res.Scan(&key, &value)
		if err != nil {
			return err
		}

		if key == "version" {
			currentPreVersion, err = strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid database data version %q: %w", value, err)
			}
			dataVersionExists = true
		}

		if key == "schema_version" {
			schemaVersionExists = true
			currentSchemaPreVersion, err = strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid database schema version %q: %w", value, err)
			}
		}

		if key == "songs_dir" {
			storedSongsDir = value
		}
	}
	if err = res.Err(); err != nil {
		return fmt.Errorf("read database information: %w", err)
	}
	if err = res.Close(); err != nil {
		return fmt.Errorf("close database information: %w", err)
	}
	if storedSongsDir != "" {
		catalogSourceMatches = sameCatalogSource(storedSongsDir, songsDir)
		if !catalogSourceMatches {
			log.Println("DatabaseManager: Songs directory changed; cached catalog will remain hidden until reconciliation completes.")
		}
	}

	if beatmapsTableExisted && !dataVersionExists && !schemaVersionExists {
		inferredVersion, inferErr := inferCatalogVersion()
		if inferErr != nil {
			return inferErr
		}
		currentPreVersion = inferredVersion
		currentSchemaPreVersion = inferredVersion
	} else if !dataVersionExists {
		currentPreVersion = currentSchemaPreVersion
	} else if !schemaVersionExists {
		currentSchemaPreVersion = currentPreVersion
	}

	log.Println("DatabaseManager: Database schema version:", currentSchemaPreVersion)
	log.Println("DatabaseManager: Database data version:", currentPreVersion)
	if currentSchemaPreVersion > databaseVersion || currentPreVersion > databaseVersion {
		return fmt.Errorf("database version %d/%d is newer than supported version %d", currentSchemaPreVersion, currentPreVersion, databaseVersion)
	}

	if currentSchemaPreVersion != databaseVersion {
		log.Println("DatabaseManager: Database schema is too old! Updating...")

		var statement strings.Builder

		for _, m := range migrations {
			if currentSchemaPreVersion < m.Date() {
				statement.WriteString(m.GetMigrationStmts())
			}
		}

		if statement.Len() > 0 {
			_, err = dbFile.Exec(statement.String())
			if err != nil {
				return fmt.Errorf("cannot migrate database schema: %w", err)
			}
		}

		log.Println("DatabaseManager: Schema has been updated!")
	}

	if err = ensureCatalogColumns(); err != nil {
		return err
	}

	_, err = dbFile.Exec("REPLACE INTO info (key, value) VALUES ('schema_version', ?)", strconv.FormatInt(databaseVersion, 10))
	if err != nil {
		return err
	}

	if currentPreVersion != databaseVersion {
		if err = migrateBeatmaps(); err != nil {
			return err
		}
	}

	if err = ensureCatalogLocationIndex(); err != nil {
		return err
	}

	_, err = dbFile.Exec("REPLACE INTO info (key, value) VALUES ('version', ?)", strconv.FormatInt(databaseVersion, 10))
	if err != nil {
		return err
	}

	return nil
}

func LoadBeatmaps(skipDatabaseCheck bool, importListener ImportListener) []*beatmap.BeatMap {
	entries := LoadCatalog(skipDatabaseCheck, importListener)
	beatmaps := make([]*beatmap.BeatMap, 0, len(entries))
	for _, entry := range entries {
		beatmaps = append(beatmaps, entry.NewBeatMap())
	}

	return beatmaps
}

// LoadRuntimeBeatMap validates and parses one catalog entry on demand. The
// launcher keeps only metadata for search, so this is the boundary at which a
// selected map is allowed to touch its backing .osu file.
func LoadRuntimeBeatMap(entry *BeatmapEntry) (*beatmap.BeatMap, error) {
	return loadRuntimeBeatMap(entry, true)
}

func loadRuntimeBeatMap(entry *BeatmapEntry, verifyContent bool) (*beatmap.BeatMap, error) {
	if entry == nil {
		return nil, fmt.Errorf("nil beatmap catalog entry")
	}

	mapPath, err := catalogEntryPath(entry)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(mapPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	sourceModified := fileInfo.ModTime().UnixNano() / int64(1e6)
	sourceSize := fileInfo.Size()
	fingerprintKnown := entry.LastModified > 0 && entry.FileSize > 0
	changed := fingerprintKnown &&
		(entry.LastModified != sourceModified || entry.FileSize != sourceSize)

	bMap, err := beatmap.ParseBeatMapFileWithError(file)
	if err != nil {
		return nil, err
	}
	// The parser derives a path from the opened file for standalone callers.
	// Catalog callers must retain the catalog identity instead: a case-only
	// rename on a case-sensitive filesystem must not make later local-stat or
	// star-rating updates miss their database row.
	bMap.Dir = entry.Dir
	bMap.File = entry.File
	if bMap.Mode != 0 {
		return nil, fmt.Errorf("beatmap mode %d is not supported", bMap.Mode)
	}

	bMap.LastModified = sourceModified
	bMap.FileSize = sourceSize
	if verifyContent || changed || entry.MD5 == "" || !fingerprintKnown {
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		bMap.MD5, err = hashBeatmapFile(file)
		if err != nil {
			return nil, err
		}
		if entry.MD5 != "" && !strings.EqualFold(entry.MD5, bMap.MD5) {
			changed = true
		}
	} else {
		bMap.MD5 = entry.MD5
	}
	if changed {
		// A source edit invalidates the cached difficulty calculation. The
		// background reconciliation will replace the catalog row and can
		// calculate a fresh rating later.
		bMap.Stars = -1
		bMap.StarsVersion = 0
	}

	// Local play statistics and star-rating state belong to danser's catalog,
	// not to the source file. Preserve them after parsing the authoritative
	// gameplay metadata.
	bMap.TimeAdded = entry.TimeAdded
	bMap.PlayCount = entry.PlayCount
	bMap.LastPlayed = entry.LastPlayed
	bMap.LocalOffset = entry.LocalOffset
	if !changed {
		bMap.Stars = entry.Stars
		bMap.StarsVersion = entry.StarsVersion
	}

	return bMap, nil
}

func hashBeatmapFile(file io.Reader) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func catalogEntryPath(entry *BeatmapEntry) (string, error) {
	if entry.File == "" || isAbsoluteCatalogPath(entry.Dir) || isAbsoluteCatalogPath(entry.File) {
		return "", fmt.Errorf("invalid beatmap path %q/%q", entry.Dir, entry.File)
	}
	if filepath.Base(entry.File) != entry.File || strings.ContainsAny(entry.File, `/\`) {
		return "", fmt.Errorf("invalid beatmap filename %q", entry.File)
	}

	directory := strings.ReplaceAll(entry.Dir, "\\", "/")
	if strings.HasPrefix(directory, "/") || (len(directory) >= 2 && directory[1] == ':') {
		return "", fmt.Errorf("invalid beatmap directory %q", entry.Dir)
	}
	relativePath := filepath.Clean(filepath.Join(filepath.FromSlash(directory), entry.File))
	if relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid beatmap directory %q", entry.Dir)
	}

	return filepath.Join(songsDir, relativePath), nil
}

// LoadCatalog imports source changes and returns lightweight standard-mode
// metadata. Runtime beatmap state is materialized only by callers that need
// to play, preview, or calculate a map.
func LoadCatalog(skipDatabaseCheck bool, importListener ImportListener) []*BeatmapEntry {
	if dbFile == nil {
		log.Println("DatabaseManager: Cannot load catalog before initialization")
		return nil
	}

	// The command-line path is intentionally synchronous for compatibility.
	// The launcher performs the same optional seed on its reconciliation worker,
	// so parsing a large accelerator never delays warm launcher startup.
	SeedCatalogFromStableDatabase()

	var unpackedMaps []string
	if settings.General.UnpackOszFiles {
		unpackedMaps = unpackMaps()
	}

	importMaps(skipDatabaseCheck, unpackedMaps, importListener)

	log.Println("DatabaseManager: Loading beatmaps from database...")

	stdMaps := loadCatalogEntries()

	log.Println("DatabaseManager: Loaded", len(stdMaps), "total.")

	return stdMaps
}

// LoadCachedCatalog loads danser's last committed standard-mode catalog
// without touching the Songs directory. It is the cache-first startup path.
func LoadCachedCatalog() *CatalogSnapshot {
	catalog, err := LoadCachedCatalogContext(context.Background())
	if err != nil {
		log.Println("DatabaseManager: Failed to load cached catalog:", err)
		return NewCatalogSnapshot(nil)
	}

	return catalog
}

// LoadCachedCatalogContext is the cancellable cache-first read used by the
// launcher. It never touches the Songs directory; a source mismatch returns an
// empty snapshot so rows from a different profile cannot be shown as current.
func LoadCachedCatalogContext(ctx context.Context) (*CatalogSnapshot, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	if dbFile == nil {
		return nil, errors.New("database is not initialized")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !catalogSourceMatches {
		return NewCatalogSnapshot(nil), nil
	}

	entries, err := queryCatalogEntriesContext(ctx, "WHERE mode = 0")
	if err != nil {
		return nil, err
	}

	return NewCatalogSnapshot(entries), nil
}

// SeedCatalogFromStableDatabase ingests the optional adjacent osu!.db only
// when danser has no standard-mode catalog rows. It returns the rows added to
// danser's authoritative catalog so the launcher can publish them before the
// filesystem reconciliation finishes. A bad, stale, or unavailable Stable
// database is an accelerator miss, never a catalog failure.
func SeedCatalogFromStableDatabase() CatalogDelta {
	delta, err := SeedCatalogFromStableDatabaseContext(context.Background())
	if err != nil {
		log.Println("DatabaseManager: Stable database seed stopped:", err)
	}

	return delta
}

// SeedCatalogFromStableDatabaseContext ingests the optional adjacent osu!.db
// without making it a correctness dependency. Only cancellation is returned
// to the caller; malformed, stale, or unavailable Stable data is logged and
// treated as an accelerator miss.
func SeedCatalogFromStableDatabaseContext(ctx context.Context) (CatalogDelta, error) {
	if ctx == nil {
		return CatalogDelta{}, errors.New("nil context")
	}
	if err := ctx.Err(); err != nil {
		return CatalogDelta{}, err
	}
	if dbFile == nil || !catalogSourceMatches {
		return CatalogDelta{}, nil
	}

	var hasEntries bool
	if err := dbFile.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM beatmaps WHERE mode = 0)").Scan(&hasEntries); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return CatalogDelta{}, contextErr
		}

		log.Println("DatabaseManager: Could not inspect catalog before osu!.db import:", err)
		return CatalogDelta{}, nil
	}
	if hasEntries {
		return CatalogDelta{}, nil
	}

	stableEntries, err := loadStableDatabaseContext(ctx)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return CatalogDelta{}, contextErr
		}

		if !os.IsNotExist(err) {
			log.Println("DatabaseManager: Ignoring osu!.db accelerator:", err)
		}
		return CatalogDelta{}, nil
	}

	standardEntries := make([]*BeatmapEntry, 0, len(stableEntries))
	for _, entry := range stableEntries {
		if entry.Mode == 0 {
			standardEntries = append(standardEntries, entry)
		}
	}
	if len(standardEntries) == 0 {
		return CatalogDelta{}, nil
	}
	if err := ctx.Err(); err != nil {
		return CatalogDelta{}, err
	}

	if err = upsertCatalogEntries(standardEntries); err != nil {
		log.Println("DatabaseManager: Could not seed catalog from osu!.db:", err)
		return CatalogDelta{}, nil
	}

	log.Println("DatabaseManager: Seeded", len(standardEntries), "maps from optional osu!.db accelerator.")
	return CatalogDelta{Upserts: standardEntries}, nil
}

// ReconcileCatalog updates danser's catalog from the configured source. It
// does not load or materialize the resulting catalog; callers can publish a
// new snapshot after the operation completes.
func ReconcileCatalog(skipDatabaseCheck bool, importListener ImportListener) CatalogDelta {
	delta, err := ReconcileCatalogContext(context.Background(), skipDatabaseCheck, importListener)
	if err != nil {
		log.Println("DatabaseManager: Catalog reconciliation stopped:", err)
	}

	return delta
}

// ReconcileCatalogContext updates danser's catalog from the configured source
// and stops at safe boundaries when ctx is cancelled. The durable catalog may
// contain work committed before cancellation, but the returned delta is only
// published by callers that completed the operation for their generation.
func ReconcileCatalogContext(ctx context.Context, skipDatabaseCheck bool, importListener ImportListener) (CatalogDelta, error) {
	return ReconcileCatalogWithDeltasContext(ctx, skipDatabaseCheck, importListener, nil)
}

// CatalogDeltaListener receives catalog changes after they have been written
// successfully. The listener must treat entries as immutable and return
// promptly so reconciliation can continue.
type CatalogDeltaListener func(CatalogDelta)

// ReconcileCatalogWithDeltasContext updates danser's catalog and reports each
// durable batch as it becomes available. The returned delta still contains the
// complete operation for callers that publish only after reconciliation.
func ReconcileCatalogWithDeltasContext(ctx context.Context, skipDatabaseCheck bool, importListener ImportListener, deltaListener CatalogDeltaListener) (CatalogDelta, error) {
	if ctx == nil {
		return CatalogDelta{}, errors.New("nil context")
	}
	if dbFile == nil {
		log.Println("DatabaseManager: Cannot reconcile catalog before initialization")
		return CatalogDelta{}, errors.New("database is not initialized")
	}
	if err := ctx.Err(); err != nil {
		return CatalogDelta{}, err
	}

	var unpackedMaps []string
	if settings.General.UnpackOszFiles {
		if err := ctx.Err(); err != nil {
			return CatalogDelta{}, err
		}
		unpackedMaps = unpackMaps()
	}

	delta, err := SeedCatalogFromStableDatabaseContext(ctx)
	if err != nil {
		return delta, err
	}
	notifyCatalogDelta(deltaListener, delta)

	importDelta, err := importMapsContext(ctx, skipDatabaseCheck, unpackedMaps, importListener, deltaListener)
	if err != nil {
		return delta, err
	}
	delta.Upserts = append(delta.Upserts, importDelta.Upserts...)
	delta.Removals = append(delta.Removals, importDelta.Removals...)
	return delta, nil
}

// RebuildCatalog invalidates every cached source fingerprint and performs a
// complete reconciliation. It deliberately keeps local play statistics and
// rows until replacement succeeds, so a failed rebuild does not destroy the
// last usable catalog.
func RebuildCatalog(importListener ImportListener) CatalogDelta {
	if dbFile == nil {
		log.Println("DatabaseManager: Cannot rebuild an unopened database")
		return CatalogDelta{}
	}

	if _, err := dbFile.Exec("UPDATE beatmaps SET lastModified = -1, fileSize = -1, metadataState = ?", MetadataInvalid); err != nil {
		log.Println("DatabaseManager: Failed to invalidate cached fingerprints:", err)
		return CatalogDelta{}
	}

	return ReconcileCatalog(false, importListener)
}

func unpackMaps() (dirs []string) {
	oszs, err := files.SearchFiles(songsDir, "*.osz", 0)

	if err == nil && len(oszs) > 0 {
		for _, osz := range oszs {
			baseName := filepath.Base(osz)
			if !strings.EqualFold(filepath.Ext(baseName), ".osz") {
				continue
			}
			dirName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

			destination := filepath.Join(filepath.Dir(osz), dirName)

			log.Println("DatabaseManager: Unpacking", osz, "->", destination)

			if _, err := utils.Unzip(osz, destination); err != nil {
				log.Println("DatabaseManager: Failed to unpack", osz, err)
				continue
			}
			if err := os.Remove(osz); err != nil {
				log.Println("DatabaseManager: Failed to remove unpacked archive", osz, err)
				continue
			}

			dirs = append(dirs, dirName)
		}
	}

	return
}

type ImportListener func(stage ImportStage, progress, target int)

type ImportStage int

const (
	Discovery = ImportStage(iota)
	Comparison
	Cleanup
	Import
	StarRating
	Finished
)

func importMaps(skipDatabaseCheck bool, mustCheckDirs []string, importListener ImportListener) CatalogDelta {
	delta, err := importMapsContext(context.Background(), skipDatabaseCheck, mustCheckDirs, importListener, nil)
	if err != nil {
		log.Println("DatabaseManager: Map import stopped:", err)
	}

	return delta
}

func importMapsContext(ctx context.Context, skipDatabaseCheck bool, mustCheckDirs []string, importListener ImportListener, deltaListener CatalogDeltaListener) (CatalogDelta, error) {
	const workers = 2
	var delta CatalogDelta
	// A cache from another Songs directory is not safe to skip, even when the
	// caller requested the fast -nodbcheck path. Source identity therefore
	// forces one complete reconciliation before the new location is recorded.
	effectiveSkipDatabaseCheck := skipDatabaseCheck && catalogSourceMatches

	cachedFolders, mapsInDB, fingerprintsComplete := getLastModifiedContext(ctx)
	if !fingerprintsComplete {
		if err := ctx.Err(); err != nil {
			return delta, err
		}
		return delta, errors.New("load cached beatmap fingerprints")
	}

	log.Printf("DatabaseManager: Scanning %q for .osu files...", songsDir)

	if effectiveSkipDatabaseCheck {
		log.Println("DatabaseManager: '-nodbcheck' is active so only new directories will be imported.")
	} else if skipDatabaseCheck && !catalogSourceMatches {
		log.Println("DatabaseManager: '-nodbcheck' is ignored until the changed Songs directory is reconciled.")
	}

	trySendStatus(importListener, Discovery, 0, 0)

	scan, err := scanBeatmapFilesContext(ctx, songsDir, effectiveSkipDatabaseCheck, cachedFolders, mustCheckDirs, func(directoriesSeen int) {
		trySendStatus(importListener, Discovery, directoriesSeen, 0)
	})

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return delta, err
		}

		return delta, fmt.Errorf("scan Songs directory: %w", err)
	}

	log.Printf("DatabaseManager: Scan complete. Found %d files in %d directories.", len(scan.candidates), scan.directoriesSeen)

	log.Println("DatabaseManager: Comparing files with database...")

	mapsToImport := make([]modMap, 0)
	cleanupFailed := false

	trySendStatus(importListener, Comparison, 0, 0)

	for _, candidate := range scan.candidates {
		if err := ctx.Err(); err != nil {
			return delta, err
		}

		if previous, ok := mapsInDB[candidate.location.key()]; ok {
			// A seen path is not stale merely because its fingerprint changed.
			// Keep it out of the cleanup set before parsing so a transient read,
			// replacement, or malformed edit cannot erase the last good row.
			delete(mapsInDB, candidate.location.key())

			// Stable's optional database is only an accelerator. Its metadata can
			// lag behind the filesystem and it does not provide a reliable content
			// fingerprint for danser's catalog, so every seeded row must be parsed
			// once by the authoritative filesystem reconciliation.
			unchanged := previous.fingerprint.metadataState != MetadataFromStableDatabase && catalogSourceMatches && previous.fingerprint.modified == candidate.fingerprint.modified && previous.fingerprint.size == candidate.fingerprint.size
			// Preserve the spelling used by the source filesystem. The catalog
			// identity is case-insensitive for Stable/Windows compatibility, but
			// a case-sensitive filesystem still needs the current path to open the
			// file after a rename.
			unchanged = unchanged && previous.location.dir == candidate.location.dir && previous.location.file == candidate.location.file
			if unchanged {
				continue
			}

			if settings.General.VerboseImportLogs {
				log.Println("DatabaseManager: New beatmap version found:", candidate.location.file)
			}

			candidate.localState = previous.localState
		} else if settings.General.VerboseImportLogs {
			log.Println("DatabaseManager: New beatmap found:", candidate.location.file)
		}

		mapsToImport = append(mapsToImport, candidate)
	}

	log.Println("DatabaseManager: Compare complete.")
	if scan.complete && len(mapsInDB) > 0 && !effectiveSkipDatabaseCheck {
		trySendStatus(importListener, Cleanup, 0, len(mapsInDB))

		log.Println("DatabaseManager: Removing leftover maps from database...")

		mapsToRemove := make([]mapLocation, 0, len(mapsInDB))

		for _, cached := range mapsInDB {
			if err := ctx.Err(); err != nil {
				return delta, err
			}

			mapsToRemove = append(mapsToRemove, cached.location)
		}

		if err := removeBeatmaps(mapsToRemove); err != nil {
			log.Println("DatabaseManager: Failed to remove stale catalog rows:", err)
			cleanupFailed = true
		} else {
			cleanupDelta := CatalogDelta{Removals: make([]string, 0, len(mapsToRemove))}
			for _, location := range mapsToRemove {
				key := location.key()
				delta.Removals = append(delta.Removals, key)
				cleanupDelta.Removals = append(cleanupDelta.Removals, key)
			}
			notifyCatalogDelta(deltaListener, cleanupDelta)
			trySendStatus(importListener, Cleanup, len(mapsToRemove), len(mapsToRemove))
		}

		log.Println("DatabaseManager: Removal complete.")
	}

	if len(mapsToImport) == 0 {
		if scan.complete && !cleanupFailed {
			persistCatalogSource()
		}
		return delta, nil
	}

	log.Println("DatabaseManager: Starting import of", len(mapsToImport), "maps. It may take up to several minutes...")

	trySendStatus(importListener, Import, 0, len(mapsToImport))

	receive := importBeatmapsContext(ctx, mapsToImport, workers)

	var numImported int
	var imported []*beatmap.BeatMap
	persistenceFailed := false

	for bMap := range receive {
		numImported++
		trySendStatus(importListener, Import, numImported, len(mapsToImport))

		imported = append(imported, bMap)

		if len(imported) >= 1000 { // Commit periodically so a crash or close loses only a bounded batch.
			entries := insertBeatmaps(imported)
			if len(entries) != len(imported) {
				persistenceFailed = true
			}
			appendImportedDelta(&delta, entries)
			notifyCatalogDelta(deltaListener, CatalogDelta{Upserts: entries})

			imported = imported[:0]
		}

		if err := ctx.Err(); err != nil {
			return delta, err
		}
	}

	if len(imported) > 0 {
		entries := insertBeatmaps(imported)
		if len(entries) != len(imported) {
			persistenceFailed = true
		}
		appendImportedDelta(&delta, entries)
		notifyCatalogDelta(deltaListener, CatalogDelta{Upserts: entries})
	}

	trySendStatus(importListener, Finished, 100, 100)

	if numImported > 0 {
		log.Println("DatabaseManager: Imported", numImported, "new/updated beatmaps.")
	} else {
		log.Println("DatabaseManager: No new/updated beatmaps imported.")
	}

	// A changed source directory must not become trusted merely because its
	// directory walk completed. Keep the old source identity hidden until all
	// observed replacements were parsed and durably written; otherwise an
	// unreadable file could make an old row appear to belong to the new drive.
	if scan.complete && !cleanupFailed && !persistenceFailed && numImported == len(mapsToImport) {
		persistCatalogSource()
	}

	return delta, nil
}

func notifyCatalogDelta(listener CatalogDeltaListener, delta CatalogDelta) {
	if listener == nil || len(delta.Upserts) == 0 && len(delta.Removals) == 0 {
		return
	}

	listener(delta)
}

// importBeatmapsContext feeds map candidates through a bounded worker pool.
// The coordinator goroutine owns both channel closure and worker joining, so
// callers can simply range over the result channel without racing a producer
// that is still trying to send after cancellation.
func importBeatmapsContext(ctx context.Context, candidates []modMap, workers int) <-chan *beatmap.BeatMap {
	if workers < 1 {
		workers = 1
	}

	receive := make(chan *beatmap.BeatMap, workers)

	go func() {
		defer close(receive)

		queue := make(chan modMap)
		var importWorkers sync.WaitGroup

		for range workers {
			importWorkers.Add(1)
			go func() {
				defer importWorkers.Done()

				for {
					select {
					case <-ctx.Done():
						return
					case candidate, ok := <-queue:
						if !ok {
							return
						}

						partialPath := filepath.Join(candidate.location.dir, candidate.location.file)
						bMap, ok := importBeatmap(candidate, partialPath)
						if !ok {
							continue
						}

						select {
						case receive <- bMap:
						case <-ctx.Done():
							return
						}
					}
				}
			}()
		}

		for _, candidate := range candidates {
			select {
			case queue <- candidate:
			case <-ctx.Done():
				close(queue)
				importWorkers.Wait()
				return
			}
		}

		close(queue)
		importWorkers.Wait()
	}()

	return receive
}

// importBeatmap parses one candidate from the current Songs directory. The
// parser accepts third-party and partially malformed files, so an isolated
// failure must be logged and skipped without taking down the catalog worker.
func importBeatmap(candidate modMap, partialPath string) (bMap *beatmap.BeatMap, ok bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("DatabaseManager: Failed to load %q: %v", partialPath, recovered)
			bMap = nil
			ok = false
		}
	}()

	mapPath := filepath.Join(songsDir, partialPath)

	file, err := os.Open(mapPath)
	if err != nil {
		log.Printf("DatabaseManager: Failed to read %q, skipping: %v", partialPath, err)
		return nil, false
	}
	defer file.Close()

	if settings.General.VerboseImportLogs {
		log.Println("DatabaseManager: Importing:", partialPath)
	}

	bMap, parseErr := beatmap.ParseBeatMapFileWithError(file)
	if parseErr != nil {
		log.Println("DatabaseManager: Failed to import:", partialPath, parseErr)
		return nil, false
	}

	fileInfo, statErr := file.Stat()
	if statErr != nil {
		log.Println("DatabaseManager: Failed to stat imported map:", partialPath, statErr)
		return nil, false
	}

	// Use metadata from the same open handle as the parser and hasher. A map
	// replaced between discovery and import must not be cached with the older
	// directory-entry fingerprint.
	bMap.Dir = candidate.location.dir
	bMap.File = candidate.location.file
	bMap.LastModified = fileInfo.ModTime().UnixNano() / int64(1e6)
	bMap.FileSize = fileInfo.Size()
	bMap.TimeAdded = candidate.localState.timeAdded
	if bMap.TimeAdded == 0 {
		bMap.TimeAdded = time.Now().UnixNano() / 1000000
	}
	bMap.PlayCount = candidate.localState.playCount
	bMap.LastPlayed = candidate.localState.lastPlayed
	bMap.LocalOffset = candidate.localState.localOffset

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		log.Println("DatabaseManager: Failed to rewind:", partialPath, err)
		return nil, false
	}

	if bMap.MD5, err = hashBeatmapFile(file); err != nil {
		log.Println("DatabaseManager: Failed to hash:", partialPath, err)
		return nil, false
	}

	if settings.General.VerboseImportLogs {
		log.Println("DatabaseManager: Imported:", partialPath)
	}

	return bMap, true
}

func sameCatalogSource(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}

	return strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}

func inferCatalogVersion() (int, error) {
	columns, err := catalogColumns()
	if err != nil {
		return 0, err
	}

	has := func(column string) bool {
		_, ok := columns[column]
		return ok
	}

	// The version marker was introduced after the table itself, so infer the
	// newest migration represented by the columns when recovering a database
	// that lost its info rows before the first successful initialization.
	switch {
	case has("filesize") && has("metadatastate"):
		return databaseVersion, nil
	case has("localoffset"):
		return 20220622, nil
	case has("starsversion"):
		return 20220605, nil
	case has("setid") && has("mapid"):
		return 20210423, nil
	case has("bpmin") && has("bpmmax") && has("endtime"):
		return 20201118, nil
	case has("stars"):
		return 20201117, nil
	case has("hpdrain") && has("od"):
		return 20181111, nil
	default:
		return 0, nil
	}
}

func catalogColumns() (map[string]struct{}, error) {
	rows, err := dbFile.Query("PRAGMA table_info(beatmaps)")
	if err != nil {
		return nil, fmt.Errorf("cannot inspect beatmap catalog columns: %w", err)
	}
	defer rows.Close()

	columns := make(map[string]struct{})
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
			return nil, fmt.Errorf("read beatmap catalog columns: %w", err)
		}
		columns[strings.ToLower(columnName)] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("read beatmap catalog columns: %w", err)
	}

	return columns, nil
}

func ensureCatalogColumns() error {
	columns, err := catalogColumns()
	if err != nil {
		return err
	}

	missing := []struct {
		name       string
		definition string
	}{
		{name: "filesize", definition: "fileSize INTEGER DEFAULT 0"},
		{name: "metadatastate", definition: "metadataState INTEGER DEFAULT 0"},
	}

	for _, column := range missing {
		if _, exists := columns[column.name]; exists {
			continue
		}

		if _, err = dbFile.Exec("ALTER TABLE beatmaps ADD COLUMN " + column.definition); err != nil {
			return fmt.Errorf("add beatmap catalog column %s: %w", column.name, err)
		}
	}

	return nil
}

func ensureCatalogLocationIndex() error {
	var exists bool
	if err := dbFile.QueryRow("SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'index' AND name = 'idx_beatmaps_location_nocase')").Scan(&exists); err != nil {
		return fmt.Errorf("cannot inspect beatmap location index: %w", err)
	}
	if exists {
		return nil
	}

	// Windows treats source paths case-insensitively. Older danser databases
	// used a binary-collated index, so repair duplicate casing before creating
	// the authoritative index. This branch runs once per database; subsequent
	// startups only perform the inexpensive sqlite_master probe above.
	if _, err := dbFile.Exec(`DELETE FROM beatmaps
		WHERE rowid NOT IN (
			SELECT MIN(rowid)
			FROM beatmaps
			GROUP BY dir COLLATE NOCASE, file COLLATE NOCASE
		)`); err != nil {
		return fmt.Errorf("cannot remove duplicate beatmap locations: %w", err)
	}

	if _, err := dbFile.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_beatmaps_location_nocase ON beatmaps (dir COLLATE NOCASE, file COLLATE NOCASE)"); err != nil {
		return fmt.Errorf("cannot create beatmap location index: %w", err)
	}

	return nil
}

func persistCatalogSource() {
	if _, err := dbFile.Exec("REPLACE INTO info (key, value) VALUES ('songs_dir', ?)", songsDir); err != nil {
		log.Println("DatabaseManager: Failed to persist Songs directory identity:", err)
		return
	}

	catalogSourceMatches = true
}

func appendImportedDelta(delta *CatalogDelta, entries []*BeatmapEntry) {
	if delta == nil {
		return
	}

	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if entry.Mode == 0 {
			delta.Upserts = append(delta.Upserts, entry)
		} else {
			delta.Removals = append(delta.Removals, entry.MapKey())
		}
	}
}

func trySendStatus(listener ImportListener, stage ImportStage, progress, target int) {
	if listener != nil {
		listener(stage, progress, target)
	}
}

func UpdateStarRating(maps []*beatmap.BeatMap, progressListener func(processed, target int, message string)) {
	const workers = 1 // For now using only one thread because calculating 4 aspire maps at once can OOM since (de)allocation can't keep up with many complex sliders

	var toCalculate []*beatmap.BeatMap

	for _, b := range maps {
		if b.Mode == 0 && (b.Stars < 0 || b.StarsVersion < difficultyCalc.GetVersion()) {
			toCalculate = append(toCalculate, b)
		}
	}

	if len(toCalculate) == 0 {
		return
	}

	var message string

	if progressListener != nil {
		progressListener(0, len(toCalculate), message)
	}

	receive := make(chan *beatmap.BeatMap, workers)

	var progress int

	goroutines.Run(func() {
		util.BalanceChanWatchdog(workers, toCalculate, receive, time.Minute, func(worker int, a *beatmap.BeatMap) {
			log.Println("DatabaseManager: It seems like SR calculation for this file got stuck! Please report it to developer(s). File:", a.Dir+"/"+a.File)
			message = "Calculation got stuck! Please check logs"

			if progressListener != nil {
				progressListener(progress, len(toCalculate), message)
			}
		}, func(bMap *beatmap.BeatMap) (ret *beatmap.BeatMap, ret2 bool) {
			ret = bMap // HACK: still return the beatmap even if execution panics: https://golangbyexample.com/return-value-function-panic-recover-go/
			ret2 = true

			defer func() {
				bMap.StarsVersion = difficultyCalc.GetVersion()
				bMap.Clear() //Clear objects and timing to avoid OOM

				if err := recover(); err != nil { //TODO: Technically should be fixed but unexpected parsing problem won't crash whole process
					bMap.Stars = 0
					log.Println("DatabaseManager: Failed to load \"", bMap.Dir+"/"+bMap.File, "\":", err)
				}
			}()

			beatmap.ParseTimingPointsAndPauses(bMap)
			beatmap.ParseObjects(bMap, true, false)

			if len(bMap.HitObjects) < 2 {
				log.Println("DatabaseManager:", bMap.Dir+"/"+bMap.File, "doesn't have enough hitobjects")
				bMap.Stars = 0
			} else {
				attr := difficultyCalc.CalculateSingle(bMap, bMap.Diff)
				bMap.Stars = attr.Total
			}

			return
		})

		close(receive)
	})

	var calculated []*beatmap.BeatMap

	for bMap := range receive {
		if progressListener != nil {
			message = ""
			progress++
			progressListener(progress, len(toCalculate), message)
		}

		calculated = append(calculated, bMap)

		if len(calculated) >= 200 { // Commit periodically so a crash or close loses only a bounded batch.
			if err := pushSRToDB(calculated); err != nil {
				log.Println("DatabaseManager: Failed to persist star ratings:", err)
			}

			calculated = calculated[:0]
		}
	}

	if len(calculated) > 0 {
		if err := pushSRToDB(calculated); err != nil {
			log.Println("DatabaseManager: Failed to persist star ratings:", err)
		}
	}

	log.Println("DatabaseManager: Star rating updated!")
}

// UpdateStarRatingContext is the cancellable form used by the launcher's
// background catalog coordinator. Rating calculation remains serial because
// parsing complex sliders temporarily materializes a large object graph.
func UpdateStarRatingContext(ctx context.Context, maps []*beatmap.BeatMap, progressListener func(processed, target int, message string)) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	var toCalculate []*beatmap.BeatMap

	for _, bMap := range maps {
		if bMap != nil && bMap.Mode == 0 && (bMap.Stars < 0 || bMap.StarsVersion < difficultyCalc.GetVersion()) {
			toCalculate = append(toCalculate, bMap)
		}
	}

	if len(toCalculate) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if progressListener != nil {
		progressListener(0, len(toCalculate), "")
	}

	calculated := make([]*beatmap.BeatMap, 0, min(len(toCalculate), 200))
	for progress, bMap := range toCalculate {
		if err := ctx.Err(); err != nil {
			return err
		}

		func() {
			defer func() {
				bMap.StarsVersion = difficultyCalc.GetVersion()
				bMap.Clear()

				if recovered := recover(); recovered != nil {
					bMap.Stars = 0
					log.Printf("DatabaseManager: Failed to calculate star rating for %q/%q: %v", bMap.Dir, bMap.File, recovered)
				}
			}()

			beatmap.ParseTimingPointsAndPauses(bMap)
			beatmap.ParseObjects(bMap, true, false)
			if len(bMap.HitObjects) < 2 {
				log.Println("DatabaseManager:", bMap.Dir+"/"+bMap.File, "doesn't have enough hitobjects")
				bMap.Stars = 0
				return
			}

			attr := difficultyCalc.CalculateSingle(bMap, bMap.Diff)
			bMap.Stars = attr.Total

		}()

		if progressListener != nil {
			progressListener(progress+1, len(toCalculate), "")
		}

		calculated = append(calculated, bMap)
		if len(calculated) < 200 && progress+1 != len(toCalculate) {
			continue
		}

		if err := pushSRToDB(calculated); err != nil {
			return err
		}
		calculated = calculated[:0]
	}

	log.Println("DatabaseManager: Star rating updated!")
	return nil
}

// UpdateCatalogStarRating calculates ratings only for catalog entries that
// need them. This keeps cache-first startup from materializing every map just
// to retain the old rating refresh behavior.
func UpdateCatalogStarRating(progressListener func(processed, target int, message string)) CatalogDelta {
	if dbFile == nil {
		return CatalogDelta{}
	}

	entries := loadStaleCatalogEntries(difficultyCalc.GetVersion())
	toCalculate := make([]*beatmap.BeatMap, 0)

	for _, entry := range entries {
		if entry.MetadataState == MetadataFromStableDatabase && entry.FileSize < 0 {
			// -nodbcheck deliberately leaves Stable-only rows provisional. Do
			// not turn a fast launch into a full read and hash of every source
			// file just to refresh ratings; the normal scanner repairs the size
			// fingerprint before this pass.
			continue
		}

		bMap, err := loadRuntimeBeatMap(entry, false)
		if err != nil {
			log.Println("DatabaseManager: Failed to load map for star rating:", entry.Dir+"/"+entry.File, err)
			continue
		}
		toCalculate = append(toCalculate, bMap)
	}

	if len(toCalculate) == 0 {
		return CatalogDelta{}
	}

	UpdateStarRating(toCalculate, progressListener)

	delta := CatalogDelta{Upserts: make([]*BeatmapEntry, 0, len(toCalculate))}
	for _, bMap := range toCalculate {
		entry := NewBeatmapEntry(bMap)
		if entry == nil {
			continue
		}
		delta.Upserts = append(delta.Upserts, entry)
	}
	if err := upsertCatalogEntries(delta.Upserts); err != nil {
		log.Println("DatabaseManager: Failed to persist star ratings:", err)
		return CatalogDelta{}
	}

	return delta
}

// UpdateCatalogStarRatingContext is the cancellable catalog-level rating
// refresh. The existing non-context API remains for command-line callers.
func UpdateCatalogStarRatingContext(ctx context.Context, progressListener func(processed, target int, message string)) (CatalogDelta, error) {
	if ctx == nil {
		return CatalogDelta{}, errors.New("nil context")
	}
	if dbFile == nil {
		return CatalogDelta{}, errors.New("database is not initialized")
	}
	if err := ctx.Err(); err != nil {
		return CatalogDelta{}, err
	}

	entries, err := loadStaleCatalogEntriesContext(ctx, difficultyCalc.GetVersion())
	if err != nil {
		return CatalogDelta{}, err
	}
	toCalculate := make([]*beatmap.BeatMap, 0, len(entries))

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return CatalogDelta{}, err
		}

		if entry.MetadataState == MetadataFromStableDatabase && entry.FileSize < 0 {
			// Stable metadata without a verified file size is provisional. Do not
			// turn the rating pass into an implicit full filesystem validation.
			continue
		}

		bMap, err := loadRuntimeBeatMap(entry, false)
		if err != nil {
			log.Println("DatabaseManager: Failed to load map for star rating:", entry.Dir+"/"+entry.File, err)
			continue
		}
		toCalculate = append(toCalculate, bMap)
	}

	if len(toCalculate) == 0 {
		return CatalogDelta{}, nil
	}

	if err := UpdateStarRatingContext(ctx, toCalculate, progressListener); err != nil {
		return CatalogDelta{}, err
	}

	delta := CatalogDelta{Upserts: make([]*BeatmapEntry, 0, len(toCalculate))}
	for _, bMap := range toCalculate {
		if entry := NewBeatmapEntry(bMap); entry != nil {
			delta.Upserts = append(delta.Upserts, entry)
		}
	}

	if err := upsertCatalogEntries(delta.Upserts); err != nil {
		return CatalogDelta{}, fmt.Errorf("persist star-rating catalog update: %w", err)
	}

	return delta, nil
}

func pushSRToDB(maps []*beatmap.BeatMap) error {
	if dbFile == nil {
		return errors.New("database is not initialized")
	}

	tx, err := dbFile.Begin()
	if err != nil {
		return fmt.Errorf("begin star-rating update: %w", err)
	}
	defer tx.Rollback()

	st, err := tx.Prepare("UPDATE beatmaps SET stars = ?, starsVersion = ? WHERE dir = ? COLLATE NOCASE AND file = ? COLLATE NOCASE")
	if err != nil {
		return fmt.Errorf("prepare star-rating update: %w", err)
	}
	defer st.Close()

	for _, bMap := range maps {
		if bMap == nil {
			continue
		}

		_, err = st.Exec(
			bMap.Stars,
			bMap.StarsVersion,
			bMap.Dir,
			bMap.File)

		if err != nil {
			return fmt.Errorf("update star rating for %q/%q: %w", bMap.Dir, bMap.File, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit star-rating update: %w", err)
	}

	return nil
}

func UpdatePlayStats(beatmap *beatmap.BeatMap) {
	if dbFile == nil || beatmap == nil {
		return
	}

	_, err := dbFile.Exec("UPDATE beatmaps SET playCount = ?, lastPlayed = ? WHERE dir = ? COLLATE NOCASE AND file = ? COLLATE NOCASE", beatmap.PlayCount, beatmap.LastPlayed, beatmap.Dir, beatmap.File)
	if err != nil {
		log.Println(err)
	}
}

func UpdateLocalOffset(beatmap *beatmap.BeatMap) {
	if dbFile == nil || beatmap == nil {
		return
	}

	_, err := dbFile.Exec("UPDATE beatmaps SET localOffset = ? WHERE dir = ? COLLATE NOCASE AND file = ? COLLATE NOCASE", beatmap.LocalOffset, beatmap.Dir, beatmap.File)
	if err != nil {
		log.Println(err)
	}
}

func removeBeatmaps(toRemove []mapLocation) error {
	if len(toRemove) == 0 {
		return nil
	}
	if dbFile == nil {
		return errors.New("database is not initialized")
	}

	tx, err := dbFile.Begin()
	if err != nil {
		return fmt.Errorf("begin beatmap removal: %w", err)
	}
	defer tx.Rollback()

	st, err := tx.Prepare("DELETE FROM beatmaps WHERE dir = ? COLLATE NOCASE AND file = ? COLLATE NOCASE")
	if err != nil {
		return fmt.Errorf("prepare beatmap removal: %w", err)
	}
	defer st.Close()

	for _, bMap := range toRemove {
		if _, err := st.Exec(bMap.dir, bMap.file); err != nil {
			return fmt.Errorf("remove beatmap %q/%q: %w", bMap.dir, bMap.file, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit beatmap removal: %w", err)
	}

	return nil
}

func migrateBeatmaps() error {
	_, lastModified, fingerprintsComplete := getLastModified()
	if !fingerprintsComplete {
		return errors.New("load cached beatmap fingerprints")
	}

	var removeList []mapLocation

	if currentPreVersion < databaseVersion {
		updateBeatmaps := false

		for _, m := range migrations {
			if currentPreVersion < m.Date() {
				updateBeatmaps = updateBeatmaps || m.FieldsToMigrate() != nil
			}
		}

		if updateBeatmaps {
			log.Println("Updating cached beatmaps...")

			log.Println("Loading cached beatmaps from disk...")

			toUpdate := make([]*beatmap.BeatMap, 0)

			for _, cached := range lastModified {
				location := cached.location
				file, err := os.Open(filepath.Join(songsDir, location.dir, location.file))
				if err != nil {
					log.Println("Failed to open file, removing from database:", location.file)
					log.Println("Error:", err)

					removeList = append(removeList, location)

					continue
				}

				bMap := beatmap.ParseBeatMapFile(file)
				if err := file.Close(); err != nil {
					log.Println("Failed to close cached beatmap:", location.file, err)
				}
				if bMap == nil {
					log.Println("Corrupted cached beatmap found. Removing from database:", location.file)

					removeList = append(removeList, location)

					continue
				}

				toUpdate = append(toUpdate, bMap)
			}

			log.Println("Cached beatmaps loaded! Performing migrations...")

			tx, err := dbFile.Begin()
			if err != nil {
				return fmt.Errorf("begin beatmap migration: %w", err)
			}
			migrationCommitted := false
			defer func() {
				if !migrationCommitted {
					_ = tx.Rollback()
				}
			}()

			for _, m := range migrations {
				if currentPreVersion < m.Date() {
					log.Println("Performing", m.Date(), "migration...")

					if m.FieldsToMigrate() == nil {
						continue
					}

					fieldsArray := m.FieldsToMigrate()
					for i := range fieldsArray {
						fieldsArray[i] += " = ?"
					}

					st, err := tx.Prepare(fmt.Sprintf("UPDATE beatmaps SET %s WHERE dir = ? AND file = ?", strings.Join(fieldsArray, ", ")))
					if err != nil {
						return fmt.Errorf("prepare %d migration: %w", m.Date(), err)
					}

					for _, bMap := range toUpdate {
						values := append(m.GetValues(bMap), bMap.Dir, bMap.File)

						_, err = st.Exec(values...)

						if err != nil {
							_ = st.Close()
							return fmt.Errorf("apply %d migration: %w", m.Date(), err)
						}
					}

					if err = st.Close(); err != nil {
						return fmt.Errorf("close %d migration: %w", m.Date(), err)
					}
				}
			}

			log.Println("Committing migrations to database...")

			err = tx.Commit()
			if err != nil {
				return fmt.Errorf("commit beatmap migration: %w", err)
			}
			migrationCommitted = true
		}
	}

	if err := removeBeatmaps(removeList); err != nil {
		return fmt.Errorf("remove migrated beatmaps: %w", err)
	}

	return nil
}

func insertBeatmaps(bMaps []*beatmap.BeatMap) []*BeatmapEntry {
	if len(bMaps) == 0 {
		return nil
	}

	entries := make([]*BeatmapEntry, 0, len(bMaps))
	for _, bMap := range bMaps {
		if entry := NewBeatmapEntry(bMap); entry != nil {
			entries = append(entries, entry)
		}
	}

	if err := upsertCatalogEntries(entries); err != nil {
		log.Println("DatabaseManager: Failed to persist imported beatmaps:", err)
		return nil
	}

	return entries
}

func upsertCatalogEntries(entries []*BeatmapEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if dbFile == nil {
		return errors.New("database is not initialized")
	}

	tx, err := dbFile.Begin()
	if err != nil {
		return fmt.Errorf("begin beatmap upsert: %w", err)
	}
	defer tx.Rollback()

	const statement = `INSERT INTO beatmaps (
		dir, file, lastModified, title, titleUnicode, artist, artistUnicode,
		creator, version, source, tags, cs, ar, sliderMultiplier, sliderTickRate,
		audioFile, previewTime, sampleSet, stackLeniency, mode, bg, md5, dateAdded,
		playCount, lastPlayed, hpdrain, od, stars, bpmMin, bpmMax, circles,
		sliders, spinners, endTime, setID, mapID, starsVersion, localOffset,
		fileSize, metadataState
	) VALUES (
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	)
	ON CONFLICT DO UPDATE SET
		dir = excluded.dir,
		file = excluded.file,
		lastModified = excluded.lastModified,
		title = excluded.title,
		titleUnicode = excluded.titleUnicode,
		artist = excluded.artist,
		artistUnicode = excluded.artistUnicode,
		creator = excluded.creator,
		version = excluded.version,
		source = excluded.source,
		tags = excluded.tags,
		cs = excluded.cs,
		ar = excluded.ar,
		sliderMultiplier = excluded.sliderMultiplier,
		sliderTickRate = excluded.sliderTickRate,
		audioFile = excluded.audioFile,
		previewTime = excluded.previewTime,
		sampleSet = excluded.sampleSet,
		stackLeniency = excluded.stackLeniency,
		mode = excluded.mode,
		bg = excluded.bg,
		md5 = excluded.md5,
		hpdrain = excluded.hpdrain,
		od = excluded.od,
		stars = excluded.stars,
		bpmMin = excluded.bpmMin,
		bpmMax = excluded.bpmMax,
		circles = excluded.circles,
		sliders = excluded.sliders,
		spinners = excluded.spinners,
		endTime = excluded.endTime,
		setID = excluded.setID,
		mapID = excluded.mapID,
		starsVersion = excluded.starsVersion,
		fileSize = excluded.fileSize,
		metadataState = excluded.metadataState`

	statementHandle, err := tx.Prepare(statement)
	if err != nil {
		return fmt.Errorf("prepare beatmap upsert: %w", err)
	}
	defer statementHandle.Close()

	for _, entry := range entries {
		if entry == nil {
			continue
		}
		entry.prepare()
		if entry.TimeAdded == 0 {
			entry.TimeAdded = time.Now().UnixNano() / 1000000
		}

		_, err = statementHandle.Exec(
			entry.Dir,
			entry.File,
			entry.LastModified,
			entry.Name,
			entry.NameUnicode,
			entry.Artist,
			entry.ArtistUnicode,
			entry.Creator,
			entry.Difficulty,
			entry.Source,
			entry.Tags,
			entry.CircleSize,
			entry.ApproachRate,
			entry.SliderMult,
			entry.SliderTickRate,
			entry.Audio,
			entry.PreviewTime,
			entry.SampleSet,
			entry.StackLeniency,
			entry.Mode,
			entry.Background,
			entry.MD5,
			entry.TimeAdded,
			entry.PlayCount,
			entry.LastPlayed,
			entry.HealthDrain,
			entry.OverallDifficulty,
			entry.Stars,
			entry.MinBPM,
			entry.MaxBPM,
			entry.Circles,
			entry.Sliders,
			entry.Spinners,
			entry.Length,
			entry.SetID,
			entry.ID,
			entry.StarsVersion,
			entry.LocalOffset,
			entry.FileSize,
			entry.MetadataState,
		)
		if err != nil {
			return fmt.Errorf("upsert beatmap %q/%q: %w", entry.Dir, entry.File, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit beatmap upsert: %w", err)
	}

	return nil
}

func loadCatalogEntries() []*BeatmapEntry {
	return queryCatalogEntries("WHERE mode = 0")
}

func loadStaleCatalogEntries(starsVersion int) []*BeatmapEntry {
	entries, err := loadStaleCatalogEntriesContext(context.Background(), starsVersion)
	if err != nil {
		log.Println("DatabaseManager: Failed to load stale catalog entries:", err)
	}

	return entries
}

func loadStaleCatalogEntriesContext(ctx context.Context, starsVersion int) ([]*BeatmapEntry, error) {
	return queryCatalogEntriesContext(ctx, "WHERE mode = 0 AND (stars < 0 OR starsVersion < ?)", starsVersion)
}

func queryCatalogEntries(filter string, args ...any) []*BeatmapEntry {
	entries, err := queryCatalogEntriesContext(context.Background(), filter, args...)
	if err != nil {
		log.Println("DatabaseManager: Failed to load catalog:", err)
	}

	return entries
}

func queryCatalogEntriesContext(ctx context.Context, filter string, args ...any) ([]*BeatmapEntry, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	if dbFile == nil {
		return nil, errors.New("database is not initialized")
	}

	query := `SELECT dir, file, lastModified, title, titleUnicode, artist,
	artistUnicode, creator, version, source, tags, cs, ar, sliderMultiplier,
	sliderTickRate, audioFile, previewTime, sampleSet, stackLeniency, mode, bg,
	md5, dateAdded, playCount, lastPlayed, hpdrain, od, stars, bpmMin, bpmMax,
	circles, sliders, spinners, endTime, setID, mapID, starsVersion, localOffset,
	fileSize, metadataState FROM beatmaps ` + filter

	rows, err := dbFile.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query catalog: %w", err)
	}
	defer rows.Close()

	entries := make([]*BeatmapEntry, 0)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return entries, err
		}

		entry := &BeatmapEntry{}
		var circleSize, approachRate, healthDrain, overallDifficulty float64
		var metadataState int

		err = rows.Scan(
			&entry.Dir,
			&entry.File,
			&entry.LastModified,
			&entry.Name,
			&entry.NameUnicode,
			&entry.Artist,
			&entry.ArtistUnicode,
			&entry.Creator,
			&entry.Difficulty,
			&entry.Source,
			&entry.Tags,
			&circleSize,
			&approachRate,
			&entry.SliderMult,
			&entry.SliderTickRate,
			&entry.Audio,
			&entry.PreviewTime,
			&entry.SampleSet,
			&entry.StackLeniency,
			&entry.Mode,
			&entry.Background,
			&entry.MD5,
			&entry.TimeAdded,
			&entry.PlayCount,
			&entry.LastPlayed,
			&healthDrain,
			&overallDifficulty,
			&entry.Stars,
			&entry.MinBPM,
			&entry.MaxBPM,
			&entry.Circles,
			&entry.Sliders,
			&entry.Spinners,
			&entry.Length,
			&entry.SetID,
			&entry.ID,
			&entry.StarsVersion,
			&entry.LocalOffset,
			&entry.FileSize,
			&metadataState,
		)
		if err != nil {
			return nil, fmt.Errorf("read catalog row: %w", err)
		}

		entry.CircleSize = mutils.Clamp(circleSize, 0, 10)
		entry.ApproachRate = mutils.Clamp(approachRate, 0, 10)
		entry.HealthDrain = mutils.Clamp(healthDrain, 0, 10)
		entry.OverallDifficulty = mutils.Clamp(overallDifficulty, 0, 10)
		entry.MetadataState = MetadataState(metadataState)
		entry.prepare()
		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}

	return entries, nil
}

func getLastModified() (map[string]uint8, map[string]cachedMap, bool) {
	return getLastModifiedContext(context.Background())
}

func getLastModifiedContext(ctx context.Context) (map[string]uint8, map[string]cachedMap, bool) {
	if ctx == nil {
		return nil, nil, false
	}
	if dbFile == nil {
		return nil, nil, false
	}

	res, err := dbFile.QueryContext(ctx, "SELECT dir, file, lastModified, fileSize, metadataState, dateAdded, playCount, lastPlayed, localOffset FROM beatmaps")
	if err != nil {
		log.Println("DatabaseManager: Failed to load file fingerprints:", err)
		return nil, nil, false
	}
	defer res.Close()

	dirs := make(map[string]uint8)

	mod := make(map[string]cachedMap)
	complete := true

	for res.Next() {
		if err := ctx.Err(); err != nil {
			return dirs, mod, false
		}

		var (
			dir, file     string
			lastModified  int64
			fileSize      int64
			metadataState int
			timeAdded     int64
			playCount     int64
			lastPlayed    int64
			localOffset   int
		)

		if err := res.Scan(&dir, &file, &lastModified, &fileSize, &metadataState, &timeAdded, &playCount, &lastPlayed, &localOffset); err != nil {
			log.Println("DatabaseManager: Failed to read file fingerprint:", err)
			complete = false
			break
		}

		dirs[normalizeRelativePath(dir)] = 1

		location := mapLocation{dir: dir, file: file}
		mod[location.key()] = cachedMap{
			location: location,
			fingerprint: fileFingerprint{
				modified:      lastModified,
				size:          fileSize,
				metadataState: MetadataState(metadataState),
			},
			localState: catalogLocalState{
				timeAdded:   timeAdded,
				playCount:   playCount,
				lastPlayed:  lastPlayed,
				localOffset: localOffset,
			},
		}
	}

	if err := res.Err(); err != nil {
		log.Println("DatabaseManager: Failed while reading file fingerprints:", err)
		complete = false
	}

	return dirs, mod, complete
}

func Close() {
	if dbFile != nil {
		err := dbFile.Close()
		if err != nil {
			log.Println("Failed to close database:", err)
		}

		dbFile = nil
	}
}
