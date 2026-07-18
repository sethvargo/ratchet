package resolver

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeTimestampResolver struct {
	ts  *Timestamp
	err error
}

func (f fakeTimestampResolver) ResolvedTimestamp(_ context.Context, _ string) (*Timestamp, error) {
	return f.ts, f.err
}

func Test_checkBakeDelay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC()
	young := &Timestamp{Time: now.Add(-1 * time.Hour), Source: "commit"}
	old := &Timestamp{Time: now.Add(-48 * time.Hour), Source: "release"}

	cases := []struct {
		name      string
		bakeDelay time.Duration
		res       any
		wantErr   bool
	}{
		{
			name:      "disabled_ignores_young_artifact",
			bakeDelay: 0,
			res:       fakeTimestampResolver{ts: young},
		},
		{
			name:      "negative_delay_ignores_young_artifact",
			bakeDelay: -1 * time.Hour,
			res:       fakeTimestampResolver{ts: young},
		},
		{
			name:      "young_artifact_fails",
			bakeDelay: 24 * time.Hour,
			res:       fakeTimestampResolver{ts: young},
			wantErr:   true,
		},
		{
			name:      "old_artifact_passes",
			bakeDelay: 24 * time.Hour,
			res:       fakeTimestampResolver{ts: old},
		},
		{
			name:      "unknown_age_degrades",
			bakeDelay: 24 * time.Hour,
			res:       fakeTimestampResolver{ts: nil},
		},
		{
			name:      "resolver_without_capability_degrades",
			bakeDelay: 24 * time.Hour,
			res:       struct{}{},
		},
		{
			name:      "resolver_error_propagates",
			bakeDelay: 24 * time.Hour,
			res:       fakeTimestampResolver{err: errors.New("boom")},
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := &DefaultResolver{bakeDelay: tc.bakeDelay}
			err := r.checkBakeDelay(ctx, tc.res, "actions/checkout@v4", "actions/checkout@abc123")
			if got := err != nil; got != tc.wantErr {
				t.Errorf("expected error=%v, got %v", tc.wantErr, err)
			}
		})
	}
}
