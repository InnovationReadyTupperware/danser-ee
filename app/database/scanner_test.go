package database

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestScanBeatmapFilesUsesSetDirectories(t *testing.T) {
	root := t.TempDir()

	// A stray root-level file is intentionally ignored, but it must not stop
	// the scanner before it reaches normal set directories.
	writeScanFixture(t, filepath.Join(root, "stray.osu"))
	writeScanFixture(t, filepath.Join(root, "Set", "map.osu"))
	writeScanFixture(t, filepath.Join(root, "Set", "nested", "ignored.osu"))
	writeScanFixture(t, filepath.Join(root, "Collection", "Nested", "map.osu"))

	result, err := scanBeatmapFiles(root, false, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.complete {
		t.Fatal("scan was marked incomplete")
	}
	if len(result.candidates) != 2 {
		t.Fatalf("scan found %d files, want 2", len(result.candidates))
	}

	if result.candidates[0].location.dir == result.candidates[1].location.dir {
		t.Fatalf("both candidates were assigned to one directory: %+v", result.candidates)
	}
}

func TestScanBeatmapFilesCanSkipCachedSets(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, filepath.Join(root, "Set", "map.osu"))

	result, err := scanBeatmapFiles(root, true, map[string]uint8{"set": 1}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.candidates) != 0 {
		t.Fatalf("cached set produced %d candidates, want 0", len(result.candidates))
	}

	result, err = scanBeatmapFiles(root, true, map[string]uint8{"set": 1}, []string{"Set"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.candidates) != 1 {
		t.Fatalf("required cached set produced %d candidates, want 1", len(result.candidates))
	}
}

func BenchmarkScanBeatmapFiles(b *testing.B) {
	root := b.TempDir()
	for set := 0; set < 512; set++ {
		for mapIndex := 0; mapIndex < 4; mapIndex++ {
			path := filepath.Join(root, "Set"+strconv.Itoa(set), "map"+strconv.Itoa(mapIndex)+".osu")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("[TimingPoints]\n0,500\n"), 0o644); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	for range b.N {
		result, err := scanBeatmapFiles(root, false, nil, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
		if len(result.candidates) != 2048 {
			b.Fatalf("scan found %d files, want 2048", len(result.candidates))
		}
	}
}

func writeScanFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[TimingPoints]\n0,500\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
