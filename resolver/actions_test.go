package resolver

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestActions_Resolve(t *testing.T) {
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

func TestActions_LatestVersion(t *testing.T) {
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

func TestParseActionRef(t *testing.T) {
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

func TestGitHubRef_Name(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ref  *GitHubRef
		exp  string
	}{
		{
			name: "simple",
			ref: &GitHubRef{
				owner: "actions",
				repo:  "checkout",
				path:  "",
				ref:   "v3",
			},
			exp: "actions/checkout",
		},
		{
			name: "with_path",
			ref: &GitHubRef{
				owner: "github",
				repo:  "codeql-action",
				path:  "init",
				ref:   "v1",
			},
			exp: "github/codeql-action/init",
		},
		{
			name: "nested_path",
			ref: &GitHubRef{
				owner: "owner",
				repo:  "repo",
				path:  "path/to/action",
				ref:   "v2",
			},
			exp: "owner/repo/path/to/action",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got, want := tc.ref.Name(), tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}

func TestFormatVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		originalRef string
		tagName     string
		exp         string
	}{
		{
			name:        "major_only",
			originalRef: "v3",
			tagName:     "v4.2.1",
			exp:         "v4",
		},
		{
			name:        "major_minor",
			originalRef: "v3.1",
			tagName:     "v4.2.1",
			exp:         "v4.2",
		},
		{
			name:        "major_minor_patch",
			originalRef: "v3.1.0",
			tagName:     "v4.2.1",
			exp:         "v4.2.1",
		},
		{
			name:        "non_v_prefix",
			originalRef: "main",
			tagName:     "v4.2.1",
			exp:         "v4.2.1",
		},
		{
			name:        "tag_shorter_than_ref",
			originalRef: "v3.1.0",
			tagName:     "v4.2",
			exp:         "v4.2",
		},
		{
			name:        "codeql_style_tag",
			originalRef: "v1",
			tagName:     "codeql-bundle-v2.19.4",
			exp:         "codeql-bundle-v2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got, want := formatVersion(tc.originalRef, tc.tagName), tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}

func TestErrCooldownNotMet(t *testing.T) {
	t.Parallel()

	err := &ErrCooldownNotMet{
		Ref:         "actions/checkout@v4",
		PublishedAt: time.Now(),
		Cooldown:    3 * 24 * time.Hour,
	}

	if got, want := err.Error(), "release does not meet cooldown requirement"; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}

	if !IsCooldownNotMet(err) {
		t.Error("expected IsCooldownNotMet to return true")
	}

	if IsCooldownNotMet(nil) {
		t.Error("expected IsCooldownNotMet to return false for nil")
	}

	otherErr := context.DeadlineExceeded
	if IsCooldownNotMet(otherErr) {
		t.Error("expected IsCooldownNotMet to return false for other error type")
	}
}
