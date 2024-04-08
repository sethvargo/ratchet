package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

type GitLabCI struct{}

// Parse pulls the image references from GitLab CI configuration files. It does
// not support references with variables.
func (c *GitLabCI) Parse(nodes []*yaml.Node) (*RefsList, error) {
	var refs RefsList

	for i, node := range nodes {
		if err := c.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse node %d: %w", i, err)
		}
	}

	return &refs, nil
}

func (c *GitLabCI) parseOne(refs *RefsList, m *yaml.Node) error {
	var imageRef *yaml.Node

	// GitLab CI global top level keywords
	globalKeywords := map[string]struct{}{
		"default":   {},
		"include":   {},
		"stages":    {},
		"variables": {},
		"workflow":  {},
	}

	if m == nil {
		return nil
	}

	if m.Kind != yaml.DocumentNode {
		return fmt.Errorf("expected document node, got %v", m.Kind)
	}

	// Top-level object map
	for _, docMap := range m.Content {
		if docMap.Kind != yaml.MappingNode {
			continue
		}
		// jobs names
		for i, keysMap := range docMap.Content {
			// exclude global keywords
			if _, hit := globalKeywords[keysMap.Value]; hit || (keysMap.Value == "") {
				continue
			}

			job := docMap.Content[i+1]
			if job.Kind != yaml.MappingNode {
				continue
			}

			for k, property := range job.Content {
				if property.Value == "image" {
					image := job.Content[k+1]

					// match image reference with name key
					if image.Kind == yaml.MappingNode {
						for j, nameRef := range image.Content {
							if nameRef.Value == "name" {
								imageRef = image.Content[j+1]
								break
							}
						}
					} else {
						imageRef = image
					}

					ref := resolver.NormalizeContainerRef(imageRef.Value)
					refs.Add(ref, imageRef)
				} else if property.Value == "services" {
					node := job.Content[k+1]
					for _, service := range node.Content {
						if service.Kind == yaml.MappingNode {
							for j, nameRef := range service.Content {
								if nameRef.Value == "name" {
									imageRef = service.Content[j+1]
									break
								}
							}
						} else {
							imageRef = service
						}
						ref := resolver.NormalizeContainerRef(imageRef.Value)
						refs.Add(ref, imageRef)
					}
				}
			}
		}
	}

	return nil
}
