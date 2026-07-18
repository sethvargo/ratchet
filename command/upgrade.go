package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sethvargo/ratchet/internal/concurrency"
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
	flagConcurrency int64
	flagParser      string
	flagOut         string
	flagPin         bool
	flagBakeDelay   time.Duration
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
	f.BoolVar(&c.flagPin, "pin", true, "pin resolved upgraded versions")

	f.DurationVar(&c.flagBakeDelay, "bake-delay", bakeDelayFromEnv(),
		"minimum age before ratchet resolves a commit, release, or image (0=off); also RATCHET_BAKE_DELAY")

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

	res, err := resolver.NewDefaultResolver(ctx, resolver.WithBakeDelay(c.flagBakeDelay))
	if err != nil {
		return fmt.Errorf("failed to create resolver: %w", err)
	}

	loadResult, err := loadYAMLFiles(os.DirFS("."), args)
	if err != nil {
		return err
	}

	if len(loadResult) > 1 && c.flagOut != "" && !strings.HasSuffix(c.flagOut, "/") {
		return fmt.Errorf("-out must be a directory when upgrading multiple files")
	}

	if err := parser.Unpin(ctx, loadResult.nodes()); err != nil {
		return fmt.Errorf("failed to unpin refs: %w", err)
	}

	if err := parser.Upgrade(ctx, res, par, loadResult.nodes(), c.flagConcurrency); err != nil {
		return fmt.Errorf("failed to upgrade refs: %w", err)
	}

	if c.flagPin {
		if err := parser.Pin(ctx, res, par, loadResult.nodes(), c.flagConcurrency); err != nil {
			return fmt.Errorf("failed to pin upgraded refs: %w", err)
		}
	}

	if err := loadResult.writeYAMLFiles(c.flagOut); err != nil {
		return fmt.Errorf("failed to save files: %w", err)
	}

	return nil
}
