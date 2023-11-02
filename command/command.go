//go:generate go run ./cmd/gen/main.go
package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/sethvargo/ratchet/internal/atomic"
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

func keepNewlinesEnv() bool {
	value := true
	if v, ok := os.LookupEnv("RATCHET_EXP_KEEP_NEWLINES"); ok {
		if t, err := strconv.ParseBool(v); err == nil {
			value = t
		}
	}
	return value
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

// writeYAML encodes the yaml node into the given writer.
func writeYAML(w io.Writer, m *yaml.Node) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("failed to encode yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("failed to finalize yaml: %w", err)
	}
	return nil
}

// writeYAMLFile renders the given yaml and atomically writes it to the provided
// filepath.
func writeYAMLFile(src, dst string, m *yaml.Node) (retErr error) {
	r, w := io.Pipe()
	defer func() {
		if err := r.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close reader: %w", err)
		}
	}()
	defer func() {
		if err := w.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("failed to closer writer: %w", err)
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		if err := writeYAML(w, m); err != nil {
			select {
			case errCh <- fmt.Errorf("failed to render yaml: %w", err):
			default:
			}
		}

		if err := w.Close(); err != nil {
			select {
			case errCh <- fmt.Errorf("failed to close writer: %w", err):
			default:
			}
		}
	}()

	if err := atomic.Write(src, dst, r); err != nil {
		retErr = fmt.Errorf("failed to save file %s: %w", dst, err)
		return
	}

	select {
	case err := <-errCh:
		retErr = err
		return
	default:
		return
	}
}

// parseYAML parses the given reader as a yaml node.
func parseYAML(r io.Reader) (*yaml.Node, error) {
	var m yaml.Node
	if err := yaml.NewDecoder(r).Decode(&m); err != nil {
		return nil, fmt.Errorf("failed to decode yaml: %w", err)
	}
	return &m, nil
}

// parseYAMLFile opens the file at the path and parses it as yaml. It closes the
// file handle.
func parseYAMLFile(pth string) (m *yaml.Node, retErr error) {
	f, err := os.Open(pth)
	if err != nil {
		retErr = fmt.Errorf("failed to open file: %w", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close file: %w", err)
		}
	}()

	m, retErr = parseYAML(f)
	return
}

func parseFile(pth string) (contents string, retErr error) {
	f, err := os.Open(pth)
	if err != nil {
		retErr = fmt.Errorf("failed to open file: %w", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close file: %w", err)
		}
	}()
	c, retErr := io.ReadAll(f)
	contents = string(c)
	return
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
