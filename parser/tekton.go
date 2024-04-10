package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

type Tekton struct{}

// Parse pulls the Tekton Ci refs from the documents.
func (d *Tekton) Parse(nodes []*yaml.Node) (*RefsList, error) {
	var refs RefsList
	for i, node := range nodes {
		if err := d.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse node %d: %w", i, err)
		}
	}

	return &refs, nil
}

func (d *Tekton) parseOne(refs *RefsList, node *yaml.Node) error {
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

		// Confirm it's a tekton file then proceed to look for image keyword
		for _, stepsMap := range docMap.Content {
			if stepsMap.Value == "apiVersion" {
				d.findSpecs(refs, docMap)
				break
			}
		}
	}

	return nil
}
func (d *Tekton) findSpecs(refs *RefsList, node *yaml.Node) error {
	for i, specsMap := range node.Content {
		if specsMap.Value == "spec" {
			specs := node.Content[i+1]
			d.findImages(refs, specs)
		}
	}
	return nil
}

func (d *Tekton) findImages(refs *RefsList, node *yaml.Node) error {
	for i, property := range node.Content {
		if property.Value == "image" {
			image := node.Content[i+1]
			ref := resolver.NormalizeContainerRef(image.Value)
			refs.Add(ref, image)
			break
		} else {
			d.findImages(refs, property)
		}
	}
	return nil
}
