//go:build !windows

package ffmpeg

import (
	"fmt"
	"os"
)

func publishRecording(stagedPath, finalPath string) error {
	// Linking creates the destination atomically and fails if it already
	// exists. Both paths live under the output root and therefore share a
	// filesystem. Removing the staged link completes the no-replace move.
	if err := os.Link(stagedPath, finalPath); err != nil {
		return fmt.Errorf("publish %q without replacement: %w", finalPath, err)
	}
	if err := os.Remove(stagedPath); err != nil {
		return fmt.Errorf("remove staged link after publishing %q: %w", finalPath, err)
	}

	return nil
}
