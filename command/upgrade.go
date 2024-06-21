package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sethvargo/ratchet/internal/atomic"
	"github.com/sethvargo/ratchet/internal/concurrency"
	"github.com/sethvargo/ratchet/parser"
	"github.com/sethvargo/ratchet/resolver"
)

const upgradeCommandDesc = `Upgrade all pinned versions to the latest version`

const upgradeCommandHelp = `
Usage: ratchet f [FILE...]

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
	flagConcurrency int64
	flagParser      string
	flagOut         string
	flagPin         bool
}

func (c *UpgradeCommand) Desc() string {
	return upgradeCommandDesc
}

func (c *UpgradeCommand) Flags() *flag.FlagSet {
	f := flag.NewFlagSet("", flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", strings.TrimSpace(upgradeCommandHelp))
		f.PrintDefaults()
	}

	f.Int64Var(&c.flagConcurrency, "concurrency", concurrency.DefaultConcurrency(1),
		"maximum number of concurrent resolutions")
	f.StringVar(&c.flagParser, "parser", "actions", "parser to use")
	f.StringVar(&c.flagOut, "out", "", "output path (defaults to input file)")
	f.BoolVar(&c.flagPin, "pin", true, "pin resolved upgraded versions (defaults to true)")

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
		return fmt.Errorf("-out must be a directory when upgrading multiple files")
	}

	if err := parser.Unpin(ctx, files.nodes()); err != nil {
		return fmt.Errorf("failed to unpin refs: %w", err)
	}

	if err := parser.Upgrade(ctx, res, par, files.nodes(), c.flagConcurrency); err != nil {
		return fmt.Errorf("failed to upgrade refs: %w", err)
	}

	if c.flagPin {
		if err := parser.Pin(ctx, res, par, files.nodes(), c.flagConcurrency); err != nil {
			return fmt.Errorf("failed to pin upgraded refs: %w", err)
		}
	}

	for _, f := range files {
		outFile := c.flagOut
		if strings.HasSuffix(c.flagOut, "/") {
			outFile = filepath.Join(c.flagOut, f.path)
		}
		if outFile == "" {
			outFile = f.path
		}

		final, err := f.marshalYAML()
		if err != nil {
			return fmt.Errorf("failed to marshal yaml for %s: %w", f.path, err)
		}

		if err := atomic.Write(f.path, outFile, strings.NewReader(final)); err != nil {
			return fmt.Errorf("failed to save file %s: %w", outFile, err)
		}
	}

	return nil
}
