package gcontext

import "testing"

func TestSupportsOpenGL45(t *testing.T) {
	tests := []struct {
		name         string
		major, minor int32
		want         bool
	}{
		{name: "minimum", major: 4, minor: 5, want: true},
		{name: "newer minor", major: 4, minor: 6, want: true},
		{name: "newer major", major: 5, minor: 0, want: true},
		{name: "older", major: 4, minor: 4},
		{name: "legacy", major: 3, minor: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportsOpenGL45(tt.major, tt.minor); got != tt.want {
				t.Fatalf("supportsOpenGL45() = %t, want %t", got, tt.want)
			}
		})
	}
}
