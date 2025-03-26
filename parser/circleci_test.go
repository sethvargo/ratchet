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
			exp: nil,
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nodes := map[string]*yaml.Node{
				"test.yml": helperStringToYAML(t, tc.in),
			}

			refs, err := new(CircleCI).Parse(nodes)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := refs.Refs(), tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
