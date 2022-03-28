package resolver

import (
	"context"
	"fmt"
	"strings"
)

const (
	ActionsProtocol   = "actions://"
	ContainerProtocol = "container://"
)

type Resolver interface {
	Resolve(context.Context, string) (string, error)
}

type DefaultResolver struct {
	actions   *Actions
	container *Container
}

func NewDefaultResolver(ctx context.Context) (Resolver, error) {
	actions, err := NewActions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to setup actions resolver: %w", err)
	}

	container, err := NewContainer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to setup docker resolver: %w", err)
	}

	return &DefaultResolver{
		actions:   actions,
		container: container,
	}, nil
}

func (r *DefaultResolver) Resolve(ctx context.Context, ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, ActionsProtocol):
		return r.actions.Resolve(ctx, strings.TrimPrefix(ref, ActionsProtocol))
	case strings.HasPrefix(ref, ContainerProtocol):
		return r.container.Resolve(ctx, strings.TrimPrefix(ref, ContainerProtocol))
	default:
		return "", fmt.Errorf("missing resolver protocol")
	}
}

func DenormalizeRef(in string) string {
	in = strings.TrimPrefix(in, ActionsProtocol)
	in = strings.TrimPrefix(in, ContainerProtocol)
	return in
}
