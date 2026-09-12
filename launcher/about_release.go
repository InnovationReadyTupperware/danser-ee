//go:build danser_release

package launcher

import "github.com/innovationreadytupperware/danser-ee/build"

func aboutDeveloperDialogOptions(*launcher) []popupDialogOption {
	return nil
}

func aboutDisplayedBuildIdentity() (version string, releaseStyle bool) {
	if build.IsRelease() {
		return build.Version, true
	}
	return build.Version, false
}

func aboutAutomaticUpdateChecksEnabled() bool {
	return launcherConfig.CheckForUpdates
}
