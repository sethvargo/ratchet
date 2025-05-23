package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sethvargo/ratchet/parser"
)

const unpinCommandDesc = `Revert pinned versions to their unpinned values`

const unpinCommandHelp = `
Usage: ratchet unpin [FILE...]

The "unpin" command reverts any pinned versions to their non-absolute or
relative version for the given input file:

  actions/checkout@2541b1294d2704b0964813337f... -> actions/checkout@v3

This happens by replacing the value in the Ratchet comment back into the file.
This command does not communicate with upstream APIs or services. If there is no
comment, no action is taken.

To update versions that are already pinned, use the "update" command instead.

EXAMPLES

    ratchet unpin ./path/to/file.yaml

FLAGS

`

type UnpinCommand struct {
	flagOut string
}

func (c *UnpinCommand) Desc() string {
	return unpinCommandDesc
}

func (c *UnpinCommand) Flags() *flag.FlagSet {
	f := flag.NewFlagSet("", flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", strings.TrimSpace(unpinCommandHelp))
		f.PrintDefaults()
	}

	f.StringVar(&c.flagOut, "out", "", "output path (defaults to input file)")

	return f
}

func (c *UnpinCommand) Run(ctx context.Context, originalArgs []string) error {
	args, err := parseFlags(c.Flags(), originalArgs)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	loadResult, err := loadYAMLFiles(os.DirFS("."), args)
	if err != nil {
		return err
	}

	if len(loadResult) > 1 && c.flagOut != "" && !strings.HasSuffix(c.flagOut, "/") {
		return fmt.Errorf("-out must be a directory when pinning multiple files")
	}

	if err := parser.Unpin(ctx, loadResult.nodes()); err != nil {
		return fmt.Errorf("failed to pin refs: %w", err)
	}

	if err := loadResult.writeYAMLFiles(c.flagOut); err != nil {
		return fmt.Errorf("failed to save files: %w", err)
	}

	return nil
}
