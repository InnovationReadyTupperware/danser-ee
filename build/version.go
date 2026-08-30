package build

import (
	"runtime/debug"
)

var ProgramName = "danser-ee"

var CommitHash = "Unknown"

var VERSION = "dev"

var Stream = "Dev"

var DanserExec = "danser-cli"

func init() {
	if bI, ok := debug.ReadBuildInfo(); ok {
		for _, k := range bI.Settings {
			if k.Key == "vcs.revision" {
				CommitHash = k.Value

				break
			}
		}
	}

	// Keep Go's VCS metadata available for diagnostics while deriving the
	// user-visible version solely from explicit build variables.
	VERSION = formatVersion(VERSION, Stream, CommitHash)
}

// formatVersion appends a short revision only to the default development version.
func formatVersion(version, stream, commit string) string {
	if version != "dev" || stream != "Dev" {
		return version
	}

	return version + "-" + commit[:min(7, len(commit))]
}
