package utils

import "testing"

func TestNormalizeSemVer(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{name: "stable", version: "1.0.0", want: "v1.0.0"},
		{name: "prerelease", version: "1.0.0-rc.1", want: "v1.0.0-rc.1"},
		{name: "build metadata", version: "1.0.0+build.7", want: "v1.0.0+build.7"},
		{name: "optional tag prefix", version: "v1.0.0", want: "v1.0.0"},
		{name: "missing patch", version: "1.0", wantErr: true},
		{name: "leading zero", version: "01.0.0", wantErr: true},
		{name: "leading zero prerelease number", version: "1.0.0-rc.01", wantErr: true},
		{name: "empty prerelease", version: "1.0.0-", wantErr: true},
		{name: "empty build identifier", version: "1.0.0+", wantErr: true},
		{name: "empty build segment", version: "1.0.0+build..7", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeSemVer(test.version)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeSemVer() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeSemVer() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("normalizeSemVer() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCompareSemVerVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    int
		wantErr bool
	}{
		{name: "alpha precedes alpha one", current: "1.0.0-alpha", latest: "1.0.0-alpha.1", want: -1},
		{name: "alpha one precedes alpha beta", current: "1.0.0-alpha.1", latest: "1.0.0-alpha.beta", want: -1},
		{name: "alpha beta precedes beta", current: "1.0.0-alpha.beta", latest: "1.0.0-beta", want: -1},
		{name: "beta precedes beta two", current: "1.0.0-beta", latest: "1.0.0-beta.2", want: -1},
		{name: "beta two precedes beta eleven", current: "1.0.0-beta.2", latest: "1.0.0-beta.11", want: -1},
		{name: "beta eleven precedes release candidate", current: "1.0.0-beta.11", latest: "1.0.0-rc.1", want: -1},
		{name: "release candidate precedes stable", current: "1.0.0-rc.1", latest: "1.0.0", want: -1},
		{name: "build metadata does not affect precedence", current: "1.0.0+build.1", latest: "1.0.0+build.2", want: 0},
		{name: "tag prefix is accepted", current: "v1.0.0", latest: "1.0.1", want: -1},
		{name: "invalid current version", current: "1.0", latest: "1.0.0", wantErr: true},
		{name: "invalid latest version", current: "1.0.0", latest: "1.0", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := compareSemVerVersions(test.current, test.latest)
			if test.wantErr {
				if err == nil {
					t.Fatalf("compareSemVerVersions() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("compareSemVerVersions() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("compareSemVerVersions() = %d, want %d", got, test.want)
			}
		})
	}
}
