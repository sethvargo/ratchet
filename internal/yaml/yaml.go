package yaml

import (
	"fmt"
	"io"
	"os"

	"github.com/sethvargo/ratchet/internal/atomic"
	"gopkg.in/yaml.v3"
)

// Write encodes the yaml node into the given writer.
func Write(w io.Writer, m *yaml.Node) error {
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

// WriteFile renders the given yaml and atomically writes it to the provided
// filepath.
func WriteFile(src, dst string, m *yaml.Node) (retErr error) {
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
		if err := Write(w, m); err != nil {
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

// Parse parses the given reader as a yaml node.
func Parse(r io.Reader) (*yaml.Node, error) {
	var m yaml.Node
	if err := yaml.NewDecoder(r).Decode(&m); err != nil {
		return nil, fmt.Errorf("failed to decode yaml: %w", err)
	}
	return &m, nil
}

// ParseFile opens the file at the path and parses it as yaml. It closes the
// file handle.
func ParseFile(pth string) (m *yaml.Node, retErr error) {
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

	m, retErr = Parse(f)
	return
}
