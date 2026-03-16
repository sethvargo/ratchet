package command

import (
	"bytes"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/braydonk/yaml"
	"github.com/google/go-cmp/cmp"
)

func Test_loadYAMLFiles(t *testing.T) {
	t.Parallel()

	fsys := os.DirFS("../testdata")

	cases := map[string]string{
		"a.yml":                   "a.golden.yml",
		"b.yml":                   "b.golden.yml",
		"c.yml":                   "",
		"circleci.yml":            "",
		"cloudbuild.yml":          "",
		"docker.yml":              "",
		"drone.yml":               "",
		"github-crazy-indent.yml": "github.yml",
		"github-issue-80.yml":     "",
		"github.yml":              "",
		"gitlabci.yml":            "",
		"no-trailing-newline.yml": "no-trailing-newline.golden.yml",
		"tekton.yml":              "",

		// These files demonstrate the YAML marshaling bug from PR #125 where
		// comments get misplaced. Uncomment to see the failures:
		// "github-pr125.yml":        "",
		// "github-codeql-pr125.yml": "",
	}

	for input, expected := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			files, err := loadYAMLFiles(fsys, []string{input})
			if err != nil {
				t.Fatal(err)
			}

			got, err := files[input].marshalYAML()
			if err != nil {
				t.Fatal(err)
			}

			if expected == "" {
				expected = input
			}
			want, err := fsys.(fs.ReadFileFS).ReadFile(expected)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(string(want), got); diff != "" {
				t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_computeNewlineTargets_simple(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		before string
		after  string
		want   []int
	}{
		{
			name:   "empty",
			before: "",
			after:  "",
			want:   []int{},
		},
		{
			name:   "single_newline",
			before: "\n",
			after:  "\n",
			want:   []int{},
		},
		{
			name:   "leading_whitespace",
			before: "\n\nfoo",
			after:  "foo",
			want:   []int{0, 1},
		},
		{
			name:   "trailing_whitespace",
			before: "foo\nbar\n\n",
			after:  "foo\nbar",
			want:   []int{2, 3},
		},
		{
			name:   "interior_whitespace",
			before: "foo\n\nbar\n\n\nbaz",
			after:  "foo\nbar\nbaz",
			want:   []int{1, 3, 4},
		},
		{
			name:   "interior_whitespace_leading_lines",
			before: "foo\n\n  bar\n\n\nbaz",
			after:  "foo\nbar\nbaz",
			want:   []int{1, 3, 4},
		},
		{
			name:   "interior_whitespace_tailing_lines",
			before: "foo\n\nbar  \n\n\nbaz",
			after:  "foo\nbar\nbaz",
			want:   []int{1, 3, 4},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := computeNewlineTargets(tc.before, tc.after)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("unexpected diff (+got, -want):\n%s", diff)
			}
		})
	}
}

func Test_unmarshalMarshal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		yaml string
		want []int
	}{
		{
			name: "single",
			yaml: "\nAPPARENTLY_THIS_IS_VALID_YAML\n",
			want: []int{0},
		},
		{
			name: "multiline",
			yaml: `---
stages:
  - build
  - test

build-code-job:
  stage: build
  image:
    name: gcr.io/distroless/static-debian11:nonroot
    entrypoint: [""]
  script:
    - echo "Job 1"

test-code-job1:
  stage: test
  image: node:12
  script:
    - echo "Job 2"
`,
			want: []int{3, 11},
		},
		{
			name: "folded_block_scalar",
			yaml: `this:
  is: >-
    a multiline

    string that
    spans lines

  that:
    has: >-
      other multline
      folded scalars
`,
			want: []int{6},
		},
		{
			name: "literal_block_scalar",
			yaml: `this:
  is: |-
    a multiline

    string that
    spans lines

  that:
    has: |-
      other multline
      literal scalars
`,
			want: []int{6},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fsys := fstest.MapFS{
				"file.yml": &fstest.MapFile{
					Data: []byte(tc.yaml),
				},
			}

			r, err := loadYAMLFiles(fsys, []string{"file.yml"})
			if err != nil {
				t.Fatal(err)
			}

			f := r["file.yml"]
			if diff := cmp.Diff(f.newlines, tc.want); diff != "" {
				t.Errorf("unexpected newlines diff (+got, -want):\n%s", diff)
			}

			s, err := f.marshalYAML()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(s, tc.yaml); diff != "" {
				t.Errorf("unexpected render diff (+got, -want):\n%s", diff)
			}
		})
	}
}

