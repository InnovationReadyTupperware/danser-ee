package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/innovationreadytupperware/danser-ee/build"
	"golang.org/x/mod/semver"
)

// GetLatestVersionFromGitHub makes a request to GitHub and returns url and tag of the latest version found
func GetLatestVersionFromGitHub() (url string, tag string, err error) {
	return GetLatestVersionFromGitHubContext(context.Background())
}

// GetLatestVersionFromGitHubContext makes a cancellable request to GitHub and
// returns the latest release URL and tag. The timeout protects launcher
// shutdown from a network stack that never completes a request.
func GetLatestVersionFromGitHubContext(ctx context.Context) (url string, tag string, err error) {
	if ctx == nil {
		return "", "", fmt.Errorf("nil context")
	}
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/innovationreadytupperware/danser-ee/releases/latest", nil)
	if err != nil {
		return "", "", err
	}
	request = request.WithContext(ctx)

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", "", err
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

// TransformVersion transfers legacy danser version strings to a comparable format.
//   - 0.6.7 becomes 600079999
//   - 0.6.7-s(napshot)12 becomes 600070012
//   - 1.0.0 becomes 1000000009999
//
// Deprecated: release update checks use SemVer 2.0.0 comparison instead. This
// function remains for compatibility with callers of the existing API.
func TransformVersion(version string) uint64 {
	currentSplit := strings.Split(version, "-")
	splitDots := strings.Split(strings.TrimSuffix(currentSplit[0], "b"), ".")

	for i, s := range splitDots {
		splitDots[i] = fmt.Sprintf("%04s", s)
	}

	snapshot := "9999"
	if len(currentSplit) > 1 && !strings.HasPrefix(currentSplit[1], "dev") {
		snapshot = fmt.Sprintf("%04s", strings.TrimPrefix(strings.TrimPrefix(currentSplit[1], "s"), "napshot"))
	}

	versionInt, err := strconv.ParseUint(strings.Join(splitDots, "")+snapshot, 10, 64)
	if err != nil {
		panic(err)
	}

	return versionInt
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
	Snapshot
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
	if build.Stream != "Release" {
		return Ignored, "", nil
	}
	if _, err := normalizeSemVer(build.VERSION); err != nil {
		return Failed, "", fmt.Errorf("invalid local release version: %w", err)
	}

	log.Println("Checking GitHub for a new version of danser-ee...")

	url, tag, err := GetLatestVersionFromGitHubContext(ctx)
	if err != nil {
		return Failed, "", err
	}

	comparison, err := compareSemVerVersions(build.VERSION, tag)
	if err != nil {
		return Failed, "", err
	}

	if comparison >= 0 {
		return UpToDate, "", nil
	}

	return UpdateAvailable, url, nil
}
