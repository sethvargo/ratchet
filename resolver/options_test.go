package resolver

import (
	"testing"
	"time"
)

func TestDefaultResolverOptions(t *testing.T) {
	t.Parallel()

	opts := DefaultResolverOptions()

	if opts == nil {
		t.Fatal("expected non-nil options")
	}

	if opts.Cooldown != 0 {
		t.Errorf("expected default cooldown to be 0, got %v", opts.Cooldown)
	}
}

func TestResolverOptions_Cooldown(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cooldown time.Duration
	}{
		{
			name:     "zero",
			cooldown: 0,
		},
		{
			name:     "one_day",
			cooldown: 24 * time.Hour,
		},
		{
			name:     "three_days",
			cooldown: 3 * 24 * time.Hour,
		},
		{
			name:     "one_week",
			cooldown: 7 * 24 * time.Hour,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := &ResolverOptions{
				Cooldown: tc.cooldown,
			}

			if got, want := opts.Cooldown, tc.cooldown; got != want {
				t.Errorf("expected cooldown %v to be %v", got, want)
			}
		})
	}
}
