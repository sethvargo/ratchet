package parser

import (
	"reflect"
	"testing"

	"github.com/braydonk/yaml"
)

func TestDrone_Parse(t *testing.T) {
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
  - name: git
    image: alpine/git

  - name: test
    image: mysql
`,
			exp: []string{
				"container://alpine/git",
				"container://mysql",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nodes := map[string]*yaml.Node{
				"test.yml": helperStringToYAML(t, tc.in),
			}

			refs, err := new(Drone).Parse(nodes)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := refs.Refs(), tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
