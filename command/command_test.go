package command

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/google/go-cmp/cmp"
)

const (
	yamlA = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
	  - uses: 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # ratchet:actions/checkout@v3

      - uses: 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # ratchet:actions/checkout@v3

      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@52a8396df1c40bde244947c887d2c5dfbd36e4ce' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlAChanges = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
	  - uses: 'actions/checkout@9239842384293848238sfsdf823e234234234sds' # ratchet:actions/checkout@v3
      - uses: 'actions/checkout@9239842384293848238sfsdf823e234234234sds' # ratchet:actions/checkout@v3
      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@sdfswdf23423423423423sdfsdfsdfsdfdsfsdf2' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlAChangesFormatted = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
	  - uses: 'actions/checkout@9239842384293848238sfsdf823e234234234sds' # ratchet:actions/checkout@v3

      - uses: 'actions/checkout@9239842384293848238sfsdf823e234234234sds' # ratchet:actions/checkout@v3

      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@sdfswdf23423423423423sdfsdfsdfsdfdsfsdf2' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlB = `
jobs:
  init:
    runs-on:  'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - name: 'Checkout'
        uses: 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # ratchet:actions/checkout@v3

      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@52a8396df1c40bde244947c887d2c5dfbd36e4ce' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlBChanges = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - name: 'Checkout'
        uses: 'actions/checkout@9239842384293848238sfsdf823e234234234sds' # ratchet:actions/checkout@v3
      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@sdfswdf23423423423423sdfsdfsdfsdfdsfsdf2' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlBChangesFormatted = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - name: 'Checkout'
        uses: 'actions/checkout@9239842384293848238sfsdf823e234234234sds' # ratchet:actions/checkout@v3

      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@sdfswdf23423423423423sdfsdfsdfsdfdsfsdf2' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlC = `
jobs:
  init:
    runs-on:    'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - name: 'Checkout'
        uses: 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # ratchet:actions/checkout@v3

      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@52a8396df1c40bde244947c887d2c5dfbd36e4ce' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlCChanges = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - name: 'Checkout'
        uses: 'actions/checkout@9239842384293848238sfsdf823e234234234sds' # ratchet:actions/checkout@v3
      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@sdfswdf23423423423423sdfsdfsdfsdfdsfsdf2' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlCChangesFormatted = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - id : 'print'
        runs: 'echo "hello"'
      - name: 'Checkout'
        uses: 'actions/checkout@9239842384293848238sfsdf823e234234234sds' # ratchet:actions/checkout@v3

      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@sdfswdf23423423423423sdfsdfsdfsdfdsfsdf2' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
`
	yamlD = `
jobs:
  init:
    runs-on:    'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - name: 'Checkout'
        uses: 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # ratchet:actions/checkout@v3

      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@52a8396df1c40bde244947c887d2c5dfbd36e4ce' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
          thing: |-
            this is my string
            it has many lines

            some of them even
            have new new lines
`
	yamlDChanges = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - name: 'Checkout'
        uses: 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # ratchet:actions/checkout@v3
      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@52a8396df1c40bde244947c887d2c5dfbd36e4ce' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
          thing: |-
            this is my string
            it has many lines

            some of them even
            have new new lines
`
	yamlDChangesFormatted = `
jobs:
  init:
    runs-on: 'ubuntu-latest'
    outputs:
      directories: '${{ steps.dirs.outputs.directories }}'
    steps:
      - name: 'Checkout'
        uses: 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # ratchet:actions/checkout@v3

      - name: 'Guardian Directories'
        id: 'dirs'
        uses: 'abcxyz/guardian/.github/actions/directories@52a8396df1c40bde244947c887d2c5dfbd36e4ce' # ratchet:abcxyz/guardian/.github/actions/directories@main
        with:
          directories: '${{ inputs.directories }}'
          thing: |-
            this is my string
            it has many lines

            some of them even
            have new new lines
`
)

func Test_removeNewLineChanges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		yamlBefore string
		yamlAfter  string
		want       string
	}{
		{
			name:       "yamlA_multiple_empty_lines",
			yamlBefore: yamlA,
			yamlAfter:  yamlAChanges,
			want:       yamlAChangesFormatted,
		},
		{
			name:       "yamlB_single_empty_line",
			yamlBefore: yamlB,
			yamlAfter:  yamlBChanges,
			want:       yamlBChangesFormatted,
		},
		{
			name:       "yamlC_long_unchanged_section",
			yamlBefore: yamlC,
			yamlAfter:  yamlCChanges,
			want:       yamlCChangesFormatted,
		},
		{
			name:       "yamlD_multiline_string",
			yamlBefore: yamlD,
			yamlAfter:  yamlDChanges,
			want:       yamlDChangesFormatted,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := removeNewLineChanges(tc.yamlBefore, tc.yamlAfter)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %s to be %s", got, tc.want)
			}
		})
	}
}

func Test_parseYAMLFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		yamlFilename string
		want         string
	}{
		{
			name:         "yamlA_multiple_empty_lines",
			yamlFilename: "testdata/github.yml",
			want: `jobs:
    my_job:
        runs-on: 'ubuntu-latest'
        container:
            image: 'ubuntu:20.04'
        services:
            nginx:
                image: 'nginx:1.21'
        steps:
            - uses: 'actions/checkout@v3'
            - uses: 'docker://ubuntu:20.04'
              with:
                uses: '/path/to/user.png'
                image: '/path/to/image.jpg'
            - runs: |-
                echo "Hello 😀"
    other_job:
        uses: 'my-org/my-repo/.github/workflows/my-workflow.yml@v0'
    final_job:
        uses: './local/path/to/action'
`,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			node, err := parseYAMLFile(rootPath(tc.yamlFilename))
			if err != nil {
				t.Errorf("parseYAMLFile() returned error: %v", err)
			}
			var buf bytes.Buffer
			err = yaml.NewEncoder(&buf).Encode(node)
			if err != nil {
				t.Errorf("failed to marshal yaml to string: %v", err)
			}
			got := buf.String()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("removeBindingFromPolicy() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func rootPath(filename string) (fn string) {
	_, fn, _, _ = runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(fn))
	return fmt.Sprintf("%s/%s", repoRoot, filename)
}
