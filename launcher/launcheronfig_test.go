package launcher

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilePathUsesRelativeJSONName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "violet.JSON")

	if got, want := profilePath(root, path), "nested/violet"; got != want {
		t.Fatalf("profilePath() = %q, want %q", got, want)
	}
}

func TestProfileFilePathRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"..", filepath.Join("..", "outside"), filepath.Join("nested", "..", "..", "outside")} {
		if path, err := profileFilePathIn(root, name); err == nil {
			t.Fatalf("profileFilePath(%q) = %q, want an error", name, path)
		}
	}
}

func TestProfileFilePathKeepsProfilesInsideConfigDirectory(t *testing.T) {
	root := t.TempDir()
	path, err := profileFilePathIn(root, "nested/violet")
	if err != nil {
		t.Fatal(err)
	}

	relative, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		t.Fatalf("profile path escaped config directory: root=%q path=%q", root, path)
	}
	if filepath.Ext(path) != ".json" {
		t.Fatalf("profile path extension = %q, want .json", filepath.Ext(path))
	}
}
