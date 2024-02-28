package resolver

import (
	"context"
	"fmt"
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

func TestUpgrade(t *testing.T) {
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
			exp:  "actions/checkout@v4",
		},
		{
			name: "tag-name-change",
			in:   "github/codeql-action/init@v1",
			exp:  "github/codeql-action/init@codeql-bundle-v2",
		},
		{
			name: "tag-name-change-and-minor-precision",
			in:   "github/codeql-action/init@v1.0",
			exp:  "github/codeql-action/init@codeql-bundle-v2.16",
		},
		{
			name: "tag-name-change-and-patch-precision",
			in:   "github/codeql-action/init@v1.0.1",
			exp:  "github/codeql-action/init@codeql-bundle-v2.16.3",
		},
		{
			name: "main-branch",
			in:   "github/codeql-action/init@main",
			exp:  "github/codeql-action/init@main",
		},
		{
			name: "master-branch",
			in:   "github/codeql-action/init@master",
			exp:  "github/codeql-action/init@master",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := resolver.Upgrade(ctx, tc.in)
			if err != nil {
				t.Fatal(err)
			}

			if result != tc.exp {
				t.Fatal(fmt.Errorf("upgrade failed - expected %s to match %s", result, tc.exp))
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
