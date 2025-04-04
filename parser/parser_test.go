package parser

import (
	"bytes"
	"context"
	"strings"
	"testing"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	par := new(Actions)

	cases := []struct {
		name string
		in   string
		err  string
	}{
		{
			name: "no_uses",
			in: `
foo: 'bar'
`,
		},
		{
			name: "good_uses",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@2541b1294d2704b0964813337f33b291d3f8596b'
`,
		},
		{
			name: "bad_uses",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v0'
`,
			err: `found 1 unpinned refs: ["good/repo@v0"]`,
		},
		{
			name: "exclude",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v0' # ratchet:exclude
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nodes := map[string]*yaml.Node{
				"test.yml": helperStringToYAML(t, tc.in),
			}

			if err := Check(ctx, par, nodes); err != nil {
				if tc.err == "" {
					t.Fatal(err)
				} else {
					if got, want := err.Error(), tc.err; !strings.Contains(got, want) {
						t.Errorf("expected %q to contain %q", got, want)
					}
				}
			} else if tc.err != "" {
				t.Fatal("expected error, got nothing")
			}
		})
	}
}

func TestPin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	res, err := resolver.NewTest(map[string]*resolver.TestResult{
		"actions://good/repo@v0": {
			Resolved: "good/repo@a12a3943",
		},
		"actions://good/repo@v1": {
			Resolved: "good/repo@b12a3943",
		},
		"actions://good/repo/sub/path@v0": {
			Resolved: "good/repo/sub/path@a12a3943",
		},
		"actions://good/repo/sub/path@v2": {
			Resolved: "good/repo/sub/path@b12a3943",
		},
		"actions://good/repo@2541b1294d2704b0964813337f33b291d3f8596b": {
			Resolved: "good/repo@2541b1294d2704b0964813337f33b291d3f8596b",
		},
		"container://ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724": {
			Resolved: "ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724",
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	par := new(Actions)

	cases := []struct {
		name string
		in   string
		exp  string
		err  string
	}{
		{
			name: "no_uses",
			in: `
foo: 'bar'
`,
			exp: `
foo: 'bar'
`,
		},
		{
			name: "good_uses",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v0'
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@a12a3943' # ratchet:good/repo@v0
`,
		},
		{
			name: "uses_subpath",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo/sub/path@v0'
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo/sub/path@a12a3943' # ratchet:good/repo/sub/path@v0
`,
		},
		{
			name: "existing_comment",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v0' # this is a comment
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@a12a3943' # this is a comment ratchet:good/repo@v0
`,
		},
		{
			name: "already_pinned",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@2541b1294d2704b0964813337f33b291d3f8596b' # ratchet:good/repo@v0
      - uses: 'docker://ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724' # ratchet:docker://ubuntu:20.04
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@2541b1294d2704b0964813337f33b291d3f8596b' # ratchet:good/repo@v0
      - uses: 'docker://ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724' # ratchet:docker://ubuntu:20.04
`,
		},
		{
			name: "exclude",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'should_not/resolve@v0' # ratchet:exclude
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'should_not/resolve@v0' # ratchet:exclude
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := helperStringToYAML(t, tc.in)

			nodes := map[string]*yaml.Node{
				"test.yml": m,
			}

			if err := Pin(ctx, res, par, nodes, 2); err != nil {
				if tc.err == "" {
					t.Fatal(err)
				} else {
					if got, want := err.Error(), tc.err; !strings.Contains(got, want) {
						t.Errorf("expected %q to contain %q", got, want)
					}
				}
			} else if tc.err != "" {
				t.Fatal("expected error, got nothing")
			}

			if tc.err == "" {
				if got, want := helperYAMLToString(t, m), strings.TrimSpace(tc.exp); got != want {
					t.Errorf("expected \n\n%s\n\nto be\n\n%s\n\n", got, want)
				}
			}
		})
	}
}

func TestUpgrade(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	res, err := resolver.NewTest(nil,
		map[string]*resolver.TestResult{
			"actions://good/repo@v0": {
				Resolved: "actions://good/repo@v2.1.0",
			},
			"actions://good/repo@v2.1.0": {
				Resolved: "actions://good/repo@v2.1.0",
			},
			"actions://good/repo@a12a3943": {
				Resolved: "actions://good/repo@v2.1.0",
			},
			"actions://good/repo/sub/path@a12a3943": {
				Resolved: "actions://good/repo/sub/path@v2.1.0",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	par := new(Actions)

	cases := []struct {
		name string
		in   string
		exp  string
		err  string
	}{
		{
			name: "no_uses",
			in: `
foo: 'bar'
`,
			exp: `
foo: 'bar'
`,
		},
		{
			name: "unpinned_old",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v0'
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v2.1.0' # ratchet:good/repo@v2.1.0
`,
		},
		{
			name: "unpinned_latest",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v2.1.0'
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v2.1.0' # ratchet:good/repo@v2.1.0
`,
		},
		{
			name: "pinned",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@a12a3943'
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v2.1.0' # ratchet:good/repo@v2.1.0
`,
		},
		{
			name: "subpath",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo/sub/path@a12a3943'
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo/sub/path@v2.1.0' # ratchet:good/repo/sub/path@v2.1.0
`,
		},
		{
			name: "existing_comment",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v0' # this is a comment
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v2.1.0' # this is a comment ratchet:good/repo@v2.1.0
		`,
		},
		{
			name: "exclude",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v0' # ratchet:exclude # this is a comment
`,
			exp: `
jobs:
  my_job:
    steps:
      - uses: 'good/repo@v0' # ratchet:exclude # this is a comment
		`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := helperStringToYAML(t, tc.in)

			nodes := map[string]*yaml.Node{
				"test.yml": m,
			}

			if err := Upgrade(ctx, res, par, nodes, 2); err != nil {
				if tc.err == "" {
					t.Fatal(err)
				} else {
					if got, want := err.Error(), tc.err; !strings.Contains(got, want) {
						t.Errorf("expected %q to contain %q", got, want)
					}
				}
			} else if tc.err != "" {
				t.Fatal("expected error, got nothing")
			}

			if tc.err == "" {
				if got, want := helperYAMLToString(t, m), strings.TrimSpace(tc.exp); got != want {
					t.Errorf("expected \n\n%s\n\nto be\n\n%s\n\n", got, want)
				}
			}
		})
	}
}

func TestUnpin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{
			name: "no_uses",
			in:   `foo: bar`,
			exp:  `foo: bar`,
		},
		{
			name: "uses_no_comment",
			in:   `uses: "my/repo@v0"`,
			exp:  `uses: "my/repo@v0"`,
		},
		{
			name: "uses_comment",
			in:   `uses: "my/repo@abcd1234" # ratchet:my/repo@v0 this is a code comment`,
			exp:  `uses: "my/repo@v0" # this is a code comment`,
		},
		{
			name: "multiple_uses",
			in: `
- uses: "my/repo@abcd1234" # ratchet:my/repo@v0 comment
- uses: "other/repo@efgh6789" # ratchet:other/repo@v1 yep
- uses: "i/am@pinned" # comment
`,
			exp: `
- uses: "my/repo@v0" # comment
- uses: "other/repo@v1" # yep
- uses: "i/am@pinned" # comment
`,
		},
		{
			name: "exclude_comment",
			in:   `uses: "my/repo@v0" # ratchet:exclude more comment`,
			exp:  `uses: "my/repo@v0" # ratchet:exclude more comment`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := helperStringToYAML(t, tc.in)

			nodes := map[string]*yaml.Node{
				"test.yml": m,
			}

			if err := Unpin(ctx, nodes); err != nil {
				t.Fatal(err)
			}

			if got, want := helperYAMLToString(t, m), strings.TrimSpace(tc.exp); got != want {
				t.Errorf("expected \n\n%s\n\nto be\n\n%s\n\n", got, want)
			}
		})
	}
}

