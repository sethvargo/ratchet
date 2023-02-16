package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sethvargo/ratchet/internal/yaml"
	"github.com/sethvargo/ratchet/resolver"

	"github.com/sethvargo/ratchet/parser"
)

const updateCommandDesc = `Update all pinned versions to the latest value`

const updateCommandHelp = `
The "update" command unpins any pinned versions, resolves the unpinned version
constraint to the latest available value, and then re-pins the versions.

This command will pin to the latest available version that satifies the original
constraint. To upgrade to versions beyond the contraint (e.g. v2 -> v3), you
must manually edit the file and update the unpinned comment.

EXAMPLES

    ratchet update ./path/to/file.yaml

FLAGS

`

type UpdateCommand struct {
	PinCommand
}

func (c *UpdateCommand) Desc() string {
	return updateCommandDesc
}

func (c *UpdateCommand) Flags() *flag.FlagSet {
	f := c.PinCommand.Flags()
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", strings.TrimSpace(updateCommandHelp))
		f.PrintDefaults()
	}

	return f
}

func (c *UpdateCommand) Run(ctx context.Context, originalArgs []string) error {
	f := c.Flags()

	if err := f.Parse(originalArgs); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	args := f.Args()
	if got := len(args); got != 1 {
		return fmt.Errorf("expected exactly one argument, got %d %q", got, args)
	}

	return do(ctx, args[0], c.Do, c.flagParser, c.flagConcurrency)
}

func (c *UpdateCommand) Do(ctx context.Context, path string, par parser.Parser, res resolver.Resolver) error {
	m, err := yaml.ParseFile(path)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if err := parser.Unpin(m); err != nil {
		return fmt.Errorf("failed to unpin refs: %w", err)
	}

	if err := parser.Pin(ctx, res, par, m, c.flagConcurrency); err != nil {
		return fmt.Errorf("failed to pin refs: %w", err)
	}

	outFile := c.flagOut
	if outFile == "" {
		outFile = path
	}
	if err := yaml.WriteFile(path, outFile, m); err != nil {
		return fmt.Errorf("failed to save %s: %w", outFile, err)
	}

	return nil
}
