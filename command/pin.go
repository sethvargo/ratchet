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

const pinCommandDesc = `Resolve and pin all versions`

const pinCommandHelp = `
Usage: ratchet pin [FILE...]

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

	if err := parser.Pin(ctx, res, par, files.nodes(), c.flagConcurrency); err != nil {
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

		updated, err := marshalYAML(f.node)
		if err != nil {
			return fmt.Errorf("failed to marshal yaml for %s: %w", f.path, err)
		}

		final := removeNewLineChanges(string(f.contents), string(updated))
		if err := atomic.Write(f.path, outFile, strings.NewReader(final)); err != nil {
			return fmt.Errorf("failed to save file %s: %w", outFile, err)
		}
	}

	return nil
}
