package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/innovationreadytupperware/danser-ee/build"
	"golang.org/x/mod/semver"
)

// githubLatestReleaseURL is the endpoint consulted for packaged release
// builds. It is a variable so tests can point the check at a local server.
var githubLatestReleaseURL = "https://api.github.com/repos/innovationreadytupperware/danser-ee/releases/latest"

// ErrNoReleases reports that GitHub has no release information for the
// repository. The repository may be private, unreachable without
// credentials, or simply not publishing releases yet. Callers detect it
// with errors.Is to degrade gracefully instead of treating it as a
// transport failure.
var ErrNoReleases = errors.New("no releases published for this repository")

// GetLatestVersionFromGitHub makes a request to GitHub and returns url and tag of the latest version found
func GetLatestVersionFromGitHub() (url string, tag string, err error) {
	return getLatestVersion(context.Background(), githubLatestReleaseURL)
}

// GetLatestVersionFromGitHubContext makes a cancellable request to GitHub and
// returns the latest release URL and tag. The timeout protects launcher
// shutdown from a network stack that never completes a request.
func GetLatestVersionFromGitHubContext(ctx context.Context) (url string, tag string, err error) {
	return getLatestVersion(ctx, githubLatestReleaseURL)
}

// getLatestVersion queries a GitHub releases/latest endpoint and returns the
// latest release URL and tag.
func getLatestVersion(ctx context.Context, endpoint string) (url string, tag string, err error) {
	if ctx == nil {
		return "", "", fmt.Errorf("nil context")
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	request = request.WithContext(ctx)

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", "", err
	}
	if response.StatusCode == http.StatusNotFound {
		_ = response.Body.Close()
		return "", "", fmt.Errorf("get latest release from %s: %w", endpoint, ErrNoReleases)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return "", "", fmt.Errorf("GitHub returned HTTP status %s", response.Status)
	}

	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil && err == nil {
			err = fmt.Errorf("close GitHub response: %w", closeErr)
		}
	}()

	var data struct {
		URL string `json:"html_url"`
		Tag string `json:"tag_name"`
	}

	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return "", "", err
	}

	return data.URL, data.Tag, nil
}

func normalizeSemVer(version string) (string, error) {
	original := version
	version, _ = strings.CutPrefix(version, "v")
	core := version
	if separator := strings.IndexAny(core, "-+"); separator >= 0 {
		core = core[:separator]
	}
	if len(strings.Split(core, ".")) != 3 {
		return "", fmt.Errorf("invalid semantic version %q", original)
	}

	normalized := "v" + version
	if !semver.IsValid(normalized) {
		return "", fmt.Errorf("invalid semantic version %q", original)
	}

	return normalized, nil
}

func compareSemVerVersions(current, latest string) (int, error) {
	normalizedCurrent, err := normalizeSemVer(current)
	if err != nil {
		return 0, err
	}

	normalizedLatest, err := normalizeSemVer(latest)
	if err != nil {
		return 0, err
	}

	return semver.Compare(normalizedCurrent, normalizedLatest), nil
}

type UpdateStatus int

const (
	Failed = UpdateStatus(iota)
	Ignored
	UpToDate
	UpdateAvailable
)

func CheckForUpdate() (UpdateStatus, string, error) {
	return CheckForUpdateContext(context.Background())
}

// CheckForUpdateContext performs the update check with caller-owned
// cancellation. This is used by the launcher so a closing window does not
// retain a network goroutine or post a dialog after teardown.
func CheckForUpdateContext(ctx context.Context) (UpdateStatus, string, error) {
	if ctx == nil {
		return Failed, "", fmt.Errorf("nil context")
	}
	if !build.IsRelease() {
		return Ignored, "", nil
	}
	if _, err := normalizeSemVer(build.Version); err != nil {
		return Failed, "", fmt.Errorf("invalid local release version: %w", err)
	}

	log.Println("Checking GitHub for a new version of danser-ee...")

	url, tag, err := getLatestVersion(ctx, githubLatestReleaseURL)
	if err != nil {
		return Failed, "", err
	}

	comparison, err := compareSemVerVersions(build.Version, tag)
	if err != nil {
		return Failed, "", err
	}

	if comparison >= 0 {
		return UpToDate, "", nil
	}

	return UpdateAvailable, url, nil
}
