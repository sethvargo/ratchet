package command

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sethvargo/ratchet/internal/atomic"
	"github.com/sethvargo/ratchet/internal/version"
	"gopkg.in/yaml.v3"
)

// Commands is the main list of all commands.
var Commands = map[string]Command{
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
		fmt.Fprintln(os.Stderr, version.String())
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
