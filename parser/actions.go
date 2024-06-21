package parser

import (
	"fmt"
	"strings"

	// Using banydonk/yaml instead of the default yaml pkg because the default
	// pkg incorrectly escapes unicode. https://github.com/go-yaml/yaml/issues/737
	"github.com/braydonk/yaml"
	"github.com/sethvargo/ratchet/resolver"
)

type Actions struct{}

// DenormalizeRef changes the resolved ref into a ref that the parser expects.
func (a *Actions) DenormalizeRef(ref string) string {
	isContainer := strings.HasPrefix(ref, resolver.ContainerProtocol)
	ref = resolver.DenormalizeRef(ref)
	if isContainer {
		return "docker://" + ref
	}
	return ref
}

// Parse pulls the GitHub Actions refs from the documents.
func (a *Actions) Parse(nodes []*yaml.Node) (*RefsList, error) {
	var refs RefsList

	for i, node := range nodes {
		if err := a.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse node %d: %w", i, err)
		}
	}

	return &refs, nil
}

func (a *Actions) parseOne(refs *RefsList, node *yaml.Node) error {
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

		for i, topLevelMap := range docMap.Content {
			// runs: keyword
			if topLevelMap.Value == "runs" {
				runs := docMap.Content[i+1]
				if runs.Kind != yaml.MappingNode {
					continue
				}

				// Only look at composite actions.
				foundComposite := false
				for j, runMap := range runs.Content {
					if runMap.Value == "using" && len(runs.Content) > j+1 && runs.Content[j+1].Value == "composite" {
						foundComposite = true
						break
					}
				}
				if !foundComposite {
					continue
				}

				// List of steps, iterate over each step and find the "uses" clause.
				for j, runMap := range runs.Content {
					if runMap.Value == "steps" {
						steps := runs.Content[j+1]
						for _, step := range steps.Content {
							if step.Kind != yaml.MappingNode {
								continue
							}

							for k, property := range step.Content {
								if property.Value == "uses" {
									uses := step.Content[k+1]
									// Only include references to remote workflows. This could be
									// a local workflow, which should not be pinned.
									switch {
									case strings.HasPrefix(uses.Value, "docker://"):
										ref := resolver.NormalizeContainerRef(uses.Value)
										refs.Add(ref, uses)
									case strings.Contains(uses.Value, "@"):
										ref := resolver.NormalizeActionsRef(uses.Value)
										refs.Add(ref, uses)
									}
								}
							}
						}
					}
				}
			}

			// jobs: keyword
			if topLevelMap.Value == "jobs" {
				jobs := docMap.Content[i+1]
				if jobs.Kind != yaml.MappingNode {
					continue
				}

				for _, jobMap := range jobs.Content {
					if jobMap.Kind != yaml.MappingNode {
						continue
					}

					for j, sub := range jobMap.Content {
						// Container reference for running the job, should be resolved as a
						// Docker reference.
						if sub.Value == "container" {
							containerMap := jobMap.Content[j+1]
							for k, property := range containerMap.Content {
								if property.Value == "image" {
									image := containerMap.Content[k+1]
									ref := resolver.NormalizeContainerRef(image.Value)
									refs.Add(ref, image)
									break
								}
							}
						}

						// CI service container, should be resolved as a Docker reference.
						// This is a map, so the container value is nested a bit deeper.
						if sub.Value == "services" {
							servicesMap := jobMap.Content[j+1]
							for _, subMap := range servicesMap.Content {
								if subMap.Kind != yaml.MappingNode {
									continue
								}

								for k, property := range subMap.Content {
									if property.Value == "image" {
										image := subMap.Content[k+1]
										ref := resolver.NormalizeContainerRef(image.Value)
										refs.Add(ref, image)
										break
									}
								}
							}
						}

						// List of steps, iterate over each step and find the "uses" clause.
						if sub.Value == "steps" {
							steps := jobMap.Content[j+1]
							for _, step := range steps.Content {
								if step.Kind != yaml.MappingNode {
									continue
								}

								for k, property := range step.Content {
									if property.Value == "uses" {
										uses := step.Content[k+1]
										// Only include references to remote workflows. This could be
										// a local workflow, which should not be pinned.
										switch {
										case strings.HasPrefix(uses.Value, "docker://"):
											ref := resolver.NormalizeContainerRef(uses.Value)
											refs.Add(ref, uses)
										case strings.Contains(uses.Value, "@"):
											ref := resolver.NormalizeActionsRef(uses.Value)
											refs.Add(ref, uses)
										}
									}
								}
							}
						}

						// Top-level uses, likely for a reusable workflow.
						if sub.Value == "uses" {
							uses := jobMap.Content[j+1]

							// Only include references to remote workflows. This could be a
							// local workflow, which should not be pinned.
							switch {
							case strings.HasPrefix(uses.Value, "docker://"):
								ref := resolver.NormalizeContainerRef(uses.Value)
								refs.Add(ref, uses)
							case strings.Contains(uses.Value, "@"):
								ref := resolver.NormalizeActionsRef(uses.Value)
								refs.Add(ref, uses)
							}
						}
					}
				}
			}
		}
	}

	return nil
}
