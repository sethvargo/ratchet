package parser

import (
	"reflect"
	"testing"

	"github.com/braydonk/yaml"
)

func TestTekton_Parse(t *testing.T) {
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
			exp: []string{},
		},
		{
			name: "steps",
			in: `
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
   name: testData
spec:
    params:
    - name: username
      type: string
    steps:
      - name: git
        image: alpine/git
`,
			exp: []string{
				"container://alpine/git",
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := helperStringToYAML(t, tc.in)

			refs, err := new(Tekton).Parse([]*yaml.Node{m})
			if err != nil {
				t.Fatal(err)
			}

			if got, want := refs.Refs(), tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
