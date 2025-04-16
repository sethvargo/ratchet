package formatter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/sethvargo/ratchet/linter"
)

type Violation = linter.Violation

type Formatter interface {
	Format(io.Writer, []*Violation) error
}

var formatterFactory = map[string]Formatter{
	"actions": FormatterFunc(formatActions),
	"human":   FormatterFunc(formatHuman),
	"json":    FormatterFunc(formatJSON),
	"lsp":     FormatterFunc(formatLSP),
	"null":    FormatterFunc(formatNull),
}

var formatters = sync.OnceValue(func() []string {
	return slices.Sorted(maps.Keys(formatterFactory))
})

// For returns the parser that corresponds to the given name.
func For(ctx context.Context, name string) (Formatter, error) {
	typ := strings.ToLower(strings.TrimSpace(name))
	if v, ok := formatterFactory[typ]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("unknown formatter %q, valid formatters are %q",
		typ, List())
}

// List returns the list of parsers.
func List() []string {
	return formatters()
}

// FormatterFunc is a function that implements the [Formatter] interface.
type FormatterFunc func(io.Writer, []*Violation) error

// Format implements the [Formatter] interface.
func (f FormatterFunc) Format(w io.Writer, v []*Violation) error {
	return f(w, v)
}

// formatActions formats in GitHub Actions error output, which will also be
// annotated in the UI.
func formatActions(w io.Writer, violations []*Violation) error {
	var merr error
	for _, v := range violations {
		message := fmt.Sprintf("%s:%d:%d: The reference `%s` is unpinned. Either pin the reference to a SHA or mark the line with `ratchet:exclude`.",
			v.Filename, v.Line, v.Column, v.Contents)
		if _, err := fmt.Fprintf(w, "::error file=%s,line=%d,col=%d,title=Ratchet - Unpinned Reference::%s\n",
			v.Filename, v.Line, v.Column,
			message); err != nil {
			merr = errors.Join(merr, err)
		}
	}
	return merr
}

// formatHuman reports a human-friendly output format.
//
//	<path>:<line>:<column>: <message>
//
// For example:
//
//	.github/workflows/test.yml:37:8: Unpinned reference "actions/checkout@v4"
func formatHuman(w io.Writer, violations []*Violation) error {
	var merr error

	for _, v := range violations {
		if _, err := fmt.Fprintf(w, "%s:%d:%d: Unpinned reference %q\n",
			v.Filename, v.Line, v.Column, v.Contents); err != nil {
			merr = errors.Join(merr, err)
		}
	}

	if len(violations) > 0 {
		if _, err := fmt.Fprintf(w, "\n❌ found %d violation(s)\n",
			len(violations)); err != nil {
			merr = errors.Join(merr, err)
		}
	}

	return merr
}

// formatNull produces no output.
func formatNull(w io.Writer, violations []*Violation) error {
	return nil
}

// formatJSON formats in JSON output.
func formatJSON(w io.Writer, violations []*Violation) error {
	type InternalJSON struct {
		Filename string `json:"filename,omitempty"`
		Contents string `json:"contents,omitempty"`
		Line     int    `json:"line,omitempty"`
		Column   int    `json:"column,omitempty"`
	}

	list := make([]*InternalJSON, 0, len(violations))
	for _, v := range violations {
		list = append(list, &InternalJSON{
			Filename: v.Filename,
			Contents: v.Contents,
			Line:     v.Line,
			Column:   v.Column,
		})
	}

	return json.NewEncoder(w).Encode(list)
}

// formatLSP formats a JSON response that is compatible with the Language Server
// Protocol. This is useful for surfacing findings in an IDE that uses an LSP.
func formatLSP(w io.Writer, violations []*Violation) error {
	type Position struct {
		Line      int `json:"line,omitempty"`
		Character int `json:"character,omitempty"`
	}

	type Range struct {
		Start *Position `json:"start,omitempty"`
		End   *Position `json:"end,omitempty"`
	}

	type InternalJSON struct {
		Message  string `json:"message,omitempty"`
		Code     string `json:"code,omitempty"`
		Severity string `json:"severity,omitempty"`
		Range    *Range `json:"range,omitempty"`
	}

	list := make([]*InternalJSON, 0, len(violations))
	for _, v := range violations {
		list = append(list, &InternalJSON{
			Message:  "Reference is unpinned",
			Code:     "unpinned",
			Severity: "Error",
			Range: &Range{
				Start: &Position{
					Line:      v.Line,
					Character: v.Column,
				},
				End: &Position{
					Line:      v.Line,
					Character: v.Column + len(v.Contents),
				},
			},
		})
	}

	return json.NewEncoder(w).Encode(list)
}
