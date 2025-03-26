package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

type CircleCI struct{}

// DenormalizeRef changes the resolved ref into a ref that the parser expects.
func (c *CircleCI) DenormalizeRef(ref string) string {
	return resolver.DenormalizeRef(ref)
}

// Parse pulls the CircleCI refs from the documents. Unfortunately it does not
// process "orbs" because there is no documented API for resolving orbs to an
// absolute version.
func (c *CircleCI) Parse(nodes map[string]*yaml.Node) (*RefsList, error) {
	var refs RefsList

	for pth, node := range nodes {
		if err := c.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", pth, err)
		}
	}

	return &refs, nil
}

func (c *CircleCI) parseOne(refs *RefsList, node *yaml.Node) error {
	if node == nil {
		return nil
	}

	if node.Kind != yaml.DocumentNode {
		return fmt.Errorf("expected document node, got %v", node.Kind)
	}

	// Top-level object map
	for _, docMap := range node.Content {
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

	return nil
}
