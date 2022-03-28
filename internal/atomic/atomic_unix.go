//go:build !windows

package atomic

import (
	"fmt"
	"os"
	"syscall"
)

// preserveOwnership attempts to preserve the file ownership for the file at
// the provided path.
func preserveOwnership(pth string, info os.FileInfo) error {
	sysInfo := info.Sys()
	if sysInfo != nil {
		stat, ok := sysInfo.(*syscall.Stat_t)
		if ok {
			if err := os.Chown(pth, int(stat.Uid), int(stat.Gid)); err != nil {
				return fmt.Errorf("failed to change ownership: %w", err)
			}
		}
	}

	if err := os.Chmod(pth, info.Mode()); err != nil {
		return fmt.Errorf("failed to update permissions: %w", err)
	}

	return nil
}
