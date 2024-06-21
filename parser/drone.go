package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

type Drone struct{}

// DenormalizeRef changes the resolved ref into a ref that the parser expects.
func (d *Drone) DenormalizeRef(ref string) string {
	return resolver.DenormalizeRef(ref)
}

// Parse pulls the Drone Ci refs from the documents.
func (d *Drone) Parse(nodes []*yaml.Node) (*RefsList, error) {
	var refs RefsList

	for i, node := range nodes {
		if err := d.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse node %d: %w", i, err)
		}
	}

	return &refs, nil
}

func (d *Drone) parseOne(refs *RefsList, node *yaml.Node) error {
	if node == nil {
		return nil
	}

	if node.Kind != yaml.DocumentNode {
		return fmt.Errorf("expected document node, got %v", node.Kind)
	}

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
					if property.Value == "image" {
						image := step.Content[j+1]
						ref := resolver.NormalizeContainerRef(image.Value)
						refs.Add(ref, image)
						break
					}
				}
			}
		}
	}

	return nil
}
