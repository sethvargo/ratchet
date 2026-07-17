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
	for key, value := range mapPairs(node) {
		if key.Value == "spec" {
			d.findImages(refs, value)
		}
	}
}

func (d *Tekton) findImages(refs *RefsList, node *yaml.Node) {
	if node.Kind == yaml.MappingNode {
		for key, value := range mapPairs(node) {
			if key.Value == "image" {
				addContainerRef(refs, value)
				return
			}
			d.findImages(refs, value)
		}
		return
	}

	for _, child := range node.Content {
		d.findImages(refs, child)
	}
}
