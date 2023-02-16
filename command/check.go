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

const checkCommandDesc = `Check if all versions are pinned`

const checkCommandHelp = `
The "check" command checks if all versions are pinned to an absolute version,
ignoring any versions with the "ratchet:exclude" comment.

If any versions are unpinned, it returns a non-zero exit code. This command does
not communicate with upstream APIs or services.

EXAMPLES

  ratchet check ./path/to/file.yaml
  ratchet check ./path/to/dir

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
	f := c.Flags()

	if err := f.Parse(originalArgs); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	args := f.Args()
	if got := len(args); got != 1 {
		return fmt.Errorf("expected exactly one argument, got %d", got)
	}

	return do(ctx, args[0], c.Do, c.flagParser, 0)
}

func (c *CheckCommand) Do(ctx context.Context, path string, par parser.Parser, _ resolver.Resolver) error {
	m, err := yaml.ParseFile(path)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if err := parser.Check(ctx, par, m); err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	return nil
}
