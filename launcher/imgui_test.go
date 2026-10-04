package launcher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AllenDang/cimgui-go/imgui"
)

func TestFontAtlasOwnsBothFontBuffersWithDefaultSizing(t *testing.T) {
	oldIO := ImIO
	context := imgui.CreateContext()
	ImIO = imgui.CurrentIO()
	t.Cleanup(func() {
		imgui.DestroyContextV(context)
		ImIO = oldIO
	})
	for _, file := range []string{"Quicksand-Bold.ttf", "Font Awesome 6 Free-Solid-900.otf"} {
		// Check each asset as the atlas's first source so this assertion uses
		// a native configuration directly instead of indexing binding wrappers
		ImIO.Fonts().ClearFonts()
		data, err := os.ReadFile(filepath.Join("..", "assets", "fonts", file))
		if err != nil {
			t.Fatal(err)
		}
		font := addFontData(data, false)
		if font == nil || font.LegacySize() != 0 {
			t.Fatalf("font %q unexpectedly forced a legacy pixel size", file)
		}
		// The helper destroys its temporary config; the atlas must keep a
		// copied configuration and the native font data until destruction
		source := ImIO.Fonts().Sources().Data
		if font.Sources().Size != 1 || !source.FontDataOwnedByAtlas() || source.MergeMode() || source.SizePixels() != 0 {
			t.Fatalf("font %q has unexpected source ownership or merge mode", file)
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "assets", "fonts", "Quicksand-Bold.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	addFontData(data, false)
	if ImIO.Fonts().Fonts().Size != 2 {
		t.Fatal("launcher fonts were not kept as two separate fonts")
	}
}
