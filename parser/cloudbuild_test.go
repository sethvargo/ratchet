package parser

import (
	"reflect"
	"testing"

	"github.com/braydonk/yaml"
)

func TestCloudBuild_Parse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		exp  []string
	}{
		{
			name: "mostly_empty_file",
			in: `
jobs:
`,
			exp: nil,
		},
		{
			name: "steps",
			in: `
steps:
  - name: 'ubuntu:20.04'
  - name: 'gcr.io/foo/bar/baz'
`,
			exp: []string{
				"container://gcr.io/foo/bar/baz",
				"container://ubuntu:20.04",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nodes := map[string]*yaml.Node{
				"test.yml": helperStringToYAML(t, tc.in),
			}

			refs, err := new(CloudBuild).Parse(nodes)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := refs.Refs(), tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
