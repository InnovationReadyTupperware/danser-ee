package gcontext

import (
	"slices"
	"testing"
)

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

func TestGLConfigs(t *testing.T) {
	full := glConfig{msaa: true, srgb: true}
	plain := glConfig{srgb: true}
	bare := glConfig{}

	tests := []struct {
		name        string
		requestMSAA bool
		want        []glConfig
	}{
		{name: "requested multisampling", requestMSAA: true, want: []glConfig{full, plain, bare}},
		{name: "no multisampling", requestMSAA: false, want: []glConfig{plain, bare}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := glConfigs(tt.requestMSAA); !slices.Equal(got, tt.want) {
				t.Errorf("glConfigs(%t) = %v, want %v", tt.requestMSAA, got, tt.want)
			}
		})
	}
}
