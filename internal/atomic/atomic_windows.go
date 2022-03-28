//go:build windows

package atomic

import (
	"os"
)

// preserveOwnership attempts to preserve the file ownership for the file at
// the provided path.
func preserveOwnership(pth string, info os.FileInfo) error {
	if err := os.Chmod(pth, info.Mode()); err != nil {
		return fmt.Errorf("failed to update permissions: %w", err)
	}
	return nil
}
