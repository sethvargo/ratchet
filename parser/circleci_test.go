package parser

import (
	"reflect"
	"testing"

	"github.com/braydonk/yaml"
)

func TestCircleCI_Parse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		exp  []string
	}{
		{
			name: "mostly_empty_file",
			in: `
executors:
`,
			exp: []string{},
		},
		{
			name: "executor",
			in: `
executors:
  my-executor:
    docker:
      - image: 'docker://ubuntu:20.04'
`,
			exp: []string{
				"container://ubuntu:20.04",
			},
		},
		{
			name: "job",
			in: `
jobs:
  my-job:
    docker:
      - image: 'ubuntu:20.04'
      - image: 'ubuntu:22.04'
`,
			exp: []string{
				"container://ubuntu:20.04",
				"container://ubuntu:22.04",
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := helperStringToYAML(t, tc.in)

			refs, err := new(CircleCI).Parse([]*yaml.Node{m})
			if err != nil {
				t.Fatal(err)
			}

			if got, want := refs.Refs(), tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
