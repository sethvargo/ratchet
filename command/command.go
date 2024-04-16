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
	"strings"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
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
func marshalYAML(m *yaml.Node) ([]byte, error) {
	var b bytes.Buffer

	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("failed to encode yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize yaml: %w", err)
	}
	return b.Bytes(), nil
}

type loadResult struct {
	path     string
	node     *yaml.Node
	contents []byte
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
		if err := yaml.Unmarshal(contents, &node); err != nil {
			return nil, fmt.Errorf("failed to parse yaml for %s: %w", pth, err)
		}

		r = append(r, &loadResult{
			path:     pth,
			node:     &node,
			contents: contents,
		})
	}

	return r, nil
}

// FixIndentation corrects the indentation for the given loadResult and edits it in-place.
func FixIndentation(f *loadResult) error {
	updated, err := marshalYAML(f.node)
	if err != nil {
		return fmt.Errorf("failed to marshal yaml for %s: %w", f.path, err)
	}
	beforeContent := string(f.contents)
	afterContent := string(updated)
	lines := strings.Split(beforeContent, "\n")

	editedLines := []string{}
	lastLineInAfter := 0
	for _, l := range lines {
		token := strings.TrimSpace(l)
		if token == "" {
			editedLines = append(editedLines, l)
			continue
		}
		a := strings.Index(afterContent[lastLineInAfter:], token)
		if a == -1 {
			a = 0
		}
		after := a + lastLineInAfter
		newline := strings.LastIndex(afterContent[lastLineInAfter:after], "\n") + lastLineInAfter

		if newline == -1 {
			newline = 0
		} else if after != newline {
			newline++
		}

		lineWithCorrectIndent := afterContent[newline:after] + token
		editedLines = append(editedLines, lineWithCorrectIndent)

		lastLineInAfter = after
	}

	f.contents = []byte(strings.Join(editedLines, "\n"))
	return nil
}

func removeNewLineChanges(beforeContent, afterContent string) string {
	lines := strings.Split(beforeContent, "\n")
	edits := myers.ComputeEdits(span.URIFromPath("before.txt"), beforeContent, afterContent)
	unified := gotextdiff.ToUnified("before.txt", "after.txt", beforeContent, edits)

	editedLines := make(map[int]string)
	// Iterates through all changes and only keep changes to lines that are not empty.
	for _, h := range unified.Hunks {
		// Changes are in-order of delete line followed by insert line for lines that were modified.
		// We want to locate the position of all deletes of non-empty lines and replace
		// these in the original content with the modified line.
		var deletePositions []int
		inserts := 0
		for i, l := range h.Lines {
			if l.Kind == gotextdiff.Delete && l.Content != "\n" {
				deletePositions = append(deletePositions, h.FromLine+i-1-inserts)
			}
			if l.Kind == gotextdiff.Insert && l.Content != "" {
				pos := deletePositions[0]
				deletePositions = deletePositions[1:]
				editedLines[pos] = strings.TrimSuffix(l.Content, "\n")
				inserts++
			}
		}
	}
	var formattedLines []string
	for i, line := range lines {
		if editedLine, ok := editedLines[i]; ok {
			formattedLines = append(formattedLines, editedLine)
		} else {
			formattedLines = append(formattedLines, line)
		}
	}
	return strings.Join(formattedLines, "\n")
}
