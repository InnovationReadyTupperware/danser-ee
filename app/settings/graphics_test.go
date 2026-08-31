package settings

import "testing"

func TestDefaultGraphicsMSAA(t *testing.T) {
	if got := NewConfigFile().Graphics.MSAA; got != 4 {
		t.Fatalf("default MSAA = %d, want 4", got)
	}
}
