package walker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sethvargo/ratchet/internal/yaml"
	yamlv3 "gopkg.in/yaml.v3"
)

// Doer is a generic function to be called for each file. It plumbs in
// through lower in the stack and make it available in all the underlying
// commands.
type Doer func(ctx context.Context, path string) error

// NoOp is a no-op doer.
var NoOp = func(ctx context.Context, path string) error {
	return nil
}

// Walk walks the file tree rooted at root, calling doer() for each file. And
// aggregate the all files into a Node slice.
func Walk(ctx context.Context, dir string, doerFn Doer) ([]*yamlv3.Node, error) {
	var failures []string
	var nodes []*yamlv3.Node
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

			m, err := yaml.ParseFile(path)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", path, err)
			}

			nodes = append(nodes, m)

			return nil
		}); err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		for _, f := range failures {
			fmt.Printf("fail: %s\n", f)
		}
		return nil, fmt.Errorf("command failed for %d files", len(failures))
	}
	return nodes, nil
}
