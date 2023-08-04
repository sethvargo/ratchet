package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sethvargo/ratchet/internal/atomic"
	"github.com/sethvargo/ratchet/internal/concurrency"
	"github.com/sethvargo/ratchet/parser"
	"github.com/sethvargo/ratchet/resolver"
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
	flagConcurrency              int64
	flagParser                   string
	flagOut                      string
	flagExperimentalKeepNewlines bool
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
	f.BoolVar(&c.flagExperimentalKeepNewlines, "experimental-keep-newlines", keepNewlinesEnv(), "")

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

	inFile := args[0]

	uneditedContent, err := parseFile(inFile)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", inFile, err)
	}

	m, err := parseYAMLFile(inFile)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", inFile, err)
	}

	par, err := parser.For(ctx, c.flagParser)
	if err != nil {
		return err
	}

	res, err := resolver.NewDefaultResolver(ctx)
	if err != nil {
		return fmt.Errorf("failed to create github resolver: %w", err)
	}

	if err := parser.Pin(ctx, res, par, m, c.flagConcurrency); err != nil {
		return fmt.Errorf("failed to pin refs: %w", err)
	}

	outFile := c.flagOut
	if outFile == "" {
		outFile = inFile
	}

	if err := writeYAMLFile(inFile, outFile, m); err != nil {
		return fmt.Errorf("failed to save %s: %w", outFile, err)
	}

	if !c.flagExperimentalKeepNewlines {
		return nil
	}

	editedContent, err := parseFile(outFile)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", outFile, err)
	}

	final := removeNewLineChanges(uneditedContent, editedContent)
	if err := atomic.Write(inFile, outFile, strings.NewReader(final)); err != nil {
		return fmt.Errorf("failed to save file %s: %w", outFile, err)
	}

	return nil
}
