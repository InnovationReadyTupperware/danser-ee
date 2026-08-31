package texture

import "testing"

func TestAtlasAllocatorBoundsLayers(t *testing.T) {
	allocator := newAtlasAllocator(8, 1, 1, 2)

	for i := range 2 {
		layer, x, y, ok := allocator.allocate(7, 7)
		if !ok {
			t.Fatalf("allocation %d failed", i)
		}
		if layer != i || x != 0 || y != 0 {
			t.Fatalf("allocation %d = layer %d at (%d, %d), want layer %d at origin", i, layer, x, y, i)
		}
	}

	if _, _, _, ok := allocator.allocate(1, 1); ok {
		t.Fatal("allocation succeeded after reaching the layer limit")
	}
	if len(allocator.emptySpaces) != 2 {
		t.Fatalf("allocator grew to %d layers, want 2", len(allocator.emptySpaces))
	}
}

func TestAtlasAllocatorRejectsPaddedOversize(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
	}{
		{name: "width consumes padding", width: 8, height: 1},
		{name: "height consumes padding", width: 1, height: 8},
		{name: "zero width", height: 1},
		{name: "negative height", width: 1, height: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocator := newAtlasAllocator(8, 1, 1, 2)
			if _, _, _, ok := allocator.allocate(tt.width, tt.height); ok {
				t.Fatal("allocate() succeeded, want rejection")
			}
			if len(allocator.emptySpaces) != 1 {
				t.Fatalf("rejected allocation grew to %d layers", len(allocator.emptySpaces))
			}
		})
	}
}

func TestTextureByteSize(t *testing.T) {
	tests := []struct {
		name                         string
		width, height, bytesPerPixel int
		want                         int
		valid                        bool
	}{
		{name: "RGBA image", width: 4, height: 3, bytesPerPixel: 4, want: 48, valid: true},
		{name: "zero dimension", height: 3, bytesPerPixel: 4},
		{name: "overflow", width: int(^uint(0) >> 1), height: 2, bytesPerPixel: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := textureByteSize(tt.width, tt.height, tt.bytesPerPixel)
			if got != tt.want || valid != tt.valid {
				t.Fatalf("textureByteSize() = (%d, %t), want (%d, %t)", got, valid, tt.want, tt.valid)
			}
		})
	}
}
