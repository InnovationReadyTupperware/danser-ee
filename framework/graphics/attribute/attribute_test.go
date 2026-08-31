package attribute

import (
	"testing"

	"github.com/go-gl/gl/v4.5-core/gl"
)

func TestIntegerAttributeTypes(t *testing.T) {
	tests := []struct {
		name     string
		attrType Type
		wantGL   int
	}{
		{name: "signed integer", attrType: Int, wantGL: gl.INT},
		{name: "unsigned integer", attrType: UInt, wantGL: gl.UNSIGNED_INT},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.attrType.IsInteger() {
				t.Fatalf("IsInteger() = false, want true")
			}

			if got := tt.attrType.InternalType(); got != tt.wantGL {
				t.Fatalf("InternalType() = %d, want %d", got, tt.wantGL)
			}
		})
	}
}

func TestMatrixAttributeTypes(t *testing.T) {
	for _, attrType := range []Type{Mat2, Mat23, Mat24, Mat3, Mat32, Mat34, Mat4, Mat42, Mat43} {
		if !attrType.IsMatrix() {
			t.Fatalf("IsMatrix() = false for type %d", attrType)
		}
	}
}
