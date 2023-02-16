//go:generate go run ./cmd/gen/main.go
package command

import (
	"context"
	"fmt"
	"os"

	"github.com/sethvargo/ratchet/parser"
	"github.com/sethvargo/ratchet/resolver"

	"github.com/sethvargo/ratchet/internal/walker"

	"github.com/sethvargo/ratchet/internal/version"
)

// Commands is the main list of all commands.
var Commands = map[string]Command{
	"check":  &CheckCommand{},
	"pin":    &PinCommand{},
	"unpin":  &UnpinCommand{},
	"update": &UpdateCommand{},
}

// Command is the interface for a subcommand.
type Command interface {
	Desc() string
	Run(ctx context.Context, args []string) error
	Do(ctx context.Context, path string, par parser.Parser, res resolver.Resolver) error
}

// Doer is a type that implements Command.Do() function.
type Doer func(ctx context.Context, path string, par parser.Parser, res resolver.Resolver) error

// Run executes the main entrypoint for the CLI.
func Run(ctx context.Context, args []string) error {
	name, args := extractCommandAndArgs(args)

	// Short-circuit top-level help.
	if name == "" || name == "-h" || name == "-help" || name == "--help" {
		fmt.Fprint(os.Stderr, topLevelHelp)
		return nil
	}

	if name == "-v" || name == "-version" || name == "--version" {
		fmt.Fprintln(os.Stderr, version.HumanVersion)
		return nil
	}

	cmd, ok := Commands[name]
	if !ok {
		return fmt.Errorf("unknown command %q", name)
	}

	return cmd.Run(ctx, args)
}

// extractCommandAndArgs is a helper that pulls the subcommand and arguments.
func extractCommandAndArgs(args []string) (string, []string) {
	switch len(args) {
	case 0:
		return "", nil
	case 1:
		return args[0], nil
	default:
		return args[0], args[1:]
	}
}

// do calls Run() command as-is, if given path is a file. If the path
// is a directory, it will walk the directory and issues Run() on each file.
func do(ctx context.Context, path string, doerFn Doer, flagParser string, flagConcurrency int64) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("input not found %w", err)
	}

	// Initialize parser and resolver once per command. To avoid
	// creating unnecessary allocations while we are walking
	// in a directory.
	par, res, err := getParserAndResolver(ctx, flagParser)
	if err != nil {
		return err
	}

	// doer is a wrapper around the actual walker.Doer() since
	// that function takes only a path and a parser. But we need
	// to pass the par and res args to actual command itself.
	doer := func(ctx context.Context, path string) error {
		return doerFn(ctx, path, par, res)
	}

	if fileInfo.IsDir() {
		// If this function is called from either `pin` and `update` commands,
		// we should run the caching logic.
		if flagParser != "" {
			nodes, err := walker.Walk(ctx, path, walker.NoOp)
			if err != nil {
				return fmt.Errorf("walk %s: %w", path, err)
			}

			if err := parser.FetchAndCacheReferences(ctx, res, par, nodes, flagConcurrency, false); err != nil {
				return fmt.Errorf("cache references: %w", err)
			}
		}

		// Now we are ready to run the command.
		_, err := walker.Walk(ctx, path, doer)
		return err
	}

	// If the path is a file, keep the as-is behavior.
	return doer(ctx, path)
}

func getParserAndResolver(ctx context.Context, flagParser string) (parser.Parser, resolver.Resolver, error) {
	par, err := parser.For(ctx, flagParser)
	if err != nil {
		return nil, nil, err
	}

	res, err := resolver.NewDefaultResolver(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create github resolver: %w", err)
	}

	return par, res, nil
}
