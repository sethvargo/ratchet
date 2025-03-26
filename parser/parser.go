package parser

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/linter"
	"github.com/sethvargo/ratchet/resolver"
	"golang.org/x/sync/semaphore"
)

const (
	ratchetPrefix  = "ratchet:"
	ratchetExclude = "ratchet:exclude"
)

// Parser defines an interface which parses references out of the given yaml
// node.
type Parser interface {
	DenormalizeRef(ref string) string
	Parse(nodes map[string]*yaml.Node) (*RefsList, error)
}

var parserFactory = map[string]func() Parser{
	"actions":    func() Parser { return new(Actions) },
	"circleci":   func() Parser { return new(CircleCI) },
	"cloudbuild": func() Parser { return new(CloudBuild) },
	"drone":      func() Parser { return new(Drone) },
	"gitlabci":   func() Parser { return new(GitLabCI) },
	"tekton":     func() Parser { return new(Tekton) },
}

var parsers = sync.OnceValue(func() []string {
	return slices.Sorted(maps.Keys(parserFactory))
})

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
	return parsers()
}

// Check iterates over all references in the yaml and checks if they are pinned
// to an absolute reference. It ignores "ratchet:exclude" nodes from the lookup.
func Check(ctx context.Context, parser Parser, nodes map[string]*yaml.Node) error {
	unpinned := make(map[string]struct{}, 8)

	violations, err := Lint(ctx, parser, nodes)
	if err != nil {
		return err
	}

	for _, violation := range violations {
		unpinned[violation.Contents] = struct{}{}
	}

	if l := len(unpinned); l > 0 {
		list := slices.Sorted(maps.Keys(unpinned))
		return fmt.Errorf("found %d unpinned refs: %q", l, list)
	}

	return nil
}

// Lint iterates over all references in the yaml and checks if they are pinned
// to an absolute reference. It returns a structure of violations.
//
// It ignores "ratchet:exclude" nodes from the lookup.
func Lint(ctx context.Context, parser Parser, nodes map[string]*yaml.Node) ([]*linter.Violation, error) {
	var violations []*linter.Violation

	// This is a little bit weird, but we parse files individually so we can know
	// which file we're operating on. Other functions intentionally munge
	// everything together to mminimze API calls, but we need to retain the
	// information here for reporting.
	for filename, document := range nodes {
		refsList, err := parser.Parse(map[string]*yaml.Node{
			filename: document,
		})
		if err != nil {
			return nil, err
		}
		refs := refsList.All()

		for _, nodes := range refs {
			for _, node := range nodes {
				if shouldExclude(node.LineComment) {
					continue
				}

				if !isAbsolute(node.Value) {
					violations = append(violations, &linter.Violation{
						Filename: filename,
						Contents: node.Value,
						Line:     node.Line,
						Column:   node.Column,
					})
				}
			}
		}

	}

	return violations, nil
}

// Pin extracts all references from the given YAML document and resolves them
// using the given resolver, updating the associated YAML nodes.
func Pin(ctx context.Context, res resolver.Resolver, parser Parser, nodes map[string]*yaml.Node, concurrency int64) error {
	refsList, err := parser.Parse(nodes)
	if err != nil {
		return err
	}
	refs := refsList.All()

	sem := semaphore.NewWeighted(concurrency)

	var merrLock sync.Mutex
	var merr error

	for ref, nodes := range refs {
		ref := ref
		nodes := nodes

		// Remove any absolute references from the list. We do not want to pin
		// absolute references since they are already pinned.
		if isAbsolute(ref) {
			continue
		}

		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}

		go func() {
			defer sem.Release(1)

			// Pre-filter any nodes that should be excluded from the lookup. It's
			// important we do this before doing any lookups because, if the node list
			// is empty, we don't want to make any API calls.
			//
			// It would actually be better to do this in the actual parser, but that
			// would not scale to all the parsers (and would be difficult to debug).
			tmp := nodes[:0]
			for _, node := range nodes {
				if !shouldExclude(node.LineComment) {
					tmp = append(tmp, node)
				}
			}
			nodes = tmp

			// If there's no nodes left that are eligible, skip this reference.
			if len(nodes) == 0 {
				return
			}

			resolved, err := res.Resolve(ctx, ref)
			if err != nil {
				merrLock.Lock()
				merr = errors.Join(merr, fmt.Errorf("failed to resolve %q: %w", ref, err))
				merrLock.Unlock()
			}

			denormRef := resolver.DenormalizeRef(ref)

			for _, node := range nodes {
				node.LineComment = appendOriginalToComment(node.LineComment, node.Value)
				node.Value = strings.Replace(node.Value, denormRef, resolved, 1)
			}
		}()
	}

	if err := sem.Acquire(ctx, concurrency); err != nil {
		return fmt.Errorf("failed to wait for semaphore: %w", err)
	}

	return merr
}

