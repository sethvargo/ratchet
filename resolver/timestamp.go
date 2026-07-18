package resolver

import (
	"context"
	"time"
)

// Timestamp is the most trustworthy known publish or creation time for a
// resolved artifact, along with the source it was derived from (for example
// "release", "tag", "commit", or "image").
type Timestamp struct {
	Time   time.Time
	Source string
}

// newTimestamp builds a Timestamp with the time normalized to UTC so
// comparisons and formatting are timezone-independent.
func newTimestamp(t time.Time, source string) *Timestamp {
	return &Timestamp{Time: t.UTC(), Source: source}
}

// TimestampResolver is an optional capability that a Resolver may implement to
// report the age of a resolved artifact. It powers the bake delay. Resolvers
// that cannot determine a time return a nil Timestamp with a nil error, and the
// caller treats the artifact as passing the bake delay (graceful degradation).
type TimestampResolver interface {
	ResolvedTimestamp(ctx context.Context, ref string) (*Timestamp, error)
}
