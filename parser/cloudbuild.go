package parser

import (
	"fmt"

	"github.com/sethvargo/ratchet/resolver"
	"gopkg.in/yaml.v3"
)

type CloudBuild struct{}

// Parse pulls the Google Cloud Buidl refs from the document.
func (c *CloudBuild) Parse(m *yaml.Node) (*RefsList, error) {
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

		// jobs: keyword
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

	return &refs, nil
}
