package utils

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/innovationreadytupperware/danser-ee/build"
)

func TestGetLatestVersion(t *testing.T) {
	t.Run("latest release is parsed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"html_url":"https://example.invalid/releases/v1.2.3","tag_name":"v1.2.3"}`))
		}))
		defer server.Close()

		url, tag, err := getLatestVersion(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("getLatestVersion() error = %v", err)
		}
		if url != "https://example.invalid/releases/v1.2.3" {
			t.Fatalf("getLatestVersion() url = %q", url)
		}
		if tag != "v1.2.3" {
			t.Fatalf("getLatestVersion() tag = %q", tag)
		}
	})

	t.Run("missing releases report ErrNoReleases", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer server.Close()

		_, _, err := getLatestVersion(context.Background(), server.URL)
		if !errors.Is(err, ErrNoReleases) {
			t.Fatalf("getLatestVersion() error = %v, want errors.Is(err, ErrNoReleases)", err)
		}
	})

	t.Run("server errors are not ErrNoReleases", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer server.Close()

		_, _, err := getLatestVersion(context.Background(), server.URL)
		if err == nil {
			t.Fatalf("getLatestVersion() error = nil, want an error")
		}
		if errors.Is(err, ErrNoReleases) {
			t.Fatalf("getLatestVersion() error = %v, must not match ErrNoReleases", err)
		}
	})

	t.Run("invalid payload is an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`not json`))
		}))
		defer server.Close()

		if _, _, err := getLatestVersion(context.Background(), server.URL); err == nil {
			t.Fatalf("getLatestVersion() error = nil, want an error")
		}
	})

	t.Run("nil context is an error", func(t *testing.T) {
		if _, _, err := getLatestVersion(nil, "https://example.invalid"); err == nil {
			t.Fatalf("getLatestVersion() error = nil, want an error")
		}
	})
}

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

func TestSelectLatestEligibleRelease(t *testing.T) {
	releases := []githubRelease{
		{Tag: "1.2.0", URL: "https://example.invalid/1.2.0"},
		{Tag: "1.3.0-rc.1", URL: "https://example.invalid/1.3.0-rc.1", Prerelease: true},
		{Tag: "1.3.0-beta.100", URL: "https://example.invalid/1.3.0-beta.100", Prerelease: true},
		{Tag: "1.3.0-alpha.14", URL: "https://example.invalid/1.3.0-alpha.14", Prerelease: true},
	}

	tests := []struct {
		name              string
		current           string
		includePrerelease bool
		want              string
	}{
		{name: "stable stays on stable channel", current: "1.1.0", want: "1.2.0"},
		{name: "stable can opt into prereleases", current: "1.1.0", includePrerelease: true, want: "1.3.0-rc.1"},
		{name: "prerelease off returns to stable release line", current: "1.2.0-beta.2", want: "1.2.0"},
		{name: "prerelease on can advance to later release line", current: "1.2.0-beta.2", includePrerelease: true, want: "1.3.0-rc.1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, got, found, err := selectLatestEligibleRelease(test.current, releases, test.includePrerelease)
			if err != nil {
				t.Fatalf("selectLatestEligibleRelease() error = %v", err)
			}
			if !found {
				t.Fatal("selectLatestEligibleRelease() found = false")
			}
			if got != "v"+test.want {
				t.Fatalf("selectLatestEligibleRelease() = %q, want %q", got, "v"+test.want)
			}
		})
	}
}

func TestSelectLatestEligibleReleaseKeepsPrereleaseWithinBaseVersion(t *testing.T) {
	releases := []githubRelease{
		{Tag: "1.2.0-rc.3", Prerelease: true},
		{Tag: "1.2.0-beta.100", Prerelease: true},
		{Tag: "1.2.0-alpha.14", Prerelease: true},
		{Tag: "1.3.0-rc.1", Prerelease: true},
	}

	_, got, found, err := selectLatestEligibleRelease("1.2.0-beta.2", releases, false)
	if err != nil {
		t.Fatalf("selectLatestEligibleRelease() error = %v", err)
	}
	if !found {
		t.Fatal("selectLatestEligibleRelease() found = false")
	}
	if got != "v1.2.0-rc.3" {
		t.Fatalf("selectLatestEligibleRelease() = %q, want %q", got, "v1.2.0-rc.3")
	}
}

func TestCheckForUpdateDetailsIncludesLatestVersion(t *testing.T) {
	originalEndpoint := githubReleasesURL
	originalVersion := build.Version
	originalStream := build.Stream
	t.Cleanup(func() {
		githubReleasesURL = originalEndpoint
		build.Version = originalVersion
		build.Stream = originalStream
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"html_url":"https://example.invalid/releases/1.2.0","tag_name":"1.2.0"}]`))
	}))
	defer server.Close()

	githubReleasesURL = server.URL
	build.Version = "1.1.0"
	build.Stream = "Release"

	result, err := CheckForUpdateDetailsContext(t.Context())
	if err != nil {
		t.Fatalf("CheckForUpdateDetailsContext() error = %v", err)
	}
	if result.Status != UpdateAvailable {
		t.Fatalf("result.Status = %v, want %v", result.Status, UpdateAvailable)
	}
	if result.LatestVersion != "1.2.0" {
		t.Fatalf("result.LatestVersion = %q, want %q", result.LatestVersion, "1.2.0")
	}
	if result.ReleaseURL != "https://example.invalid/releases/1.2.0" {
		t.Fatalf("result.ReleaseURL = %q", result.ReleaseURL)
	}
}

func TestCheckForUpdateDetailsPrereleasePolicy(t *testing.T) {
	originalEndpoint := githubReleasesURL
	originalVersion := build.Version
	originalStream := build.Stream
	t.Cleanup(func() {
		githubReleasesURL = originalEndpoint
		build.Version = originalVersion
		build.Stream = originalStream
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"html_url":"https://example.invalid/releases/1.2.0","tag_name":"1.2.0"},
			{"html_url":"https://example.invalid/releases/1.3.0-rc.1","tag_name":"1.3.0-rc.1","prerelease":true}
		]`))
	}))
	defer server.Close()

	githubReleasesURL = server.URL
	build.Version = "1.2.0-beta.2"
	build.Stream = "Release"

	bounded, err := CheckForUpdateDetailsContext(t.Context())
	if err != nil {
		t.Fatalf("bounded CheckForUpdateDetailsContext() error = %v", err)
	}
	if bounded.Status != UpdateAvailable || bounded.LatestVersion != "1.2.0" {
		t.Fatalf("bounded result = %#v, want stable 1.2.0", bounded)
	}

	optedIn, err := CheckForUpdateDetailsWithOptionsContext(t.Context(), UpdateCheckOptions{IncludePrerelease: true})
	if err != nil {
		t.Fatalf("opted-in CheckForUpdateDetailsWithOptionsContext() error = %v", err)
	}
	if optedIn.Status != UpdateAvailable || optedIn.LatestVersion != "1.3.0-rc.1" {
		t.Fatalf("opted-in result = %#v, want 1.3.0-rc.1", optedIn)
	}
}
