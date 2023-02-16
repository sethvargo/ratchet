package walker

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/sethvargo/ratchet/parser"
	"gopkg.in/yaml.v3"
)

func TestWalk(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		wantErr bool
	}{
		{
			"happy path",
			"../../.github/workflows",
			false,
		},
		{
			"non-existent path",
			"foo.yml",
			true,
		},
		{
			"contains non-y(aml|ml) file",
			"../../docs",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Walk(context.Background(), tt.dir, doer); (err != nil) != tt.wantErr {
				t.Errorf("Walk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// doer is similar to the check function in command/check.go
func doer(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	par, err := parser.For(ctx, "actions")
	if err != nil {
		return err
	}

	var m yaml.Node
	if err := yaml.NewDecoder(f).Decode(&m); err != nil {
		return fmt.Errorf("failed to decode yaml: %w", err)
	}

	return parser.Check(ctx, par, &m)
}
