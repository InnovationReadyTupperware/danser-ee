package utils

import (
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/innovationreadytupperware/danser-ee/build"
	"golang.org/x/mod/semver"
)

// githubLatestReleaseURL is the endpoint consulted for packaged release
// builds. It is a variable so tests can point the check at a local server.
var githubLatestReleaseURL = "https://api.github.com/repos/innovationreadytupperware/danser-ee/releases/latest"

// githubReleasesURL is the endpoint used by the update checker when it needs
// to evaluate stable and pre-release versions according to SemVer policy.
var githubReleasesURL = "https://api.github.com/repos/innovationreadytupperware/danser-ee/releases"

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

func semVerCore(version string) (string, error) {
	normalized, err := normalizeSemVer(version)
	if err != nil {
		return "", err
	}

	core, _, _ := strings.Cut(normalized, "+")
	core, _, _ = strings.Cut(core, "-")
	return core, nil
}

type githubRelease struct {
	URL        string `json:"html_url"`
	Tag        string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

const githubReleasePageSize = 100

func getPublishedReleases(ctx context.Context, endpoint string) ([]githubRelease, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil context")
	}
	baseURL, err := neturl.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}

	var releases []githubRelease
	for page := 1; ; page++ {
		requestURL := baseURL.Clone()
		query := requestURL.Query()
		query.Set("per_page", strconv.Itoa(githubReleasePageSize))
		query.Set("page", strconv.Itoa(page))
		requestURL.RawQuery = query.Encode()

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return nil, err
		}

		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}

		if response.StatusCode == http.StatusNotFound {
			_ = response.Body.Close()
			if page == 1 {
				return nil, fmt.Errorf("get releases from %s: %w", endpoint, ErrNoReleases)
			}
			break
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return nil, fmt.Errorf("GitHub returned HTTP status %s", response.Status)
		}

		var pageReleases []githubRelease
		decodeErr := jsonv2.UnmarshalRead(response.Body, &pageReleases)
		closeErr := response.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close GitHub response: %w", closeErr)
		}

		for _, release := range pageReleases {
			if !release.Draft {
				releases = append(releases, release)
			}
		}
		if len(pageReleases) < githubReleasePageSize {
			break
		}
	}

	if len(releases) == 0 {
		return nil, ErrNoReleases
	}
	return releases, nil
}

func selectLatestEligibleRelease(currentVersion string, releases []githubRelease, includePrerelease bool) (githubRelease, string, bool, error) {
	current, err := normalizeSemVer(currentVersion)
	if err != nil {
		return githubRelease{}, "", false, err
	}
	currentCore, err := semVerCore(currentVersion)
	if err != nil {
		return githubRelease{}, "", false, err
	}
	currentPrerelease := semver.Prerelease(current) != ""

	var selected githubRelease
	selectedVersion := ""
	found := false
	validReleaseFound := false
	for _, release := range releases {
		version, normalizeErr := normalizeSemVer(release.Tag)
		if normalizeErr != nil {
			continue
		}
		validReleaseFound = true

		candidatePrerelease := release.Prerelease || semver.Prerelease(version) != ""
		if candidatePrerelease && !includePrerelease {
			if !currentPrerelease {
				continue
			}
			candidateCore, coreErr := semVerCore(release.Tag)
			if coreErr != nil || candidateCore != currentCore {
				continue
			}
		}

		if !found || semver.Compare(version, selectedVersion) > 0 {
			selected = release
			selectedVersion = version
			found = true
		}
	}

	if !validReleaseFound {
		return githubRelease{}, "", false, fmt.Errorf("no published release has a valid semantic version")
	}
	return selected, selectedVersion, found, nil
}

type UpdateStatus int

const (
	Failed = UpdateStatus(iota)
	Ignored
	UpToDate
	UpdateAvailable
)

// UpdateResult is the complete result of comparing this build with the latest
// eligible published GitHub release. LatestVersion is populated for both
// up-to-date and update-available results so higher-level update services can
// cache and present the release without repeating the network request.
type UpdateResult struct {
	Status        UpdateStatus
	ReleaseURL    string
	LatestVersion string
}

// UpdateCheckOptions controls which published releases are considered by an
// update check. Stable releases are always eligible. When IncludePrerelease is
// false, a packaged pre-release build may still advance within its current
// base version until that release becomes stable.
type UpdateCheckOptions struct {
	IncludePrerelease bool
}

func CheckForUpdate() (UpdateStatus, string, error) {
	return CheckForUpdateContext(context.Background())
}

// CheckForUpdateContext performs the update check with caller-owned
// cancellation. The shared update service uses this to stop network work when
// its caller is shutting down.
func CheckForUpdateContext(ctx context.Context) (UpdateStatus, string, error) {
	result, err := CheckForUpdateDetailsContext(ctx)
	return result.Status, result.ReleaseURL, err
}

// CheckForUpdateDetailsContext performs the update check and returns the latest
// eligible published version together with the legacy status and release URL.
func CheckForUpdateDetailsContext(ctx context.Context) (UpdateResult, error) {
	return CheckForUpdateDetailsWithOptionsContext(ctx, UpdateCheckOptions{})
}

// CheckForUpdateDetailsWithOptionsContext performs the update check using the
// supplied release-channel policy.
func CheckForUpdateDetailsWithOptionsContext(ctx context.Context, options UpdateCheckOptions) (UpdateResult, error) {
	if ctx == nil {
		return UpdateResult{Status: Failed}, fmt.Errorf("nil context")
	}
	if !build.IsRelease() {
		return UpdateResult{Status: Ignored}, nil
	}
	if _, err := normalizeSemVer(build.Version); err != nil {
		return UpdateResult{Status: Failed}, fmt.Errorf("invalid local release version: %w", err)
	}

	releases, err := getPublishedReleases(ctx, githubReleasesURL)
	if err != nil {
		return UpdateResult{Status: Failed}, err
	}

	release, normalizedLatest, found, err := selectLatestEligibleRelease(build.Version, releases, options.IncludePrerelease)
	if err != nil {
		return UpdateResult{Status: Failed}, err
	}
	if !found {
		return UpdateResult{
			Status:        UpToDate,
			LatestVersion: build.Version,
		}, nil
	}

	comparison, err := compareSemVerVersions(build.Version, normalizedLatest)
	if err != nil {
		return UpdateResult{Status: Failed, LatestVersion: release.Tag}, err
	}
	if comparison >= 0 {
		return UpdateResult{
			Status:        UpToDate,
			LatestVersion: release.Tag,
		}, nil
	}

	return UpdateResult{
		Status:        UpdateAvailable,
		ReleaseURL:    release.URL,
		LatestVersion: release.Tag,
	}, nil
}
