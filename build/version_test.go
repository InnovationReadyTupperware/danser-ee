package build

import "testing"

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		stream  string
		commit  string
		want    string
	}{
		{
			name:    "development version includes a short commit",
			version: "dev",
			stream:  "Dev",
			commit:  "abcdef123456",
			want:    "dev-abcdef1",
		},
		{
			name:    "short development commit is not padded",
			version: "dev",
			stream:  "Dev",
			commit:  "abc",
			want:    "dev-abc",
		},
		{
			name:    "release version is exact",
			version: "1.0.0",
			stream:  "Release",
			commit:  "abcdef123456",
			want:    "1.0.0",
		},
		{
			name:    "release prerelease version is exact",
			version: "1.0.0-rc.1+build.7",
			stream:  "Release",
			commit:  "abcdef123456",
			want:    "1.0.0-rc.1+build.7",
		},
		{
			name:    "release stream suppresses development suffix",
			version: "dev",
			stream:  "Release",
			commit:  "abcdef123456",
			want:    "dev",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatVersion(test.version, test.stream, test.commit); got != test.want {
				t.Fatalf("formatVersion() = %q, want %q", got, test.want)
			}
		})
	}
}
