package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

type Tekton struct{}

// DenormalizeRef changes the resolved ref into a ref that the parser expects.
func (t *Tekton) DenormalizeRef(ref string) string {
	return resolver.DenormalizeRef(ref)
}

// Parse pulls the Tekton Ci refs from the documents.
func (t *Tekton) Parse(nodes map[string]*yaml.Node) (*RefsList, error) {
	var refs RefsList
	for pth, node := range nodes {
		if err := t.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", pth, err)
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

func (d *Tekton) findSpecs(refs *RefsList, node *yaml.Node) {
	for i, specsMap := range node.Content {
		if specsMap.Value == "spec" {
			specs := node.Content[i+1]
			d.findImages(refs, specs)
		}
	}
}

func (d *Tekton) findImages(refs *RefsList, node *yaml.Node) {
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
}
