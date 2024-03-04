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

// Resolver is an interface that resolvers can implement.
type Resolver interface {
	// Resolve resolves the given reference, returning the resolved reference or
	// an error. If the provided context is canceled, the resolution is also
	// canceled.
	Resolve(context.Context, string) (string, error)

	// LatestVersion resolves the given reference to the most recent release version,
	// returning the resolved reference or an error. If the provided context is
	// canceled, the resolution is also canceled.
	LatestVersion(context.Context, string) (string, error)
}

// DefaultResolver is the default resolver.
type DefaultResolver struct {
	actions   *Actions
	container *Container
}

// NewDefaultResolver returns the default resolver.
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

// Resolve resolves the ref.
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

// LatestVersion upgrades the ref.
func (r *DefaultResolver) LatestVersion(ctx context.Context, ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, ActionsProtocol):
		res, err := r.actions.LatestVersion(ctx, strings.TrimPrefix(ref, ActionsProtocol))
		if err != nil {
			return "", fmt.Errorf("failed to upgrade ref: %w", err)
		}
		return NormalizeActionsRef(res), nil
	case strings.HasPrefix(ref, ContainerProtocol):
		// TODO: Figure out a strategy for container upgrades.
		return ref, nil
	default:
		return "", fmt.Errorf("missing resolver protocol")
	}
}

// DenormalizeRef removes the reference prefix.
func DenormalizeRef(in string) string {
	in = strings.TrimPrefix(in, ActionsProtocol)
	in = strings.TrimPrefix(in, ContainerProtocol)
	return in
}
