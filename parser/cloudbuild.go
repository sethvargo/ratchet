package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

type CloudBuild struct{}

// DenormalizeRef changes the resolved ref into a ref that the parser expects.
func (c *CloudBuild) DenormalizeRef(ref string) string {
	return resolver.DenormalizeRef(ref)
}

// Parse pulls the Google Cloud Build refs from the documents.
func (c *CloudBuild) Parse(nodes map[string]*yaml.Node) (*RefsList, error) {
	var refs RefsList

	for pth, node := range nodes {
		if err := c.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", pth, err)
		}
	}

	return &refs, nil
}

func (c *CloudBuild) parseOne(refs *RefsList, node *yaml.Node) error {
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

		// steps: keyword
		for i, stepsMap := range docMap.Content {
			if stepsMap.Value != "steps" {
				continue
			}

			// Individual step arrays
			steps := docMap.Content[i+1]
			if steps.Kind != yaml.SequenceNode {
				continue
			}

			for _, step := range steps.Content {
				if step.Kind != yaml.MappingNode {
					continue
				}

				for j, property := range step.Content {
					if property.Value == "name" {
						name := step.Content[j+1]
						ref := resolver.NormalizeContainerRef(name.Value)
						refs.Add(ref, name)
						break
					}
				}
			}
		}
	}

	return nil
}
