//go:generate go run ./cmd/gen/main.go
package command

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/internal/version"
)

// Commands is the main list of all commands.
var Commands = map[string]Command{
	"check":   &CheckCommand{},
	"pin":     &PinCommand{},
	"unpin":   &UnpinCommand{},
	"update":  &UpdateCommand{},
	"upgrade": &UpgradeCommand{},
}

// Command is the interface for a subcommand.
type Command interface {
	Desc() string
	Run(ctx context.Context, args []string) error
}

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

// parseFlags is a helper that parses flags. Unlike [flags.Parse], it handles
// flags that occur after or between positional arguments.
func parseFlags(f *flag.FlagSet, args []string) ([]string, error) {
	var finalArgs []string
	var merr error

	merr = errors.Join(merr, f.Parse(args))

	for i := len(args) - len(f.Args()) + 1; i < len(args); {
		// Stop parsing if we hit an actual "stop parsing"
		if i > 1 && args[i-2] == "--" {
			break
		}
		finalArgs = append(finalArgs, f.Arg(0))
		merr = errors.Join(merr, f.Parse(args[i:]))
		i += 1 + len(args[i:]) - len(f.Args())
	}
	finalArgs = append(finalArgs, f.Args()...)

	return finalArgs, merr
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

// marshalYAML encodes the yaml node into the given writer.
func marshalYAML(m *yaml.Node) (string, error) {
	var b bytes.Buffer

	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	enc.SetAssumeBlockAsLiteral(true)
	if err := enc.Encode(m); err != nil {
		return "", fmt.Errorf("failed to encode yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize yaml: %w", err)
	}
	return b.String(), nil
}

type loadResult struct {
	path     string
	node     *yaml.Node
	contents string
	newlines []int
}

func (r *loadResult) marshalYAML() (string, error) {
	// Process the node tree to ensure multiline strings use LiteralStyle
	processMultilineNodes(r.node)

	contents, err := marshalYAML(r.node)
	if err != nil {
		return "", err
	}

	// Restore newlines
	lines := strings.Split(contents, "\n")

	for _, v := range r.newlines {
		lines = slices.Insert(lines, v, "")
	}

	// Handle the edge case where a document starts with "---", which the Go YAML
	// parser discards.
	if strings.HasPrefix(strings.TrimSpace(r.contents), "---") && !strings.HasPrefix(contents, "---") {
		lines = slices.Insert(lines, 0, "---")
	}

	return strings.Join(lines, "\n"), nil
}

type loadResults []*loadResult

func (r loadResults) nodes() []*yaml.Node {
	n := make([]*yaml.Node, 0, len(r))
	for _, v := range r {
		n = append(n, v.node)
	}
	return n
}

func loadYAMLFiles(fsys fs.FS, paths []string) (loadResults, error) {
	r := make(loadResults, 0, len(paths))

	for _, pth := range paths {
		pth = strings.TrimPrefix(pth, "./")
		contents, err := fs.ReadFile(fsys, pth)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", pth, err)
		}

		var node yaml.Node
		dec := yaml.NewDecoder(bytes.NewReader(contents))
		dec.SetScanBlockScalarAsLiteral(true)
		if err := dec.Decode(&node); err != nil {
			return nil, fmt.Errorf("failed to parse yaml for %s: %w", pth, err)
		}

		// Remarshal the content before any modification so we can compute the
		// places where a newline should be inserted post-rendering.
		remarshaled, err := marshalYAML(&node)
		if err != nil {
			return nil, fmt.Errorf("failed to remarshal yaml for %s: %w", pth, err)
		}

		newlines := computeNewlineTargets(string(contents), remarshaled)

		r = append(r, &loadResult{
			path:     pth,
			node:     &node,
			contents: string(contents),
			newlines: newlines,
		})
	}

	return r, nil
}

func computeNewlineTargets(before, after string) []int {
	before = strings.TrimPrefix(before, "---\n")

	debug, _ := strconv.ParseBool(os.Getenv("RATCHET_DEBUG_NEWLINE_PARSING"))
	if debug {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Original content:\n")
		for i, l := range strings.Split(string(before), "\n") {
			fmt.Fprintf(os.Stderr, "%3d:  %s\n", i, l)
		}
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Rendered content:\n")
		for i, l := range strings.Split(after, "\n") {
			fmt.Fprintf(os.Stderr, "%3d:  %s\n", i, l)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	result := make([]int, 0, 8)
	afteri, afterLines := 0, strings.Split(after, "\n")
	beforeLines := strings.Split(before, "\n")

	for beforei := 0; beforei < len(beforeLines); beforei++ {
		if afteri >= len(afterLines) {
			result = append(result, beforei)
			continue
		}

		beforeLine := strings.TrimSpace(beforeLines[beforei])
		afterLine := strings.TrimSpace(afterLines[afteri])

		if beforeLine != afterLine && beforeLine == "" {
			result = append(result, beforei)
		} else {
			afteri++
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "newline indicies: %v\n", result)
		fmt.Fprintf(os.Stderr, "\n")
	}

	return result
}

func processMultilineNodes(node *yaml.Node) {
	if node == nil {
		return
	}

	// Check if the node is a scalar and contains newlines
	if node.Kind == yaml.ScalarNode && strings.Contains(node.Value, "\n") {
		node.Style = yaml.LiteralStyle // Use '|' for block scalars
	}

	// Recursively process child nodes if applicable
	for _, child := range node.Content {
		processMultilineNodes(child)
	}
}
