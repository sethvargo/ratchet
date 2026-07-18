package resolver

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestContainer_Resolve(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resolver, err := NewContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{
			name: "default",
			in:   "alpine:3",
			exp:  "index.docker.io/library/alpine@sha256:[0-9a-f]{64}",
		},
		{
			name: "sha",
			in:   "alpine@sha256:dabf91b69c191a1a0a1628fd6bdd029c0c4018041c7f052870bb13c5a222ae76",
			exp:  "alpine@sha256:dabf91b69c191a1a0a1628fd6bdd029c0c4018041c7f052870bb13c5a222ae76",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := resolver.Resolve(ctx, tc.in)
			if err != nil {
				t.Fatal(err)
			}

			match, err := regexp.MatchString(tc.exp, result)
			if err != nil {
				t.Fatal(err)
			}

			if !match {
				t.Errorf("expected %q to match %q", result, tc.exp)
			}
		})
	}
}

func Test_imageCreatedTimestamp(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		created time.Time
		wantOK  bool
	}{
		{
			name:    "valid",
			created: created,
			wantOK:  true,
		},
		{
			name:    "zero",
			created: time.Time{},
		},
		{
			name:    "epoch",
			created: time.Unix(0, 0).UTC(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := imageCreatedTimestamp(tc.created)
			if !tc.wantOK {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %v, got nil", tc.created)
			}
			if !got.Time.Equal(tc.created) {
				t.Errorf("expected %v, got %v", tc.created, got.Time)
			}
			if got.Source != "image" {
				t.Errorf("expected source %q, got %q", "image", got.Source)
			}
		})
	}
}
