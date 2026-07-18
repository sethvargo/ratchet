package resolver

import (
	"context"
	"fmt"
	"strings"
	"time"
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

// Option configures the default resolver.
type Option func(*resolverOptions)

type resolverOptions struct {
	bakeDelay time.Duration
}

// WithBakeDelay sets the minimum age an artifact must have before Resolve will
// pin it. Zero or negative disables the check.
func WithBakeDelay(d time.Duration) Option {
	return func(o *resolverOptions) {
		o.bakeDelay = d
	}
}

// DefaultResolver is the default resolver.
type DefaultResolver struct {
	actions   *Actions
	container *Container

	// bakeDelay is the minimum age an artifact must have before Resolve will
	// pin it. Zero or negative disables the check.
	bakeDelay time.Duration
}

// NewDefaultResolver returns the default resolver. Options such as WithBakeDelay
// customize its behavior.
func NewDefaultResolver(ctx context.Context, opts ...Option) (Resolver, error) {
	var o resolverOptions
	for _, opt := range opts {
		opt(&o)
	}

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
		bakeDelay: o.bakeDelay,
	}, nil
}

// Resolve resolves the ref.
func (r *DefaultResolver) Resolve(ctx context.Context, ref string) (string, error) {
	switch {
	case strings.HasPrefix(ref, ActionsProtocol):
		value := strings.TrimPrefix(ref, ActionsProtocol)
		resolved, err := r.actions.Resolve(ctx, value)
		if err != nil {
			return "", err
		}
		if err := r.checkBakeDelay(ctx, r.actions, value, resolved); err != nil {
			return "", err
		}
		return resolved, nil
	case strings.HasPrefix(ref, ContainerProtocol):
		value := strings.TrimPrefix(ref, ContainerProtocol)
		resolved, err := r.container.Resolve(ctx, value)
		if err != nil {
			return "", err
		}
		if err := r.checkBakeDelay(ctx, r.container, value, resolved); err != nil {
			return "", err
		}
		return resolved, nil
	default:
		return "", fmt.Errorf("missing resolver protocol")
	}
}

// checkBakeDelay enforces the bake delay against a resolved artifact. It is a
// no-op when the delay is disabled or the resolver cannot report an age
// (graceful degradation).
func (r *DefaultResolver) checkBakeDelay(ctx context.Context, res any, ref, resolved string) error {
	if r.bakeDelay <= 0 {
		return nil
	}

	tr, ok := res.(TimestampResolver)
	if !ok {
		return nil
	}

	ts, err := tr.ResolvedTimestamp(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to determine age of %s: %w", ref, err)
	}
	if ts == nil {
		return nil
	}

	now := time.Now().UTC()
	if age := now.Sub(ts.Time); age < r.bakeDelay {
		return fmt.Errorf(
			"%s resolves to %s; %s dated %s (%s old) is younger than bake delay %s; wait or set -bake-delay 0",
			ref, resolved, ts.Source, ts.Time.Format(time.RFC3339), age.Round(time.Second), r.bakeDelay,
		)
	}
	return nil
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
