package resolver

import (
	"reflect"
	"strings"
	"testing"
)

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
