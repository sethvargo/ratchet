package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"

	"github.com/sethvargo/ratchet/resolver"
)

type Drone struct{}

// Parse pulls the Drone Ci refs from the document.
func (D *Drone) Parse(m *yaml.Node) (*RefsList, error) {
	var refs RefsList

	if m == nil {
		return nil, nil
	}

	if m.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("expected document node, got %v", m.Kind)
	}

	for _, docMap := range m.Content {

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

	return &refs, nil
}
