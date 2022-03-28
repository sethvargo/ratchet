package parser

import (
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
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
