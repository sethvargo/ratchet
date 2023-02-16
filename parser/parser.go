package parser

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"gopkg.in/yaml.v3"

	"github.com/sethvargo/ratchet/resolver"
)

const (
	ratchetPrefix  = "ratchet:"
	ratchetExclude = "ratchet:exclude"
)

// Parser defines an interface which parses references out of the given yaml
// node.
type Parser interface {
	Parse(m *yaml.Node) (*RefsList, error)
}

var parserFactory = map[string]func() Parser{
	"actions":    func() Parser { return new(Actions) },
	"circleci":   func() Parser { return new(CircleCI) },
	"cloudbuild": func() Parser { return new(CloudBuild) },
	"drone":      func() Parser { return new(Drone) },
	"gitlabci":   func() Parser { return new(GitLabCI) },
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

// Check iterates over all references in the yaml and checks if they are pinned
// to an absolute reference. It ignores "ratchet:exclude" nodes from the lookup.
func Check(ctx context.Context, parser Parser, m *yaml.Node) error {
	refsList, err := parser.Parse(m)
	if err != nil {
		return err
	}
	refs := refsList.All()

	var unpinned []string
	for ref, nodes := range refs {
		ref = resolver.DenormalizeRef(ref)

		// Pre-filter any nodes that should be excluded from the lookup.
		hasAny := false
		for _, node := range nodes {
			if !shouldExclude(node.LineComment) {
				hasAny = true
				break
			}
		}
		if !hasAny {
			continue
		}

		if !isAbsolute(ref) {
			unpinned = append(unpinned, ref)
		}
	}

	if l := len(unpinned); l > 0 {
		return fmt.Errorf("found %d unpinned refs: %q", l, unpinned)
	}

	return nil
}

// Cache is to cache the results of the resolver. This is
// to avoid making multiple calls to the resolver for the same
// reference. We parse all the files and de-duplicate references,
// then resolve all references for faster lookup. Eventually we
// end up with tight time-window where there could be a reference
// drift which we could resolve differently if an upstream developer
// pushes a new version.
type Cache struct {
	// refs is a map of denormalized references to the corresponding
	// resolved reference.
	refs map[string]string
}

var cache = &Cache{
	refs: make(map[string]string),
}

// CacheLen returns the number of items in the cache for unit-testing purposes.
func CacheLen() int {
	return len(cache.refs)
}

// CacheInvalidate invalidates the cache. Useful when cleaning up after each test suite.
func CacheInvalidate() {
	cache.refs = make(map[string]string)
}

// FetchAndCacheReferences caches all the references in the given yaml nodes.
func FetchAndCacheReferences(ctx context.Context, res resolver.Resolver, parser Parser, yamls []*yaml.Node, concurrency int64, forceResolve bool) error {
	var (
		cacheLock sync.Mutex
		merrLock  sync.Mutex
		merr      *multierror.Error
	)

	for _, m := range yamls {
		refsList, err := parser.Parse(m)
		if err != nil {
			return err
		}
		refs := refsList.All()

		// To avoid unnecessary network calls, `pin` command skips the absolute
		// references, by default. For `update` command, it `unpin` all references
		// first, then resolve all the references to update the cache. So we need
		// a `forceResolve` flag to force resolve all the references, especially
		// for unit-testing scenarios.
		if !forceResolve {
			// Remove any absolute references from the list. We do not do
			// HTTP lookups for absolute references.
			for ref := range refs {
				if isAbsolute(ref) {
					delete(refs, ref)
				}
			}
		}

		sem := semaphore.NewWeighted(concurrency)

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

				denormRef := resolver.DenormalizeRef(ref)

				// If we have already cached this reference, skip this reference.
				if _, ok := cache.refs[denormRef]; ok {
					return
				}

				resolved, err := res.Resolve(ctx, ref)
				if err != nil {
					merrLock.Lock()
					merr = multierror.Append(merr, fmt.Errorf("failed to resolve %q: %w", ref, err))
					merrLock.Unlock()
				}

				cacheLock.Lock()
				cache.refs[denormRef] = resolved
				cacheLock.Unlock()
			}()
		}

		if err := sem.Acquire(ctx, concurrency); err != nil {
			return fmt.Errorf("failed to wait for semaphore: %w", err)
		}
	}

	return merr.ErrorOrNil()
}

// Pin extracts all references from the given YAML document and resolves them
// using the given resolver, updating the associated YAML nodes.
func Pin(ctx context.Context, res resolver.Resolver, parser Parser, m *yaml.Node, concurrency int64) error {
	// If we run against a single file, cache would be empty. So lazy-load
	// the cache if not already loaded for single given node.
	if len(cache.refs) == 0 {
		if err := FetchAndCacheReferences(ctx, res, parser, []*yaml.Node{m}, concurrency, false); err != nil {
			return fmt.Errorf("load cache: %w", err)
		}
	}

	refsList, err := parser.Parse(m)
	if err != nil {
		return err
	}
	refs := refsList.All()

	// Remove any absolute references from the list. We do not want to pin
	// absolute references since they are already pinned.
	for ref := range refs {
		if isAbsolute(ref) {
			delete(refs, ref)
		}
	}

	var merr *multierror.Error

	for ref, nodes := range refs {
		ref := ref
		nodes := nodes

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
			continue
		}

		denormRef := resolver.DenormalizeRef(ref)

		// If the reference is already cached, use the cached value.
		if resolved, ok := cache.refs[denormRef]; ok {
			for _, node := range nodes {
				node.LineComment = appendOriginalToComment(node.LineComment, node.Value)
				node.Value = strings.Replace(node.Value, denormRef, resolved, 1)
			}
		}
	}

	return merr.ErrorOrNil()
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

	if m.LineComment != "" && !shouldExclude(m.LineComment) {
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
