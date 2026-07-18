package resolver

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
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
			exp:  `github/codeql-action/init@v[0-9]+`,
		},
		{
			name: "tag-name-change-and-minor-precision",
			in:   "github/codeql-action/init@v1.0",
			exp:  `github/codeql-action/init@v[0-9]+\.[0-9]+`,
		},
		{
			name: "tag-name-change-and-patch-precision",
			in:   "github/codeql-action/init@v1.0.1",
			exp:  `github/codeql-action/init@v[0-9]+\.[0-9]+\.[0-9]+`,
		},
		{
			name: "floating-tag-with-patch-precision",
			in:   "google-github-actions/auth@v3.0.0",
			exp:  `google-github-actions/auth@v[0-9]+\.[0-9]+\.[0-9]+`,
		},
		{
			name: "floating-tag-with-minor-precision",
			in:   "actions/github-script@v8.0",
			exp:  `actions/github-script@v[0-9]+\.[0-9]+`,
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

func Test_commitTimestamp(t *testing.T) {
	t.Parallel()

	committer := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	author := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		commit *github.RepositoryCommit
		want   time.Time
		wantOK bool
	}{
		{
			name: "prefers_committer",
			commit: &github.RepositoryCommit{
				Commit: &github.Commit{
					Committer: &github.CommitAuthor{Date: &github.Timestamp{Time: committer}},
					Author:    &github.CommitAuthor{Date: &github.Timestamp{Time: author}},
				},
			},
			want:   committer,
			wantOK: true,
		},
		{
			name: "falls_back_to_author",
			commit: &github.RepositoryCommit{
				Commit: &github.Commit{
					Author: &github.CommitAuthor{Date: &github.Timestamp{Time: author}},
				},
			},
			want:   author,
			wantOK: true,
		},
		{
			name:   "no_dates",
			commit: &github.RepositoryCommit{Commit: &github.Commit{}},
		},
		{
			name:   "no_commit",
			commit: &github.RepositoryCommit{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := commitTimestamp(tc.commit)
			if !tc.wantOK {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %v, got nil", tc.want)
			}
			if !got.Time.Equal(tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got.Time)
			}
			if got.Source != "commit" {
				t.Errorf("expected source %q, got %q", "commit", got.Source)
			}
		})
	}
}
