package resolver

import "time"

// ResolverOptions contains options for resolver operations.
type ResolverOptions struct {
	// Cooldown is the minimum age a release must have before it can be used.
	// If a release was published less than this duration ago, it will be skipped.
	// A zero value means no cooldown.
	Cooldown time.Duration
}

// DefaultResolverOptions returns the default resolver options.
func DefaultResolverOptions() *ResolverOptions {
	return &ResolverOptions{
		Cooldown: 0,
	}
}

// ErrCooldownNotMet is returned when a release doesn't meet the cooldown requirement.
type ErrCooldownNotMet struct {
	Ref         string
	PublishedAt time.Time
	Cooldown    time.Duration
}

func (e *ErrCooldownNotMet) Error() string {
	return "release does not meet cooldown requirement"
}

// IsCooldownNotMet returns true if the error is an ErrCooldownNotMet.
func IsCooldownNotMet(err error) bool {
	_, ok := err.(*ErrCooldownNotMet)
	return ok
}