func Upgrade(ctx context.Context, res resolver.Resolver, parser Parser, nodes map[string]*yaml.Node, concurrency int64) error {
	refsList, err := parser.Parse(nodes)
	if err != nil {
		return err
	}
	refs := refsList.All()

	sem := semaphore.NewWeighted(concurrency)

	var merrLock sync.Mutex
	var merr error

	for ref, nodes := range refs {
		ref := ref
		nodes := nodes

		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}

		go func() {
			defer sem.Release(1)

			// Pre-filter any nodes that should be excluded from the lookup. It's
			// important we do this before doing any lookups because, if the node list
			// is empty, we don't want to make any API calls.
			//
			// It would actually be better to do this in the actual parser, but that
			// would not scale to all the parsers (and would be difficult to debug).
			tmp := nodes[:0]
			for _, node := range nodes {
				if !shouldExclude(node.LineComment) {
					tmp = append(tmp, node)
				}
			}
			nodes = tmp

			// If there's no nodes left that are eligible, skip this reference.
			if len(nodes) == 0 {
				return
			}

			latest, err := res.LatestVersion(ctx, ref)
			if err != nil {
				merrLock.Lock()
				merr = errors.Join(merr, fmt.Errorf("failed to resolve %q: %w", ref, err))
				merrLock.Unlock()
			}

			denormRef := parser.DenormalizeRef(ref)
			denormLatest := parser.DenormalizeRef(latest)

			for _, node := range nodes {
				node.LineComment = appendOriginalToComment(node.LineComment, denormLatest)
				node.Value = strings.Replace(node.Value, denormRef, denormLatest, 1)
			}
		}()
	}

	if err := sem.Acquire(ctx, concurrency); err != nil {
		return fmt.Errorf("failed to wait for semaphore: %w", err)
	}

	return merr
}

// Unpin removes any pinned references and updates the actual YAML to be the
// original reference, leaving any other comment intact. This effectively
// replaces the YAML with the cached comment, which could result in losing the
// current pin.
//
// This function does not make any outbound network calls and relies solely on
// information in the document.
func Unpin(ctx context.Context, files map[string]*yaml.Node) error {
	nodes := make([]*yaml.Node, 0, len(files))
	for _, node := range files {
		nodes = append(nodes, node)
	}
	return unpin(ctx, nodes)
}

func unpin(ctx context.Context, nodes []*yaml.Node) error {
	for _, node := range nodes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if node.LineComment != "" && !shouldExclude(node.LineComment) {
			if v, rest := extractOriginalFromComment(node.LineComment); v != "" {
				node.Value = v
				node.LineComment = rest
			}
		}

		if err := unpin(ctx, node.Content); err != nil {
			return err
		}
	}

	return nil
}

// shouldExclude returns true if the given comment includes a ratchet exclude
// annotation, false otherwise.
func shouldExclude(comment string) bool {
	return strings.Contains(comment, ratchetExclude)
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
