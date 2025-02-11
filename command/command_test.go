package command

import (
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

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
		"github-subdirectory.yml": "",
		"github.yml":              "",
		"gitlabci.yml":            "",
		"no-trailing-newline.yml": "no-trailing-newline.golden.yml",
		"tekton.yml":              "",
	}

	for input, expected := range cases {
		inputFilename, expectedFilename := input, expected

		t.Run(inputFilename, func(t *testing.T) {
			t.Parallel()

			files, err := loadYAMLFiles(fsys, []string{inputFilename})
			if err != nil {
				t.Fatal(err)
			}

			got, err := files[0].marshalYAML()
			if err != nil {
				t.Fatal(err)
			}

			if expectedFilename == "" {
				expectedFilename = inputFilename
			}
			want, err := fsys.(fs.ReadFileFS).ReadFile(expectedFilename)
			if err != nil {
				t.Fatal(err)
			}

			if got != string(want) {
				t.Errorf("expected\n\n%s\n\nto be\n\n%s\n", got, want)
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
		tc := tc

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
		tc := tc

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

			f := r[0]
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
