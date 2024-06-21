package resolver

import (
	"context"
	"regexp"
	"testing"
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
		tc := tc

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
