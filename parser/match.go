package parser

import (
	"context"
	"fmt"
	"github.com/sethvargo/ratchet/resolver"
	"gopkg.in/yaml.v3"
	"strconv"
)

type Match struct {
	ref        string
	constraint string
	line       string
}

func NewMatch(ref string, node *yaml.Node) Match {
	originalConstraint, _ := extractOriginalFromComment(node.LineComment)
	return Match{
		ref:        ref,
		constraint: originalConstraint,
		line:       strconv.Itoa(node.Line),
	}
}

func refConsistentWithOriginalConstraint(ctx context.Context, ref string, res resolver.Resolver, m *yaml.Node) (bool, error) {
	originalConstraint, _ := extractOriginalFromComment(m.LineComment)
	r, err := res.Resolve(ctx, fmt.Sprintf("%s%s", resolver.RefPrefix(ref), originalConstraint))
	if err != nil {
		return false, fmt.Errorf("resolve %v", err)
	}

	if r != resolver.DenormalizeRef(ref) {
		return false, nil
	}

	return true, nil
}
