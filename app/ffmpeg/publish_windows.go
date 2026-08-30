//go:build windows

package ffmpeg

import (
	"errors"
	"fmt"
	"os"
)

func publishRecording(stagedPath, finalPath string) error {
	if _, err := os.Lstat(finalPath); err == nil {
		return fmt.Errorf("refusing to overwrite existing output %q", finalPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect final output: %w", err)
	}

	// On Windows, os.Rename does not replace an existing destination. The
	// session directory is created under the output root, so this remains an
	// atomic same-volume publication.
	if err := os.Rename(stagedPath, finalPath); err != nil {
		return fmt.Errorf("publish %q: %w", finalPath, err)
	}

	return nil
}
