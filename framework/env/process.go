package env

import (
	"os"
	"strings"
)

// LauncherChildEnvVar is set on gameplay processes started by the graphical
// launcher. The child must not show a second fatal dialog because the launcher
// owns that process boundary and adds the exit status to its user-facing error.
const LauncherChildEnvVar = "DANSER_LAUNCHER_CHILD"

// IsLauncherChild reports whether the current process was started and is being
// monitored by the graphical launcher.
func IsLauncherChild() bool {
	return os.Getenv(LauncherChildEnvVar) == "1"
}

// LauncherChildEnvironment returns a copy of the current environment with a
// single, unambiguous launcher-child marker. Removing existing copies matters
// on Windows, where duplicate environment keys can otherwise make the child
// inherit an implementation-dependent value.
func LauncherChildEnvironment(launcherChild bool) []string {
	keyPrefix := LauncherChildEnvVar + "="
	environment := os.Environ()
	filtered := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		key, _, found := strings.Cut(entry, "=")
		if found && strings.EqualFold(key, LauncherChildEnvVar) {
			continue
		}
		filtered = append(filtered, entry)
	}

	if launcherChild {
		filtered = append(filtered, keyPrefix+"1")
	}

	return filtered
}
