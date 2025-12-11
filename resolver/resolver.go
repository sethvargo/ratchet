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

	// ResolveWithOptions resolves the given reference with the provided options,
	// returning the resolved reference or an error. If the provided context is
	// canceled, the resolution is also canceled.
	ResolveWithOptions(context.Context, string, *ResolverOptions) (string, error)

	// LatestVersion resolves the given reference to the most recent release version,
	// returning the resolved reference or an error. If the provided context is
	// canceled, the resolution is also canceled.
	LatestVersion(context.Context, string) (string, error)

	// LatestVersionWithOptions resolves the given reference to the most recent
	// release version that meets the provided options, returning the resolved
	// reference or an error. If the provided context is canceled, the resolution
	// is also canceled.
	LatestVersionWithOptions(context.Context, string, *ResolverOptions) (string, error)
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
	return r.ResolveWithOptions(ctx, ref, nil)
}

// ResolveWithOptions resolves the ref with the provided options.
func (r *DefaultResolver) ResolveWithOptions(ctx context.Context, ref string, opts *ResolverOptions) (string, error) {
	if opts == nil {
		opts = DefaultResolverOptions()
	}

	switch {
	case strings.HasPrefix(ref, ActionsProtocol):
		return r.actions.ResolveWithOptions(ctx, strings.TrimPrefix(ref, ActionsProtocol), opts)
	case strings.HasPrefix(ref, ContainerProtocol):
		return r.container.Resolve(ctx, strings.TrimPrefix(ref, ContainerProtocol))
	default:
		return "", fmt.Errorf("missing resolver protocol")
	}
}

// LatestVersion upgrades the ref.
func (r *DefaultResolver) LatestVersion(ctx context.Context, ref string) (string, error) {
	return r.LatestVersionWithOptions(ctx, ref, nil)
}

// LatestVersionWithOptions upgrades the ref with the provided options.
func (r *DefaultResolver) LatestVersionWithOptions(ctx context.Context, ref string, opts *ResolverOptions) (string, error) {
	if opts == nil {
		opts = DefaultResolverOptions()
	}

	switch {
	case strings.HasPrefix(ref, ActionsProtocol):
		res, err := r.actions.LatestVersionWithOptions(ctx, strings.TrimPrefix(ref, ActionsProtocol), opts)
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
