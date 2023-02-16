package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sethvargo/ratchet/internal/yaml"
	"github.com/sethvargo/ratchet/resolver"

	"github.com/sethvargo/ratchet/internal/concurrency"
	"github.com/sethvargo/ratchet/parser"
)

const pinCommandDesc = `Resolve and pin all versions`

const pinCommandHelp = `
The "pin" command resolves and pins any unpinned versions to their absolute or
hashed version for the given input file:

    actions/checkout@v3 -> actions/checkout@2541b1294d2704b0964813337f...

The original unpinned version is preserved in a comment, next to the pinned
version. If a version is already pinned, it does nothing.

To update versions that are already pinned, use the "update" command instead.

EXAMPLES

  ratchet pin ./path/to/file.yaml

FLAGS

`

type PinCommand struct {
	flagConcurrency int64
	flagParser      string
	flagOut         string
}

func (c *PinCommand) Desc() string {
	return pinCommandDesc
}

func (c *PinCommand) Flags() *flag.FlagSet {
	f := flag.NewFlagSet("", flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", strings.TrimSpace(pinCommandHelp))
		f.PrintDefaults()
	}

	f.Int64Var(&c.flagConcurrency, "concurrency", concurrency.DefaultConcurrency(1),
		"maximum number of concurrent resolutions")
	f.StringVar(&c.flagParser, "parser", "actions", "parser to use")
	f.StringVar(&c.flagOut, "out", "", "output path (defaults to input file)")

	return f
}

func (c *PinCommand) Run(ctx context.Context, originalArgs []string) error {
	f := c.Flags()

	if err := f.Parse(originalArgs); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	args := f.Args()
	if got := len(args); got != 1 {
		return fmt.Errorf("expected exactly one argument, got %d", got)
	}

	return do(ctx, args[0], c.Do, c.flagParser, c.flagConcurrency)
}

func (c *PinCommand) Do(ctx context.Context, path string, par parser.Parser, res resolver.Resolver) error {
	m, err := yaml.ParseFile(path)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
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
