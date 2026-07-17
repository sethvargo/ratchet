package parser

import (
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
)

type RefsList struct {
	once sync.Once
	refs map[string][]*yaml.Node
}

func (l *RefsList) Add(ref string, m *yaml.Node) {
	l.once.Do(l.init)
	l.refs[ref] = append(l.refs[ref], m)
}

func (l *RefsList) Refs() []string {
	l.once.Do(l.init)
	return slices.Sorted(maps.Keys(l.refs))
}

func (l *RefsList) All() map[string][]*yaml.Node {
	l.once.Do(l.init)
	return maps.Clone(l.refs)
}

func (l *RefsList) init() {
	if l.refs == nil {
		l.refs = make(map[string][]*yaml.Node)
	}
}

// mapPairs iterates over the key/value pairs of a YAML mapping node. YAML stores
// a mapping's children as a flat [key, value, key, value, ...] slice, so pairing
// them explicitly ensures a scalar value that happens to equal a mapping key is
// never mistaken for a key and never triggers an out-of-range access on the
// trailing entry.
func mapPairs(node *yaml.Node) iter.Seq2[*yaml.Node, *yaml.Node] {
	return func(yield func(*yaml.Node, *yaml.Node) bool) {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if !yield(node.Content[i], node.Content[i+1]) {
				return
			}
		}
	}
}

// isAbsolute returns true if the given reference is absolute, or false
// otherwise. A reference is absolute if it is pinned.
//
// A actions ref is absolute if the ref is a 40-character SHA composed of only hex
// characters. GitHub actually forbids this format for branch names.
//
// A container ref is absolute if it's a sha256 with a hex digest.
func isAbsolute(ref string) bool {
	parts := strings.Split(ref, "@")
	last := parts[len(parts)-1]

	if len(last) == 40 && isAllHex(last) {
		return true
	}

	if len(last) == 71 && last[:6] == "sha256" && isAllHex(last[7:]) {
		return true
	}

	return false
}

// isAllHex returns true if the given string is all hex characters, false
// otherwise.
func isAllHex(s string) bool {
	for _, ch := range s {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
			return false
		}
	}
	return true
}
