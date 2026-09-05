package build

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// ProgramName is the short brand used in logs and user-visible strings.
var ProgramName = "danser-ee"

// DanserExec names the gameplay executable resolved next to the launcher in
// packaged release builds.
var DanserExec = "danser-cli"

// Version is the user-visible version string shown in window titles, the
// launcher about panel, startup logs, and Discord presence.
//
//   - Development builds (plain go build or go run) report
//     "<branch>-<shortsha>" taken from the VCS stamp Go embeds at build
//     time, with a "-dirty" suffix when the working tree had uncommitted
//     changes. The branch resolves to the checkout the binary was built
//     from, whatever it is called: main, a feature branch, or anything
//     else. When the stamp is missing only the branch is shown, and when
//     the branch is unknown as well it falls back to the bare "dev".
//   - Release builds report the exact SemVer string the dist scripts inject
//     with -ldflags "-X .../build.Version=<version>".
//
// Display sites must use Version verbatim and never prepend a "v": dev
// versions do not start with a number, so a hardcoded prefix renders as
// "vdev-..." or "vmain-...".
var Version = "dev"

// Stream selects the build flavor. It stays "Dev" unless the dist scripts
// inject "Release" at packaging time. Callers must compare it through
// IsRelease or IsDev instead of matching the raw strings.
var Stream = "Dev"

// Branch names the source branch for development builds. It resolves in
// init from explicit build variables first and from the enclosing checkout
// second; release builds carry whatever the dist scripts inject, or an
// empty string when built without VCS access.
var Branch = ""

// CommitHash is the full VCS revision the binary was built from, or
// "Unknown" when the build carries no VCS stamp.
var CommitHash = "Unknown"

// IsRelease reports whether this is a packaged release build. Release builds
// load packed assets, resolve the gameplay executable beside the launcher,
// and are eligible for GitHub update checks.
func IsRelease() bool {
	return Stream == "Release"
}

// IsDev reports whether this is a development build. Development builds load
// assets from the working tree, reuse the running executable for gameplay,
// and never contact GitHub for update checks.
func IsDev() bool {
	return !IsRelease()
}

func init() {
	revision, modified := vcsInfo()
	if revision != "" {
		CommitHash = revision
	}

	if isDefaultDev(Version, Stream) {
		// The toolchain records the revision but never the branch, so
		// resolve it from explicit build variables or the checkout.
		Branch = resolveBranch(Branch, detectBranchStart())
	}

	// Keep Go's VCS metadata available for diagnostics while deriving the
	// user-visible version solely from explicit build variables and the
	// enclosing checkout.
	Version = formatVersion(Version, Stream, CommitHash, modified, Branch)
}

// isDefaultDev reports whether the version still selects the development
// display: the default version on the default stream. Anything else,
// including release versions injected at packaging time, passes through
// untouched.
func isDefaultDev(version, stream string) bool {
	return version == "dev" && stream == "Dev"
}

// vcsInfo returns the revision and modified flag from the binary's VCS stamp.
// It returns an empty revision when the build carries no stamp.
func vcsInfo() (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}

	var revision string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	return revision, modified
}

// resolveBranch picks the branch label for development builds. An explicitly
// injected branch wins, then the detected checkout branch, then the "dev"
// fallback which preserves the historical label when nothing is known.
func resolveBranch(explicit, detected string) string {
	if explicit != "" {
		return explicit
	}
	if detected != "" {
		return detected
	}
	return "dev"
}

// detectBranchStart detects the checkout branch from the working directory,
// returning "" when it cannot be determined. Detection is silent by design:
// development binaries run from source checkouts, while packaged binaries
// never reach this path.
func detectBranchStart() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return detectBranch(dir)
}

// detectBranch walks up from dir looking for a checkout and returns its
// branch, or "" when there is none. It reads .git/HEAD directly instead of
// shelling out to git so detection works without git on PATH. A .git file
// (worktrees, submodules) is followed through its gitdir pointer. Detached
// checkouts report "" since no branch name exists.
func detectBranch(dir string) string {
	current, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	for {
		head, found := readHEAD(current)
		if found {
			return parseBranch(head)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// readHEAD returns the raw .git/HEAD content for the checkout rooted at dir.
// The second result is false when dir holds no checkout.
func readHEAD(dir string) (string, bool) {
	dotGit := filepath.Join(dir, ".git")

	info, err := os.Stat(dotGit)
	if err != nil {
		return "", false
	}

	gitDir := dotGit
	if !info.IsDir() {
		// Worktrees and submodules point at the real git directory.
		pointer, err := os.ReadFile(dotGit)
		if err != nil {
			return "", false
		}
		target, found := strings.CutPrefix(strings.TrimSpace(string(pointer)), "gitdir: ")
		if !found {
			return "", false
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
		gitDir = target
	}

	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", false
	}

	return strings.TrimSpace(string(head)), true
}

// parseBranch extracts the branch from raw HEAD content. Only refs/heads
// entries name a branch; detached revisions and anything unexpected yield "".
func parseBranch(head string) string {
	branch, found := strings.CutPrefix(head, "ref: refs/heads/")
	if !found || branch == "" {
		return ""
	}
	return branch
}

// formatVersion derives the development display from the branch, revision,
// and dirty flag. Explicit versions, including release versions injected at
// packaging time, pass through untouched.
func formatVersion(version, stream, commit string, dirty bool, branch string) string {
	if !isDefaultDev(version, stream) {
		return version
	}

	if branch == "" {
		branch = "dev"
	}

	short := shortHash(commit)
	if short == "" {
		return branch
	}

	if dirty {
		return branch + "-" + short + "-dirty"
	}

	return branch + "-" + short
}

// shortHash reduces a full revision to the 7-character form used in
// user-visible strings. It returns an empty string for missing or
// placeholder revisions so callers can fall back to the branch alone.
func shortHash(commit string) string {
	if commit == "" || commit == "Unknown" {
		return ""
	}

	return commit[:min(7, len(commit))]
}
