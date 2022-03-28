package parser

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sethvargo/ratchet/resolver"
	"gopkg.in/yaml.v3"
)

const ratchetPrefix = "ratchet:"

// Parser defines an interface which parses references out of the given yaml
// node.
type Parser interface {
	Parse(m *yaml.Node) (*RefsList, error)
}

var parserFactory = map[string]func() Parser{
	"actions":    func() Parser { return new(Actions) },
	"cloudbuild": func() Parser { return new(CloudBuild) },
}

// For returns the parser that corresponds to the given name.
func For(ctx context.Context, name string) (Parser, error) {
	typ := strings.ToLower(strings.TrimSpace(name))
	if v, ok := parserFactory[typ]; ok {
		return v(), nil
	}
	return nil, fmt.Errorf("unknown parser %q, valid parsers are %q",
		typ, List())
}

// List returns the list of parsers.
func List() []string {
	cp := make([]string, 0, len(parserFactory))
	for key := range parserFactory {
		cp = append(cp, key)
	}
	sort.Strings(cp)
	return cp
}

// Pin extracts all references from the given YAML document and resolves them
// using the given resolver, updating the associated YAML nodes.
func Pin(ctx context.Context, res resolver.Resolver, parser Parser, m *yaml.Node) error {
	refsList, err := parser.Parse(m)
	if err != nil {
		return err
	}
	refs := refsList.All()

	for ref, nodes := range refs {
		resolved, err := res.Resolve(ctx, ref)
		if err != nil {
			return err
		}

		denormRef := resolver.DenormalizeRef(ref)

		for _, node := range nodes {
			node.LineComment = appendOriginalToComment(node.LineComment, node.Value)
			node.Value = strings.Replace(node.Value, denormRef, resolved, 1)
		}
	}

	return nil
}

// Unpin removes any pinned references and updates the actual YAML to be the
// original reference, leaving any other comment intact. This effectively
// replaces the YAML with the cached comment, which could result in losing the
// current pin.
//
// This function does not make any outbound network calls and relies solely on
// information in the document.
func Unpin(m *yaml.Node) error {
	if m == nil {
		return nil
	}

	if m.LineComment != "" {
		if v, rest := extractOriginalFromComment(m.LineComment); v != "" {
			m.Value = v
			m.LineComment = rest
		}
	}

	for _, child := range m.Content {
		if err := Unpin(child); err != nil {
			return err
		}
	}

	return nil
}

// appendOriginalToComment appends the original value to the end of an original
// comment, returning the new comment value.
func appendOriginalToComment(comment, pin string) string {
	// Remove any existing ratchet references - this prevents endlessly appending
	// values on subsequent runs.
	_, comment = extractOriginalFromComment(comment)

	if comment == "" {
		return ratchetPrefix + pin
	}

	return comment + " " + ratchetPrefix + pin
}

// extractOriginalFromComment pulls the originally pinned value from the comment
// on the string.
func extractOriginalFromComment(comment string) (string, string) {
	idx := strings.Index(comment, ratchetPrefix)
	if idx < 0 {
		return "", comment
	}

	rest := comment[idx+len(ratchetPrefix):]
	parts := strings.SplitN(rest, " ", 2)
	switch len(parts) {
	case 1:
		return parts[0], ""
	case 2:
		return parts[0], parts[1]
	default:
		panic(fmt.Sprintf("impossible number of parts to extract %q", rest))
	}
}
