package font

import (
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	fontpkg "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

func testOutlineFont(t *testing.T, chars string) (*Font, fontpkg.Face) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "assets", "fonts", "Ubuntu-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	ttf, err := opentype.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{Size: 128, DPI: 72, Hinting: fontpkg.HintingFull})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = face.Close() })

	f := &Font{
		initialSize: 128,
		sfntFont:    ttf,
		glyphs:      make(map[rune]*glyphData),
		glyphIndex:  make(map[rune]sfnt.GlyphIndex),
		kernCache:   make(map[[2]rune]float64),
	}
	var buf sfnt.Buffer
	for _, r := range chars {
		index, err := ttf.GlyphIndex(&buf, r)
		if err != nil || index == 0 {
			t.Fatalf("test font lacks %q: %v", r, err)
		}
		f.glyphIndex[r] = index
		f.glyphs[r] = &glyphData{advance: 10}
	}
	return f, face
}

func TestOutlineKerningMatchesOpenTypeFace(t *testing.T) {
	f, face := testOutlineFont(t, "AVToWgé")
	for _, pair := range [][2]rune{{'A', 'V'}, {'T', 'o'}, {'W', 'A'}, {'g', 'g'}, {'é', 'V'}} {
		want := float64(face.Kern(pair[0], pair[1])) / 64
		got := f.kerning(pair[0], pair[1])
		if got != want {
			t.Errorf("kerning(%q, %q) = %v, want %v", pair[0], pair[1], got, want)
		}
		if repeated := f.kerning(pair[0], pair[1]); repeated != want {
			t.Errorf("cached kerning(%q, %q) = %v, want %v", pair[0], pair[1], repeated, want)
		}
	}
	if got := len(f.kernCache); got != 5 {
		t.Fatalf("cache contains %d pairs, want all five including zero-valued pairs", got)
	}
	if got := f.kerning('☃', 'A'); got != 0 {
		t.Fatalf("missing glyph kerning = %v, want 0", got)
	}
	if got := len(f.kernCache); got != 6 {
		t.Fatalf("missing glyph pair was not cached: got %d pairs, want 6", got)
	}
	if got := f.kerning('☃', 'A'); got != 0 || len(f.kernCache) != 6 {
		t.Fatalf("repeated missing glyph kerning = %v with %d entries", got, len(f.kernCache))
	}
}

func TestGetWidthUsesPreviousUnicodeRune(t *testing.T) {
	f, face := testOutlineFont(t, "AVToW")

	// Choose an actual nonzero kerning pair, then give its first glyph a
	// multibyte rune. This detects accidental indexing into UTF-8 bytes.
	var first, second rune
	var expectedKern float64
	for _, pair := range [][2]rune{{'A', 'V'}, {'T', 'o'}, {'W', 'A'}, {'T', 'A'}} {
		if v := float64(face.Kern(pair[0], pair[1])) / 64; v != 0 {
			first, second, expectedKern = pair[0], pair[1], v
			break
		}
	}
	if expectedKern == 0 {
		t.Fatal("test font has no nonzero kerning pair")
	}
	f.glyphIndex['é'] = f.glyphIndex[first]
	f.glyphs['é'] = &glyphData{advance: 10}

	text := string([]rune{'é', second})
	if got, want := f.GetWidth(128, text), 20+expectedKern; math.Abs(got-want) > 1e-9 {
		t.Errorf("width(%q) = %v, want %v", text, got, want)
	}
	if got, want := f.GetWidthMonospaced(128, text), 20+expectedKern; math.Abs(got-want) > 1e-9 {
		t.Errorf("monospaced width(%q) = %v, want %v", text, got, want)
	}
}

func TestOutlineKerningConcurrentWidths(t *testing.T) {
	f, _ := testOutlineFont(t, "AVToWgé")
	texts := []string{"AVATAR", "ToTo", "WAVE", "éV", "gogo"}
	wants := make([]float64, len(texts))
	for i, text := range texts {
		wants[i] = f.GetWidth(128, text)
	}
	clear(f.kernCache) // Exercise concurrent cache misses as well as cache hits

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				for i, text := range texts {
					if got := f.GetWidth(128, text); got != wants[i] {
						t.Errorf("concurrent width(%q) = %v, want %v", text, got, wants[i])
					}
				}
			}
		}()
	}
	wg.Wait()
}

func TestTextureFontHasNoKerning(t *testing.T) {
	// Texture fonts have neither an sfntFont nor a kerning cache.
	f := &Font{}
	if got := f.kerning('A', 'V'); got != 0 {
		t.Errorf("texture font kerning = %v, want 0", got)
	}
}
