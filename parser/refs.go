package parser

import (
	"sort"
	"strings"
	"sync"

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

	cp := make([]string, 0, len(l.refs))
	for k := range l.refs {
		cp = append(cp, k)
	}
	sort.Strings(cp)
	return cp
}

func (l *RefsList) All() map[string][]*yaml.Node {
	l.once.Do(l.init)

	cp := make(map[string][]*yaml.Node, len(l.refs))
	for k, v := range l.refs {
		cp[k] = append(cp[k], v...)
	}
	return cp
}

func (l *RefsList) init() {
	if l.refs == nil {
		l.refs = make(map[string][]*yaml.Node)
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