// Test_applySurgicalReplacements_preservesFormatting tests that the surgical
// replacement approach preserves original YAML formatting (PR #125).
func Test_applySurgicalReplacements_preservesFormatting(t *testing.T) {
	t.Parallel()

	fsys := os.DirFS("../testdata")

	cases := []struct {
		name     string
		file     string
		modifyFn func(node *yaml.Node)
	}{
		{
			name: "github-pr125.yml",
			file: "github-pr125.yml",
			modifyFn: func(node *yaml.Node) {
				walkAndPin(node, "actions/checkout@v2", "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab", "# ratchet:actions/checkout@v2")
				walkAndPin(node, "actions/github-script@v6", "actions/github-script@60a0d83039c74a4aee543508d2ffcb1c3799cdea", "# ratchet:actions/github-script@v6")
			},
		},
		{
			name: "github-codeql-pr125.yml",
			file: "github-codeql-pr125.yml",
			modifyFn: func(node *yaml.Node) {
				walkAndPin(node, "actions/checkout@v5", "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683", "# ratchet:actions/checkout@v5")
				walkAndPin(node, "github/codeql-action/init@v3", "github/codeql-action/init@aa578102511db1f4524ed59b8cc2bae4f6e88195", "# ratchet:github/codeql-action/init@v3")
				walkAndPin(node, "github/codeql-action/autobuild@v3", "github/codeql-action/autobuild@aa578102511db1f4524ed59b8cc2bae4f6e88195", "# ratchet:github/codeql-action/autobuild@v3")
				walkAndPin(node, "github/codeql-action/analyze@v3", "github/codeql-action/analyze@aa578102511db1f4524ed59b8cc2bae4f6e88195", "# ratchet:github/codeql-action/analyze@v3")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original, err := fsys.(fs.ReadFileFS).ReadFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}

			var node yaml.Node
			dec := yaml.NewDecoder(bytes.NewReader(original))
			dec.SetScanBlockScalarAsLiteral(true)
			if err := dec.Decode(&node); err != nil {
				t.Fatal(err)
			}

			tc.modifyFn(&node)
			got := applySurgicalReplacements(string(original), &node)

			// Verify line count unchanged (surgical replacement shouldn't add/remove lines)
			originalLines := strings.Split(string(original), "\n")
			gotLines := strings.Split(got, "\n")
			if len(originalLines) != len(gotLines) {
				t.Errorf("line count changed: original=%d, got=%d", len(originalLines), len(gotLines))
			}

			// Verify non-uses lines are unchanged, uses lines have ratchet comments
			for i := 0; i < len(originalLines) && i < len(gotLines); i++ {
				if strings.Contains(originalLines[i], "uses:") {
					if !strings.Contains(gotLines[i], "ratchet:") {
						t.Errorf("line %d: expected ratchet comment, got %q", i+1, gotLines[i])
					}
					continue
				}
				if originalLines[i] != gotLines[i] {
					t.Errorf("line %d: unexpected change\n  original: %q\n  got:      %q", i+1, originalLines[i], gotLines[i])
				}
			}
		})
	}
}

// walkAndPin simulates what Pin does when finding a matching action reference.
func walkAndPin(node *yaml.Node, oldValue, newValue, comment string) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode && node.Value == oldValue {
		node.Value = newValue
		node.LineComment = comment
	}
	for _, child := range node.Content {
		walkAndPin(child, oldValue, newValue, comment)
	}
}
