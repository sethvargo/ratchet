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

// writeYAMLFilesSurgical writes files using surgical text replacement instead of
// re-serializing the YAML. This preserves the original formatting of the file.
// It walks the modified yaml.Node tree and applies text replacements based on
// line/column positions.
func (r loadResults) writeYAMLFilesSurgical(outPath string) error {
	var merr error

	for pth, f := range r {
		outFile := outPath
		if strings.HasSuffix(outPath, "/") {
			outFile = filepath.Join(outPath, pth)
		}
		if outFile == "" {
			outFile = pth
		}

		final := applySurgicalReplacements(f.contents, f.node)

		if err := atomic.Write(pth, outFile, strings.NewReader(final)); err != nil {
			merr = errors.Join(merr, fmt.Errorf("failed to save file %s: %w", outFile, err))
			continue
		}
	}

	return merr
}

// applySurgicalReplacements walks the yaml.Node tree and applies text replacements
// to the original content based on the node's line/column positions.
func applySurgicalReplacements(contents string, node *yaml.Node) string {
	// Collect all replacements from the node tree
	var replacements []surgicalReplacement
	collectReplacements(node, contents, &replacements)

	if len(replacements) == 0 {
		return contents
	}

	// Sort by line descending, then column descending, so we can apply
	// replacements without affecting positions of subsequent ones
	slices.SortFunc(replacements, func(a, b surgicalReplacement) int {
		if a.line != b.line {
			return b.line - a.line
		}
		return b.col - a.col
	})

	lines := strings.Split(contents, "\n")

	for _, rep := range replacements {
		if rep.line < 1 || rep.line > len(lines) {
			continue
		}

		lineIdx := rep.line - 1
		line := lines[lineIdx]

		// Find the old value starting from the column position
		colIdx := rep.col - 1
		if colIdx < 0 || colIdx >= len(line) {
			continue
		}

		// Find the old value in the line
		valueStart := strings.Index(line[colIdx:], rep.oldValue)
		if valueStart == -1 {
			continue
		}
		valueStart += colIdx
		valueEnd := valueStart + len(rep.oldValue)

		// Find where any existing comment starts (after the value)
		commentStart := -1
		rest := line[valueEnd:]
		for i := 0; i < len(rest); i++ {
			if rest[i] == '#' {
				commentStart = valueEnd + i
				break
			}
		}

		// Build the new line: keep prefix, add new value, then new comment
		var newLine string
		if commentStart != -1 {
			// There's an existing comment - replace value and everything after
			newLine = line[:valueStart] + rep.newValue
		} else {
			// No existing comment - just replace value
			newLine = line[:valueStart] + rep.newValue + line[valueEnd:]
		}

		// Add new comment if specified
		if rep.newComment != "" {
			// Check if the comment already includes the # prefix
			if strings.HasPrefix(rep.newComment, "#") {
				newLine = newLine + " " + rep.newComment
			} else {
				newLine = newLine + " # " + rep.newComment
			}
		}

		lines[lineIdx] = newLine
	}

	return strings.Join(lines, "\n")
}

type surgicalReplacement struct {
	line       int
	col        int
	oldValue   string
	newValue   string
	newComment string
}

// collectReplacements walks the node tree and collects replacements for nodes
// that have been modified by the parser (detected by presence of "ratchet:" in LineComment).
func collectReplacements(node *yaml.Node, contents string, replacements *[]surgicalReplacement) {
	if node == nil {
		return
	}

	// Only process scalar nodes that have been modified by Pin/Update/Upgrade
	// These are identified by having "ratchet:" in the LineComment
	if node.Kind == yaml.ScalarNode && node.Line > 0 && node.Column > 0 &&
		strings.Contains(node.LineComment, "ratchet:") {

		lines := strings.Split(contents, "\n")
		if node.Line <= len(lines) {
			line := lines[node.Line-1]
			colIdx := node.Column - 1

			if colIdx >= 0 && colIdx < len(line) {
				origValue := extractValueAtPosition(line, colIdx)

				if origValue != "" && origValue != node.Value {
					*replacements = append(*replacements, surgicalReplacement{
						line:       node.Line,
						col:        node.Column,
						oldValue:   origValue,
						newValue:   node.Value,
						newComment: node.LineComment,
					})
				}
			}
		}
	}

	// Recurse into children
	for _, child := range node.Content {
		collectReplacements(child, contents, replacements)
	}
}

// extractValueAtPosition extracts the YAML value at the given position in the line.
// Handles both quoted and unquoted values.
func extractValueAtPosition(line string, col int) string {
	if col >= len(line) {
		return ""
	}

	rest := line[col:]

	// Handle quoted strings
	if len(rest) > 0 && (rest[0] == '\'' || rest[0] == '"') {
		quote := rest[0]
		end := 1
		for end < len(rest) {
			if rest[end] == byte(quote) && (end == 0 || rest[end-1] != '\\') {
				return rest[1:end]
			}
			end++
		}
	}

	// Handle unquoted values - read until whitespace or comment
	end := 0
	for end < len(rest) {
		ch := rest[end]
		if ch == ' ' || ch == '\t' || ch == '#' || ch == '\n' || ch == '\r' {
			break
		}
		end++
	}

	return rest[:end]
}
