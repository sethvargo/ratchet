package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sethvargo/ratchet/internal/atomic"
	"github.com/sethvargo/ratchet/parser"
	"github.com/sethvargo/ratchet/resolver"
)

const upgradeCommandDesc = `Upgrade all pinned versions to the latest version`

const upgradeCommandHelp = `
Usage: ratchet upgrade [FILE...]

The "upgrade" command unpins any pinned versions, upgrades the unpinned version
constraint to the latest available value, and then re-pins the versions with the
new version constraint.

This command will upgrade pinned versions to versions beyond the constraint
(e.g. v2 -> v3).

EXAMPLES

    ratchet upgrade ./path/to/file.yaml

FLAGS

`

type UpgradeCommand struct {
	PinCommand
}

func (c *UpgradeCommand) Desc() string {
	return upgradeCommandDesc
}

func (c *UpgradeCommand) Flags() *flag.FlagSet {
	f := c.PinCommand.Flags()
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", strings.TrimSpace(upgradeCommandDesc))
		f.PrintDefaults()
	}

	return f
}

func (c *UpgradeCommand) Run(ctx context.Context, originalArgs []string) error {
	args, err := parseFlags(c.Flags(), originalArgs)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	par, err := parser.For(ctx, c.flagParser)
	if err != nil {
		return err
	}

	res, err := resolver.NewDefaultResolver(ctx)
	if err != nil {
		return fmt.Errorf("failed to create resolver: %w", err)
	}

	fsys := os.DirFS(".")

	files, err := loadYAMLFiles(fsys, args)
	if err != nil {
		return err
	}

	if len(files) > 1 && c.flagOut != "" && !strings.HasSuffix(c.flagOut, "/") {
		return fmt.Errorf("-out must be a directory when pinning multiple files")
	}

	if err := parser.Unpin(ctx, files.nodes()); err != nil {
		return fmt.Errorf("failed to pin refs: %w", err)
	}

	if err := parser.Pin(ctx, res, par, files.nodes(), c.flagConcurrency, true); err != nil {
		return fmt.Errorf("failed to pin refs: %w", err)
	}

	for _, f := range files {
		outFile := c.flagOut
		if strings.HasSuffix(c.flagOut, "/") {
			outFile = filepath.Join(c.flagOut, f.path)
		}
		if outFile == "" {
			outFile = f.path
		}

		upgraded, err := marshalYAML(f.node)
		if err != nil {
			return fmt.Errorf("failed to marshal yaml for %s: %w", f.path, err)
		}

		final := removeNewLineChanges(string(f.contents), string(upgraded))
		if err := atomic.Write(f.path, outFile, strings.NewReader(final)); err != nil {
			return fmt.Errorf("failed to save file %s: %w", outFile, err)
		}
	}

	return nil
}