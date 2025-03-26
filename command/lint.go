package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sethvargo/ratchet/formatter"
	"github.com/sethvargo/ratchet/parser"
)

const lintCommandDesc = `Lint and report unpinned versions`

const lintCommandHelp = `
Usage: ratchet lint [FILE...]

The "lint" command reports any unpinned versions, ignoring any versions with
the "ratchet:exclude" comment.

If any versions are unpinned, it returns a non-zero exit code. This command does
not communicate with upstream APIs or services.

EXAMPLES

  ratchet lint ./path/to/file.yaml

FLAGS

`

type LintCommand struct {
	flagFormat string
	flagParser string
}

func (c *LintCommand) Desc() string {
	return lintCommandDesc
}

func (c *LintCommand) Flags() *flag.FlagSet {
	f := flag.NewFlagSet("", flag.ExitOnError)
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", strings.TrimSpace(lintCommandHelp))
		f.PrintDefaults()
	}

	format := "human"
	if v := os.Getenv("GITHUB_ACTIONS"); v != "" {
		format = "actions"
	}

	f.StringVar(&c.flagFormat, "format", format, "linter output format")
	f.StringVar(&c.flagParser, "parser", "actions", "parser to use")

	return f
}

func (c *LintCommand) Run(ctx context.Context, originalArgs []string) error {
	args, err := parseFlags(c.Flags(), originalArgs)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	par, err := parser.For(ctx, c.flagParser)
	if err != nil {
		return err
	}

	loadResult, err := loadYAMLFiles(os.DirFS("."), args)
	if err != nil {
		return err
	}

	violations, err := parser.Lint(ctx, par, loadResult.nodes())
	if err != nil {
		return fmt.Errorf("failed to run linter: %w", err)
	}

	fmter, err := formatter.For(ctx, c.flagFormat)
	if err != nil {
		return err
	}

	if err := fmter.Format(os.Stdout, violations); err != nil {
		return err
	}

	if l := len(violations); l > 0 {
		return errors.New("") // empty error to force a non-zero exit code
	}

	return nil
}
