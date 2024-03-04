package resolver

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resolver, err := NewActions(ctx)
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
			in:   "actions/checkout@v3",
			exp:  `actions\/checkout@[0-9a-f]{40}`,
		},
		{
			name: "path",
			in:   "github/codeql-action/init@v1",
			exp:  `github\/codeql-action\/init@[0-9a-f]{40}`,
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

func TestLatestVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resolver, err := NewActions(ctx)
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
			in:   "actions/checkout@v3",
			exp:  `actions/checkout@v[0-9]+`,
		},
		{
			name: "tag-name-change",
			in:   "github/codeql-action/init@v1",
			exp:  `github/codeql-action/init@codeql-bundle-v[0-9]+`,
		},
		{
			name: "tag-name-change-and-minor-precision",
			in:   "github/codeql-action/init@v1.0",
			exp:  `github/codeql-action/init@codeql-bundle-v[0-9]+\.[0-9]+`,
		},
		{
			name: "tag-name-change-and-patch-precision",
			in:   "github/codeql-action/init@v1.0.1",
			exp:  `github/codeql-action/init@codeql-bundle-v[0-9]+\.[0-9]+\.[0-9]+`,
		},
		{
			name: "skips-default-branch",
			in:   "github/codeql-action/init@main",
			exp:  `github/codeql-action/init@main`,
		},
		{
			name: "skips-branch",
			in:   "github/codeql-action/init@releases/v2",
			exp:  `github/codeql-action/init@releases/v2`,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := resolver.LatestVersion(ctx, tc.in)
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

func TestParseRef(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		exp  *GitHubRef
		err  string
	}{
		{
			name: "empty",
			in:   "",
			err:  "missing owner/repo",
		},
		{
			name: "no_slash",
			in:   "foo_bar_baz@v0",
			err:  "missing owner/repo",
		},
		{
			name: "no_ref",
			in:   "foo/bar",
			err:  "missing @",
		},
		{
			name: "ref",
			in:   "foo/bar@v0",
			exp: &GitHubRef{
				owner: "foo",
				repo:  "bar",
				path:  "",
				ref:   "v0",
			},
		},
		{
			name: "ref_path",
			in:   "foo/bar/baz@v0",
			exp: &GitHubRef{
				owner: "foo",
				repo:  "bar",
				path:  "baz",
				ref:   "v0",
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ref, err := ParseActionRef(tc.in)
			if err != nil {
				if tc.err == "" {
					t.Fatal(err)
				} else {
					if str := err.Error(); !strings.Contains(str, tc.err) {
						t.Errorf("expected %q to contain %q", str, tc.err)
					}
				}
			} else if tc.err != "" {
				t.Fatalf("expected error, but got %#v", ref)
			}

			if got, want := ref, tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %#v to be %#v", got, want)
			}
		})
	}
}
