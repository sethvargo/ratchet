package parser

import (
	"fmt"

	"github.com/sethvargo/ratchet/resolver"
	"gopkg.in/yaml.v3"
)

type GitLabCI struct{}

// Parse pulls the image references from GitLab CI configuration files.
// It does not support references with variables.

func (C *GitLabCI) Parse(m *yaml.Node) (*RefsList, error) {
	var refs RefsList
	var imageRef *yaml.Node

	// GitLab CI global top level keywords
	var globalKeywords = map[string]struct{}{
		"default":   {},
		"include":   {},
		"stages":    {},
		"variables": {},
		"workflow":  {},
	}

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
		// jobs names
		for i, keysMap := range docMap.Content {

			// exclude global keywords
			if _, hit := globalKeywords[keysMap.Value] ; hit || (keysMap.Value == "") {
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
				}
			}
		}
	}

	return &refs, nil
}
