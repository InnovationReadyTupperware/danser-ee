package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		stream  string
		commit  string
		dirty   bool
		branch  string
		want    string
	}{
		{
			name:    "development version names the branch and commit",
			version: "dev",
			stream:  "Dev",
			commit:  "abcdef123456",
			branch:  "main",
			want:    "main-abcdef1",
		},
		{
			name:    "feature branch is used verbatim",
			version: "dev",
			stream:  "Dev",
			commit:  "abcdef123456",
			branch:  "feat/some-random-feature",
			want:    "feat/some-random-feature-abcdef1",
		},
		{
			name:    "development version marks uncommitted changes",
			version: "dev",
			stream:  "Dev",
			commit:  "abcdef123456",
			dirty:   true,
			branch:  "main",
			want:    "main-abcdef1-dirty",
		},
		{
			name:    "short development commit is not padded",
			version: "dev",
			stream:  "Dev",
			commit:  "abc",
			branch:  "main",
			want:    "main-abc",
		},
		{
			name:    "missing commit falls back to the branch",
			version: "dev",
			stream:  "Dev",
			commit:  "",
			branch:  "main",
			want:    "main",
		},
		{
			name:    "placeholder commit falls back to the branch",
			version: "dev",
			stream:  "Dev",
			commit:  "Unknown",
			dirty:   true,
			branch:  "main",
			want:    "main",
		},
		{
			name:    "missing branch falls back to dev label",
			version: "dev",
			stream:  "Dev",
			commit:  "abcdef123456",
			branch:  "",
			want:    "dev-abcdef1",
		},
		{
			name:    "missing commit and branch fall back to bare dev",
			version: "dev",
			stream:  "Dev",
			commit:  "",
			branch:  "",
			want:    "dev",
		},
		{
			name:    "release version is exact",
			version: "1.0.0",
			stream:  "Release",
			commit:  "abcdef123456",
			branch:  "main",
			want:    "1.0.0",
		},
		{
			name:    "release prerelease version is exact",
			version: "1.0.0-rc.1+build.7",
			stream:  "Release",
			commit:  "abcdef123456",
			branch:  "main",
			want:    "1.0.0-rc.1+build.7",
		},
		{
			name:    "explicit version on dev stream is exact",
			version: "1.0.0",
			stream:  "Dev",
			commit:  "abcdef123456",
			dirty:   true,
			branch:  "main",
			want:    "1.0.0",
		},
		{
			name:    "release stream suppresses development suffix",
			version: "dev",
			stream:  "Release",
			commit:  "abcdef123456",
			branch:  "main",
			want:    "dev",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatVersion(test.version, test.stream, test.commit, test.dirty, test.branch); got != test.want {
				t.Fatalf("formatVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestShortHash(t *testing.T) {
	tests := []struct {
		name   string
		commit string
		want   string
	}{
		{name: "full revision is shortened", commit: "abcdef123456", want: "abcdef1"},
		{name: "short revision passes through", commit: "abc", want: "abc"},
		{name: "empty revision is empty", commit: "", want: ""},
		{name: "placeholder revision is empty", commit: "Unknown", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shortHash(test.commit); got != test.want {
				t.Fatalf("shortHash() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveBranch(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		detected string
		want     string
	}{
		{name: "explicit branch wins", explicit: "release", detected: "main", want: "release"},
		{name: "detected branch is used", explicit: "", detected: "feat/some-random-feature", want: "feat/some-random-feature"},
		{name: "fallback is dev", explicit: "", detected: "", want: "dev"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveBranch(test.explicit, test.detected); got != test.want {
				t.Fatalf("resolveBranch() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDetectBranch(t *testing.T) {
	writeHEAD := func(t *testing.T, gitDir, content string) {
		t.Helper()
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatalf("create git dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(content), 0o644); err != nil {
			t.Fatalf("write HEAD: %v", err)
		}
	}

	t.Run("branch is detected from a nested directory", func(t *testing.T) {
		root := t.TempDir()
		writeHEAD(t, filepath.Join(root, ".git"), "ref: refs/heads/feat/some-random-feature\n")
		nested := filepath.Join(root, "app", "utils")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("create nested dir: %v", err)
		}
		if got := detectBranch(nested); got != "feat/some-random-feature" {
			t.Fatalf("detectBranch() = %q, want %q", got, "feat/some-random-feature")
		}
	})

	t.Run("gitdir pointer is followed", func(t *testing.T) {
		root := t.TempDir()
		writeHEAD(t, filepath.Join(root, "real-git"), "ref: refs/heads/main\n")
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: real-git\n"), 0o644); err != nil {
			t.Fatalf("write git pointer: %v", err)
		}
		if got := detectBranch(root); got != "main" {
			t.Fatalf("detectBranch() = %q, want %q", got, "main")
		}
	})

	t.Run("detached checkout yields empty", func(t *testing.T) {
		root := t.TempDir()
		writeHEAD(t, filepath.Join(root, ".git"), "abcdef1234567890abcdef1234567890abcdef12\n")
		if got := detectBranch(root); got != "" {
			t.Fatalf("detectBranch() = %q, want %q", got, "")
		}
	})

	t.Run("missing checkout yields empty", func(t *testing.T) {
		if got := detectBranch(t.TempDir()); got != "" {
			t.Fatalf("detectBranch() = %q, want %q", got, "")
		}
	})
}
