package parser

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/braydonk/yaml"
)

func TestGitLabCI_Parse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		exp  []string
	}{
		{
			name: "no_image_reference",
			in: `
stages:
  - plan
  - destroy

workflow:
  rules:
    - if: $CI_COMMIT_TAG
    - if: $CI_COMMIT_BRANCH

variables:
  VAR1: example
`,
			exp: nil,
		},
		{
			name: "wrong_image_reference",
			in: `
test_job:
  stage: lint
  variables:
    SCAN_DIR: .
  image: $CI_REGISTRY/image:tag
`,
			exp: []string{
				"container://$CI_REGISTRY/image:tag",
			},
		},
		{
			name: "multiline_image_ref",
			in: `
test_job:
  stage: test
  variables:
    SCAN_DIR: .
  image:
    name: alpine:3.15.0
    entrypoint: [""]
  script:
    - printenv
`,
			exp: []string{
				"container://alpine:3.15.0",
			},
		},
		{
			name: "job_with_include",
			in: `
.test:base:
  stage: test
  image: python
  retry:
    max: 1
  variables:
    VAR1: true
  script:
    - test command

job:
  extends:
    - .test:base
  image: node:12
  stage: test
  script:
    - test command

job2:
  image: gcr.io/project/image:tag
  stage: test
  script:
    - test command
`,
			exp: []string{
				"container://gcr.io/project/image:tag",
				"container://node:12",
				"container://python",
			},
		},
		{
			name: "jobs_with_services",
			in: `
.test:base:
  stage: test
  image: python
  retry:
    max: 1
  variables:
    VAR1: true
  script:
    - test command

job:
  extends:
    - .test:base
  image: node:12
  services:
    - postgres:14.3
    - docker:24.0.5-dind
    - name: selenium/standalone-firefox:latest
      alias: firefox
  stage: test
  script:
    - test command

job2:
  image: gcr.io/project/image:tag
  stage: test
  script:
    - test command
`,
			exp: []string{
				"container://docker:24.0.5-dind",
				"container://gcr.io/project/image:tag",
				"container://node:12",
				"container://postgres:14.3",
				"container://python",
				"container://selenium/standalone-firefox:latest",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nodes := map[string]*yaml.Node{
				"test.yml": helperStringToYAML(t, tc.in),
			}

			refs, err := new(GitLabCI).Parse(nodes)
			if err != nil {
				fmt.Println(refs)
				t.Fatal(err)
			}

			if got, want := refs.Refs(), tc.exp; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
