package files

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileMap resolves related assets below one directory without recursively
// enumerating the entire tree during construction. This matters for skins,
// where most sessions use only a small fraction of a large skin directory.
type FileMap struct {
	path string

	mu           sync.RWMutex
	pathCache    map[string]string
	missing      map[string]struct{}
	directoryMap map[string]map[string]string
	allScanned   bool
}

// NewFileMap creates a lazy resolver rooted at path. The root must already
// exist and be a directory; child directories are inspected only when a file
// below them is requested.
func NewFileMap(path string) (*FileMap, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "readdir", Path: path, Err: os.ErrInvalid}
	}

	fileMap := &FileMap{
		path:         filepath.Clean(path),
		pathCache:    make(map[string]string),
		missing:      make(map[string]struct{}),
		directoryMap: make(map[string]map[string]string),
	}

	return fileMap, nil
}

// GetFile resolves path case-insensitively below the map root. The returned
// path uses the casing found on disk, which keeps downstream asset loaders
// portable to case-sensitive filesystems.
func (f *FileMap) GetFile(path string) (string, error) {
	if f == nil {
		return "", os.ErrNotExist
	}

	relative := f.relativePath(path)
	if relative == "" {
		return "", os.ErrNotExist
	}

	f.mu.RLock()
	if resolved, ok := f.pathCache[relative]; ok {
		f.mu.RUnlock()
		return filepath.Join(f.path, filepath.FromSlash(resolved)), nil
	}
	if _, ok := f.missing[relative]; ok {
		f.mu.RUnlock()
		return "", os.ErrNotExist
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	if resolved, ok := f.pathCache[relative]; ok {
		return filepath.Join(f.path, filepath.FromSlash(resolved)), nil
	}
	if _, ok := f.missing[relative]; ok {
		return "", os.ErrNotExist
	}

	resolved, ok := f.resolveLocked(relative)
	if !ok {
		f.missing[relative] = struct{}{}
		return "", os.ErrNotExist
	}

	f.pathCache[relative] = resolved
	return filepath.Join(f.path, filepath.FromSlash(resolved)), nil
}

// GetMap explicitly builds a complete recursive cache and returns it. Callers
// should use GetFile when they need only one asset; this method remains for
// consumers such as custom beatmap sample loading that need every file.
func (f *FileMap) GetMap() map[string]string {
	if f == nil {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.allScanned {
		results, _ := SearchFiles(f.path, "*", -1)
		root := filepath.Clean(f.path)
		for _, result := range results {
			relative, err := filepath.Rel(root, result)
			if err != nil {
				continue
			}
			fixedPath := filepath.ToSlash(relative)
			key := strings.ToLower(fixedPath)
			f.pathCache[key] = fixedPath
			delete(f.missing, key)
		}
		f.allScanned = true
	}

	retMap := make(map[string]string, len(f.pathCache))
	for key, value := range f.pathCache {
		retMap[key] = filepath.Join(f.path, filepath.FromSlash(value))
	}

	return retMap
}

func (f *FileMap) relativePath(path string) string {
	value := filepath.ToSlash(path)
	root := filepath.ToSlash(f.path)
	if strings.HasPrefix(strings.ToLower(value), strings.ToLower(root)+"/") {
		value = value[len(root)+1:]
	}
	value = strings.TrimLeft(value, "/")
	value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if value == "." || value == ".." || strings.HasPrefix(value, "../") || filepath.IsAbs(value) {
		return ""
	}

	return strings.ToLower(value)
}

func (f *FileMap) resolveLocked(relative string) (string, bool) {
	current := f.path
	resolved := make([]string, 0, strings.Count(relative, "/")+1)

	for _, component := range strings.Split(relative, "/") {
		entries, ok := f.directoryEntriesLocked(current)
		if !ok {
			return "", false
		}

		actual, ok := entries[component]
		if !ok {
			return "", false
		}

		resolved = append(resolved, actual)
		current = filepath.Join(current, actual)
	}

	info, err := os.Stat(current)
	if err != nil || info.IsDir() {
		return "", false
	}

	return filepath.ToSlash(filepath.Join(resolved...)), true
}

func (f *FileMap) directoryEntriesLocked(directory string) (map[string]string, bool) {
	key := strings.ToLower(filepath.Clean(directory))
	if entries, ok := f.directoryMap[key]; ok {
		return entries, true
	}

	readEntries, err := os.ReadDir(directory)
	if err != nil {
		return nil, false
	}

	entries := make(map[string]string, len(readEntries))
	for _, entry := range readEntries {
		entries[strings.ToLower(entry.Name())] = entry.Name()
	}
	f.directoryMap[key] = entries

	return entries, true
}
