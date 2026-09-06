package main

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// modeFileInfo reports a fixed name and mode so header construction can be
// exercised on filesystems that cannot represent every permission bit.
type modeFileInfo struct {
	name string
	mode fs.FileMode
}

func (m modeFileInfo) Name() string       { return m.name }
func (m modeFileInfo) Size() int64        { return 0 }
func (m modeFileInfo) Mode() fs.FileMode  { return m.mode }
func (m modeFileInfo) ModTime() time.Time { return time.Time{} }
func (m modeFileInfo) IsDir() bool        { return false }
func (m modeFileInfo) Sys() any           { return nil }

// Permission bits on the source file must reach the archive entry.
// Extractors restore the mode they find there, so this is what keeps the
// launcher and gameplay binaries runnable after unzipping a release.
func TestEntryHeaderPreservesPermissionBits(t *testing.T) {
	for _, mode := range []fs.FileMode{0o755, 0o644} {
		header, err := entryHeader("danser-cli", modeFileInfo{name: "danser-cli", mode: mode})
		if err != nil {
			t.Fatalf("entryHeader(%v): %v", mode, err)
		}
		if got := header.Mode().Perm(); got != mode {
			t.Errorf("entryHeader(%v) stored permission bits %v", mode, got)
		}
		if header.Method != zip.Deflate {
			t.Errorf("entryHeader(%v) method = %v, want deflated", mode, header.Method)
		}
	}
}

// Packing a build directory and reading it back must keep executable
// entries executable, including entries nested in subdirectories such as
// the bundled ffmpeg binaries. Skipped on Windows, whose filesystems
// cannot carry the executable bit into the fixture in the first place.
func TestPackRoundTripPreservesExecutableBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows filesystems cannot represent executable bits")
	}

	src := t.TempDir()
	writeFixture(t, filepath.Join(src, "danser-cli"), "#!/bin/sh\n", 0o755)
	writeFixture(t, filepath.Join(src, "assets.dpak"), "assets", 0o644)
	writeFixture(t, filepath.Join(src, "ffmpeg", "bin", "ffmpeg"), "ffmpeg", 0o755)

	dst := filepath.Join(t.TempDir(), "danser-test-linux.zip")
	if err := pack(dst, src); err != nil {
		t.Fatalf("pack: %v", err)
	}

	want := map[string]fs.FileMode{
		"danser-cli":        0o755,
		"assets.dpak":       0o644,
		"ffmpeg/bin/ffmpeg": 0o755,
	}

	reader, err := zip.OpenReader(dst)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()

	got := make(map[string]fs.FileMode, len(reader.File))
	for _, f := range reader.File {
		got[f.Name] = f.Mode().Perm()
	}

	for name, mode := range want {
		perm, ok := got[name]
		if !ok {
			t.Errorf("archive is missing entry %q", name)
			continue
		}
		if perm != mode {
			t.Errorf("entry %q permission bits = %v, want %v", name, perm, mode)
		}
	}
}

// A missing source directory must fail the pack instead of crashing on a
// nil file description.
func TestPackMissingSourceReturnsError(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "danser-test-linux.zip")
	if err := pack(dst, filepath.Join(t.TempDir(), "no-such-dir")); err == nil {
		t.Error("pack with missing source directory succeeded, want error")
	}
}

func writeFixture(t *testing.T, path, content string, mode fs.FileMode) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod fixture: %v", err)
	}
}
