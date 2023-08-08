package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"

	"github.com/sethvargo/ratchet/resolver"
)

type CircleCI struct{}

// Parse pulls the CircleCI refs from the document. Unfortunately it does not
// process "orbs" because there is no documented API for resolving orbs to an
// absolute version.
func (C *CircleCI) Parse(m *yaml.Node) (*RefsList, error) {
	var refs RefsList

	if m == nil {
		return nil, nil
	}

	if m.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("expected document node, got %v", m.Kind)
	}

	// Top-level object map
	for _, docMap := range m.Content {
		if docMap.Kind != yaml.MappingNode {
			continue
		}

		// jobs: and executors: keyword
		for i, jobsMap := range docMap.Content {
			if jobsMap.Value != "jobs" && jobsMap.Value != "executors" {
				continue
			}

			// Individual job names
			jobs := docMap.Content[i+1]
			if jobs.Kind != yaml.MappingNode {
				continue
			}

			for _, jobMap := range jobs.Content {
				if jobMap.Kind != yaml.MappingNode {
					continue
				}

				for j, sub := range jobMap.Content {
					// CI service container, should be resolved as a Docker reference.
					// This is a map, so the container value is nested a bit deeper.
					if sub.Value == "docker" {
						servicesMap := jobMap.Content[j+1]
						for _, subMap := range servicesMap.Content {
							if subMap.Kind != yaml.MappingNode {
								continue
							}

							for k, property := range subMap.Content {
								if property.Value == "image" {
									image := subMap.Content[k+1]
									ref := resolver.NormalizeContainerRef(image.Value)
									refs.Add(ref, image)
									break
								}
							}
						}
					}
				}
			}
		}
	}

	return &refs, nil
}
