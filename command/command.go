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
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/internal/atomic"
	"github.com/sethvargo/ratchet/internal/version"
)

// Commands is the main list of all commands.
var Commands = map[string]Command{
	"check":   &CheckCommand{},
	"lint":    &LintCommand{},
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
	node     *yaml.Node
	contents string
	newlines []int
}

func (r *loadResult) marshalYAML() (string, error) {
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

type loadResults map[string]*loadResult

func (r loadResults) nodes() map[string]*yaml.Node {
	m := make(map[string]*yaml.Node, len(r))
	for name, lr := range r {
		m[name] = lr.node
	}
	return m
}

func loadYAMLFiles(fsys fs.FS, paths []string) (loadResults, error) {
	r := make(loadResults, len(paths))

	for _, pth := range paths {
		// Normalize the file path to ensure consistent behavior across different
		// operating systems. filepath.Clean removes redundant elements, and
		// filepath.ToSlash converts Windows-style backslashes to slashes.
		pth = filepath.ToSlash(filepath.Clean(pth))
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

		if _, ok := r[pth]; ok {
			return nil, fmt.Errorf("internal error: entry already exists for %q: %v", pth, r)
		}

		r[pth] = &loadResult{
			node:     &node,
			contents: string(contents),
			newlines: newlines,
		}
	}

	return r, nil
}

func (r loadResults) writeYAMLFiles(outPath string) error {
	var merr error

	for pth, f := range r {
		outFile := outPath
		if strings.HasSuffix(outPath, "/") {
			outFile = filepath.Join(outPath, pth)
		}
		if outFile == "" {
			outFile = pth
		}

		final, err := f.marshalYAML()
		if err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to marshal yaml for %s: %w", pth, err))
			continue
		}

		if err := atomic.Write(pth, outFile, strings.NewReader(final)); err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to save file %s: %w", outFile, err))
			continue
		}
	}

	return merr
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
