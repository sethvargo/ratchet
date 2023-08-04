package command

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sethvargo/ratchet/internal/atomic"
	"github.com/sethvargo/ratchet/parser"
)

const unpinCommandDesc = `Revert pinned versions to their unpinned values`

const unpinCommandHelp = `
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

	if err := parser.Unpin(m); err != nil {
		return fmt.Errorf("failed to upin refs: %w", err)
	}

	outFile := c.flagOut
	if outFile == "" {
		outFile = inFile
	}
	if err := writeYAMLFile(inFile, outFile, m); err != nil {
		return fmt.Errorf("failed to save %s: %w", outFile, err)
	}

	if !keepNewlinesEnv() {
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
