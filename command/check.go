package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sethvargo/ratchet/parser"
)

const checkCommandDesc = `Check if all versions are pinned`

const checkCommandHelp = `
Usage: ratchet check [FILE...]

The "check" command checks if all versions are pinned to an absolute version,
ignoring any versions with the "ratchet:exclude" comment.

If any versions are unpinned, it returns a non-zero exit code. This command does
not communicate with upstream APIs or services.

EXAMPLES

  ratchet check ./path/to/file.yaml

FLAGS

`

type CheckCommand struct {
	flagParser string
}

func (c *CheckCommand) Desc() string {
	return checkCommandDesc
}

func (c *CheckCommand) Flags() *flag.FlagSet {
	f := flag.NewFlagSet("", flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", strings.TrimSpace(checkCommandHelp))
		f.PrintDefaults()
	}

	f.StringVar(&c.flagParser, "parser", "actions", "parser to use")

	return f
}

func (c *CheckCommand) Run(ctx context.Context, originalArgs []string) error {
	args, err := parseFlags(c.Flags(), originalArgs)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	par, err := parser.For(ctx, c.flagParser)
	if err != nil {
		return err
	}

	fsys := os.DirFS(".")

	files, err := loadYAMLFiles(fsys, args)
	if err != nil {
		return err
	}

	return parser.Check(ctx, par, files.nodes())
}
