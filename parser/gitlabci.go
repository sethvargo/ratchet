package parser

import (
	"fmt"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

type GitLabCI struct{}

// DenormalizeRef changes the resolved ref into a ref that the parser expects.
func (c *GitLabCI) DenormalizeRef(ref string) string {
	return resolver.DenormalizeRef(ref)
}

// Parse pulls the image references from GitLab CI configuration files. It does
// not support references with variables.
func (c *GitLabCI) Parse(nodes map[string]*yaml.Node) (*RefsList, error) {
	var refs RefsList

	for pth, node := range nodes {
		if err := c.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", pth, err)
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
		for key, value := range mapPairs(docMap) {
			// exclude global keywords
			if _, hit := globalKeywords[key.Value]; hit || (key.Value == "") {
				continue
			}

			job := value
			if job.Kind != yaml.MappingNode {
				continue
			}

			for propKey, propValue := range mapPairs(job) {
				if propKey.Value == "image" {
					image := propValue

					// match image reference with name key
					if image.Kind == yaml.MappingNode {
						for nameKey, nameValue := range mapPairs(image) {
							if nameKey.Value == "name" {
								imageRef = nameValue
								break
							}
						}
					} else {
						imageRef = image
					}

					ref := resolver.NormalizeContainerRef(imageRef.Value)
					refs.Add(ref, imageRef)
				} else if propKey.Value == "services" {
					for _, service := range propValue.Content {
						if service.Kind == yaml.MappingNode {
							for nameKey, nameValue := range mapPairs(service) {
								if nameKey.Value == "name" {
									imageRef = nameValue
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
