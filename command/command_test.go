package command

import (
	"reflect"
	"testing"
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