func TestAppendOriginalToComment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{
			name: "empty_string",
			in:   "",
			exp:  "ratchet:foo/bar@v1",
		},
		{
			name: "single_character",
			in:   "a",
			exp:  "a ratchet:foo/bar@v1",
		},
		{
			name: "multi_character",
			in:   "this is a code comment",
			exp:  "this is a code comment ratchet:foo/bar@v1",
		},
		{
			name: "already_pinned",
			in:   "ratchet:zip/zap@v2",
			exp:  "ratchet:foo/bar@v1",
		},
		{
			name: "already_pinned_with_comment",
			in:   "ratchet:zip/zap@v2 this is a code comment",
			exp:  "this is a code comment ratchet:foo/bar@v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pin := "foo/bar@v1"
			if got, want := appendOriginalToComment(tc.in, pin), tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}

func TestExtractOriginalFromComment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      string
		extract string
		rest    string
	}{
		{
			name:    "empty_string",
			in:      "",
			extract: "",
			rest:    "",
		},
		{
			name:    "single_character",
			in:      "a",
			extract: "",
			rest:    "a",
		},
		{
			name:    "comment",
			in:      "this is a code comment",
			extract: "",
			rest:    "this is a code comment",
		},
		{
			name:    "prefix_no_value",
			in:      "ratchet:",
			extract: "",
			rest:    "",
		},
		{
			name:    "prefix_single_character",
			in:      "ratchet:a",
			extract: "a",
			rest:    "",
		},
		{
			name:    "prefix_single_character_comment",
			in:      "ratchet:a this is a code comment",
			extract: "a",
			rest:    "this is a code comment",
		},
		{
			name:    "prefix_long",
			in:      "ratchet:foo/bar@v3 this is a code comment",
			extract: "foo/bar@v3",
			rest:    "this is a code comment",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			extracted, rest := extractOriginalFromComment(tc.in)

			if got, want := extracted, tc.extract; got != want {
				t.Errorf("expected extracted %q to be %q", got, want)
			}

			if got, want := rest, tc.rest; got != want {
				t.Errorf("expected rest %q to be %q", got, want)
			}
		})
	}
}

func helperStringToYAML(tb testing.TB, in string) *yaml.Node {
	tb.Helper()

	dec := yaml.NewDecoder(strings.NewReader(strings.TrimSpace(in)))
	var m yaml.Node
	if err := dec.Decode(&m); err != nil {
		tb.Fatal(err)
	}
	return &m
}

func helperYAMLToString(tb testing.TB, m *yaml.Node) string {
	tb.Helper()

	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		tb.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		tb.Fatal(err)
	}

	return strings.TrimSpace(b.String())
}
