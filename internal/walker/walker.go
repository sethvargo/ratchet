package walker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Doer is a generic function to be called for each file. It plumbs in
// through lower in the stack and make it available in all the underlying
// commands.
type Doer func(ctx context.Context, path string) error

// Walk walks the file tree rooted at root, calling doer() for each file.
func Walk(ctx context.Context, dir string, doerFn Doer) error {
	var failures []string
	if err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Only check .yaml and .yml files
			if !strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml") {
				return nil
			}

			if err := doerFn(ctx, path); err != nil {
				failures = append(failures, path)
			}

			return nil
		}); err != nil {
		return err
	}
	if len(failures) > 0 {
		for _, f := range failures {
			fmt.Printf("fail: %s\n", f)
		}
		return fmt.Errorf("command failed for %d files", len(failures))
	}
	return nil
}
