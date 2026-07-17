package parser

import (
	"reflect"
	"testing"

	"github.com/braydonk/yaml"
)

func TestActions_Parse(t *testing.T) {
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
			name: "uses",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'actions/checkout@v3'
      - uses: 'docker://ubuntu:20.04'
      - uses: 'docker://ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724'
        with:
          uses: 'foo/bar@v0'
  other_job:
    uses: './github/workflows/other.yml'
  final_job:
    uses: 'org/repo/.github/workflows/other@v0'
`,
			exp: []string{
				"actions://actions/checkout@v3",
				"actions://org/repo/.github/workflows/other@v0",
				"container://ubuntu:20.04",
				"container://ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724",
			},
		},
		{
			name: "container",
			in: `
jobs:
  my_job:
    container:
      image: 'ubuntu:20.04'
`,
			exp: []string{
				"container://ubuntu:20.04",
			},
		},
		{
			name: "services",
			in: `
jobs:
  my_job:
    services:
      nginx:
        image: 'nginx:1.21'
      ubuntu:
        image: 'ubuntu:20.04'
`,
			exp: []string{
				"container://nginx:1.21",
				"container://ubuntu:20.04",
			},
		},
		{
			name: "composite",
			in: `
runs:
  using: 'composite'
  steps:
    - uses: 'actions/checkout@v3'
    - uses: 'docker://ubuntu:20.04'
    - uses: 'docker://ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724'
      with:
        uses: 'foo/bar@v0'
`,
			exp: []string{
				"actions://actions/checkout@v3",
				"container://ubuntu:20.04",
				"container://ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724",
			},
		},
		{
			name: "parallel_steps",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'actions/checkout@v3'
      - parallel:
          - name: Set up Docker Buildx
            uses: 'docker/setup-buildx-action@v4'
          - name: Login to Registry
            uses: 'docker/login-action@v3'
      - uses: 'actions/upload-artifact@v4'
`,
			exp: []string{
				"actions://actions/checkout@v3",
				"actions://actions/upload-artifact@v4",
				"actions://docker/login-action@v3",
				"actions://docker/setup-buildx-action@v4",
			},
		},
		{
			name: "parallel_steps_composite",
			in: `
runs:
  using: 'composite'
  steps:
    - uses: 'actions/checkout@v3'
    - parallel:
        - uses: 'actions/setup-node@v4'
        - uses: 'actions/setup-python@v5'
`,
			exp: []string{
				"actions://actions/checkout@v3",
				"actions://actions/setup-node@v4",
				"actions://actions/setup-python@v5",
			},
		},
		{
			name: "parallel_mixed",
			in: `
jobs:
  my_job:
    steps:
      - parallel:
          - uses: 'actions/checkout@v3'
          - uses: 'docker://ubuntu:20.04'
      - uses: 'actions/upload-artifact@v4'
      - parallel:
          - uses: 'actions/cache@v4'
`,
			exp: []string{
				"actions://actions/cache@v4",
				"actions://actions/checkout@v3",
				"actions://actions/upload-artifact@v4",
				"container://ubuntu:20.04",
			},
		},
		{
			name: "parallel_with_background_and_wait",
			in: `
jobs:
  my_job:
    steps:
      - name: Start service
        uses: 'docker://redis:7'
        background: true
      - parallel:
          - uses: 'actions/checkout@v3'
          - uses: 'actions/setup-node@v4'
      - wait-all
      - uses: 'actions/upload-artifact@v4'
`,
			exp: []string{
				"actions://actions/checkout@v3",
				"actions://actions/setup-node@v4",
				"actions://actions/upload-artifact@v4",
				"container://redis:7",
			},
		},
		{
			name: "ignores_interpolated",
			in: `
jobs:
  my_job:
    container:
      image: 'ghcr.io/${{ github.repository }}/container:1.2.3'
    steps:
      - uses: 'actions/${{ github.sha }}'

`,
			exp: nil,
		},
		{
			name: "step_field_named_uses",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'actions/checkout@v3'
        name: 'uses'
`,
			exp: []string{
				"actions://actions/checkout@v3",
			},
		},
		{
			name: "job_field_named_steps",
			in: `
jobs:
  my_job:
    steps:
      - uses: 'actions/checkout@v4'
    name: 'steps'
`,
			exp: []string{
				"actions://actions/checkout@v4",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nodes := map[string]*yaml.Node{
				"test.yml": helperStringToYAML(t, tc.in),
			}

			refs, err := new(Actions).Parse(nodes)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := refs.Refs(), tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %#v to be %#v", got, want)
			}
		})
	}
}
