package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileMapResolvesFilesLazily(t *testing.T) {
	root := t.TempDir()
	writeFileMapFixture(t, filepath.Join(root, "Textures", "HitCircle.PNG"))
	writeFileMapFixture(t, filepath.Join(root, "samples", "Hitnormal.wav"))

	fileMap, err := NewFileMap(root)
	if err != nil {
		t.Fatal(err)
	}
	if fileMap.allScanned {
		t.Fatal("NewFileMap performed a full recursive scan")
	}

	resolved, err := fileMap.GetFile("textures/hitcircle.png")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(resolved) != "HitCircle.PNG" {
		t.Fatalf("resolved file = %q, want original casing", resolved)
	}
	if fileMap.allScanned {
		t.Fatal("GetFile performed a full recursive scan")
	}

	if _, err = fileMap.GetFile("missing.wav"); !os.IsNotExist(err) {
		t.Fatalf("missing file error = %v, want not-exist", err)
	}
	if _, err = fileMap.GetFile("../outside.wav"); !os.IsNotExist(err) {
		t.Fatalf("traversal error = %v, want not-exist", err)
	}
}

func TestFileMapGetMapBuildsCompleteCacheOnDemand(t *testing.T) {
	root := t.TempDir()
	writeFileMapFixture(t, filepath.Join(root, "one.txt"))
	writeFileMapFixture(t, filepath.Join(root, "nested", "two.txt"))

	fileMap, err := NewFileMap(root)
	if err != nil {
		t.Fatal(err)
	}

	files := fileMap.GetMap()
	if len(files) != 2 {
		t.Fatalf("GetMap() returned %d files, want 2", len(files))
	}
	if !fileMap.allScanned {
		t.Fatal("GetMap() did not mark the complete cache")
	}
}

func writeFileMapFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
}
