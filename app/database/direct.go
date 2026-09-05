package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/innovationreadytupperware/danser-ee/app/beatmap"
)

// LoadDirectBeatmap parses one .osu file by its source-relative location
// without walking the Songs directory or touching committed catalog rows.
// The caller must have successfully called Init so the Songs directory
// identity is known. It exists so a launcher-spawned game can start a
// selected map immediately while reconciliation is still running instead of
// repeating the full library scan before the window appears.
func LoadDirectBeatmap(relativePath string) (*beatmap.BeatMap, error) {
	if songsDir == "" {
		return nil, fmt.Errorf("direct beatmap load before database initialization")
	}

	cleaned, err := cleanDirectBeatmapPath(relativePath)
	if err != nil {
		return nil, err
	}

	// Reuse the importer's single-file pipeline so direct loads parse, stat,
	// and hash exactly like a reconciled row. Local play state is left zeroed;
	// callers preserve it from LookupBeatmapEntry when the fingerprint matches.
	location := mapLocation{
		dir:  filepath.Dir(cleaned),
		file: filepath.Base(cleaned),
	}
	if location.dir == "." {
		location.dir = ""
	}

	bMap, corrupt := importBeatmap(modMap{location: location}, cleaned)
	if bMap == nil {
		if corrupt {
			return nil, fmt.Errorf("direct beatmap load rejected %q", cleaned)
		}

		return nil, fmt.Errorf("direct beatmap load could not read %q", cleaned)
	}

	if bMap.Mode != 0 {
		return nil, fmt.Errorf("beatmap mode %d is not supported", bMap.Mode)
	}

	return bMap, nil
}

// LookupBeatmapEntry returns the committed catalog row for one source-relative
// location, or nil when the row is absent or cannot be read. It performs a
// single indexed query instead of a library scan, so a directly loaded map can
// preserve local play state without waiting for reconciliation. All failures
// are best-effort misses, never errors, because gameplay must not depend on
// the launcher's concurrent catalog writes.
func LookupBeatmapEntry(dir, file string) *BeatmapEntry {
	if dbFile == nil || file == "" {
		return nil
	}

	entries, err := queryCatalogEntriesContext(
		context.Background(),
		"WHERE dir = ? COLLATE NOCASE AND file = ? COLLATE NOCASE",
		dir, file,
	)
	if err != nil || len(entries) == 0 {
		return nil
	}

	return entries[0]
}

// cleanDirectBeatmapPath validates a source-relative map location with the
// same traversal rules as catalog rows. The gameplay child receives this path
// from the launcher over the process boundary, so absolute paths and parent
// escapes are rejected before any file is opened.
func cleanDirectBeatmapPath(relativePath string) (string, error) {
	trimmed := strings.TrimSpace(relativePath)
	if trimmed == "" {
		return "", fmt.Errorf("empty beatmap path")
	}

	if filepath.IsAbs(trimmed) || isAbsoluteCatalogPath(trimmed) {
		return "", fmt.Errorf("invalid beatmap path %q", relativePath)
	}

	slashed := filepath.ToSlash(trimmed)
	if slashed == "." || slashed == ".." || strings.HasPrefix(slashed, "../") || strings.Contains(slashed, "/../") || strings.HasSuffix(slashed, "/..") {
		return "", fmt.Errorf("invalid beatmap path %q", relativePath)
	}

	file := filepath.Base(trimmed)
	if file == "" || file == "." || file == ".." || filepath.Base(file) != file || strings.ContainsAny(file, `/\`) {
		return "", fmt.Errorf("invalid beatmap filename %q", relativePath)
	}

	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid beatmap path %q", relativePath)
	}

	return cleaned, nil
}
