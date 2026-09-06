package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type scanResult struct {
	candidates       []modMap
	complete         bool
	directoriesSeen  int
	beatmapFilesSeen int
}

// scanBeatmapFiles follows Stable's set layout without walking every child
// directory after a set has been identified. It also refuses symlinks: the
// Songs root is user-controlled input, so following links could escape the
// configured library or create cycles.
func scanBeatmapFiles(
	root string,
	skipCached bool,
	cachedFolders map[string]uint8,
	mustCheckDirs []string,
	progress func(directoriesSeen int),
) (scanResult, error) {
	return scanBeatmapFilesContext(context.Background(), root, skipCached, cachedFolders, mustCheckDirs, progress)
}

// scanBeatmapFilesContext is the cancellable implementation used by the
// launcher's background catalog worker. Directory enumeration can be slow on
// removable or mechanical drives, so cancellation must be checked between
// directories and entries instead of waiting for a complete walk to finish.
func scanBeatmapFilesContext(
	ctx context.Context,
	root string,
	skipCached bool,
	cachedFolders map[string]uint8,
	mustCheckDirs []string,
	progress func(directoriesSeen int),
) (scanResult, error) {
	if ctx == nil {
		return scanResult{}, fmt.Errorf("nil context")
	}

	result := scanResult{complete: true}
	mustCheck := make(map[string]struct{}, len(mustCheckDirs))
	for _, dir := range mustCheckDirs {
		mustCheck[normalizeRelativePath(dir)] = struct{}{}
	}

	var visit func(string, string, int) error
	visit = func(directory, relativeDirectory string, level int) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		if skipCached && level > 0 {
			normalized := normalizeRelativePath(relativeDirectory)
			if _, required := mustCheck[normalized]; !required {
				if _, cached := cachedFolders[normalized]; cached {
					return nil
				}
			}
		}

		entries, err := os.ReadDir(directory)
		if err != nil {
			result.complete = false
			if level == 0 {
				if os.IsNotExist(err) {
					log.Printf("DatabaseManager: Warning: Songs directory %q is missing, continuing with an empty library", directory)
					return nil
				}
				return fmt.Errorf("read Songs directory: %w", err)
			}

			log.Printf("DatabaseManager: Cannot enumerate %q: %v", directory, err)
			return nil
		}

		result.directoriesSeen++
		if progress != nil {
			progress(result.directoriesSeen)
		}
		hasBeatmap := false
		directories := make([]os.DirEntry, 0)

		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}

			if entry.Type()&os.ModeSymlink != 0 {
				// Symlink targets are deliberately outside the catalog's source
				// boundary. Treating one as an incomplete observation prevents a
				// full scan from deleting a cached row that may still be reachable
				// through that link.
				result.complete = false
				continue
			}

			if entry.IsDir() {
				directories = append(directories, entry)
				continue
			}

			if level == 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".osu") {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				result.complete = false
				log.Printf("DatabaseManager: Cannot inspect %q: %v", filepath.Join(directory, entry.Name()), err)
				continue
			}

			relativePath, err := filepath.Rel(root, filepath.Join(directory, entry.Name()))
			if err != nil {
				result.complete = false
				log.Printf("DatabaseManager: Cannot make relative path for %q: %v", entry.Name(), err)
				continue
			}

			result.candidates = append(result.candidates, modMap{
				location: mapLocation{
					dir:  normalizeRelativeDirPreserveCase(filepath.Dir(relativePath)),
					file: entry.Name(),
				},
				fingerprint: fileFingerprint{
					modified: info.ModTime().UnixNano() / int64(1e6),
					size:     info.Size(),
				},
			})
			result.beatmapFilesSeen++
			hasBeatmap = true
		}

		// Stable stores maps in set directories. Preserve the historical
		// behavior of ignoring stray .osu files directly under Songs; more
		// importantly, a stray root file must not prevent the real set
		// directories from being visited.
		if hasBeatmap && level > 0 {
			return nil
		}

		for _, entry := range directories {
			if err := ctx.Err(); err != nil {
				return err
			}

			nextRelative := entry.Name()
			if relativeDirectory != "" {
				nextRelative = filepath.Join(relativeDirectory, entry.Name())
			}

			if err := visit(filepath.Join(directory, entry.Name()), nextRelative, level+1); err != nil {
				return err
			}
		}

		return nil
	}

	if err := visit(root, "", 0); err != nil {
		return result, err
	}

	return result, nil
}

func normalizeRelativePath(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = filepath.ToSlash(filepath.Clean(value))
	if value == "." {
		return ""
	}

	return strings.ToLower(value)
}

// normalizeRelativeDirPreserveCase normalizes separators for a catalog
// location without changing its spelling. The case-insensitive catalog key
// remains the identity; the stored directory must keep the source
// filesystem's casing so case-sensitive platforms can still open the file
// and a later scan does not mistake every mixed-case set for a rename.
func normalizeRelativeDirPreserveCase(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = filepath.ToSlash(filepath.Clean(value))
	if value == "." {
		return ""
	}

	return value
}
