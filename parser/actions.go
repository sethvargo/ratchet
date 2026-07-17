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
func (a *Actions) Parse(nodes map[string]*yaml.Node) (*RefsList, error) {
	var refs RefsList

	for pth, node := range nodes {
		if err := a.parseOne(&refs, node); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", pth, err)
		}
	}

	return &refs, nil
}

func (a *Actions) processStep(refs *RefsList, step *yaml.Node) {
	if step.Kind != yaml.MappingNode {
		return
	}

	for key, value := range mapPairs(step) {
		if key.Value == "uses" {
			uses := value

			if strings.Contains(uses.Value, "${{") {
				continue
			}

			switch {
			case strings.HasPrefix(uses.Value, "docker://"):
				ref := resolver.NormalizeContainerRef(uses.Value)
				refs.Add(ref, uses)
			case strings.Contains(uses.Value, "@"):
				ref := resolver.NormalizeActionsRef(uses.Value)
				refs.Add(ref, uses)
			}
		}

		if key.Value == "parallel" {
			parallelSteps := value
			if parallelSteps.Kind == yaml.SequenceNode {
				for _, innerStep := range parallelSteps.Content {
					a.processStep(refs, innerStep)
				}
			}
		}
	}
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

		for topKey, topValue := range mapPairs(docMap) {
			// runs: keyword
			if topKey.Value == "runs" {
				runs := topValue
				if runs.Kind != yaml.MappingNode {
					continue
				}

				// Only look at composite actions.
				foundComposite := false
				for runKey, runValue := range mapPairs(runs) {
					if runKey.Value == "using" && runValue.Value == "composite" {
						foundComposite = true
						break
					}
				}
				if !foundComposite {
					continue
				}

				// List of steps, iterate over each step and find the "uses" clause.
				for runKey, runValue := range mapPairs(runs) {
					if runKey.Value == "steps" {
						steps := runValue
						for _, step := range steps.Content {
							a.processStep(refs, step)
						}
					}
				}
			}

			// jobs: keyword
			if topKey.Value == "jobs" {
				jobs := topValue
				if jobs.Kind != yaml.MappingNode {
					continue
				}

				for _, jobMap := range jobs.Content {
					if jobMap.Kind != yaml.MappingNode {
						continue
					}

					for subKey, subValue := range mapPairs(jobMap) {
						// Container reference for running the job, should be resolved as a
						// Docker reference.
						if subKey.Value == "container" {
							containerMap := subValue
							for propKey, propValue := range mapPairs(containerMap) {
								if propKey.Value == "image" {
									image := propValue

									// Ignore interpolations, since we cannot resolve most of
									// their values.
									if strings.Contains(image.Value, "${{") {
										continue
									}

									ref := resolver.NormalizeContainerRef(image.Value)
									refs.Add(ref, image)
									break
								}
							}
						}

						// CI service container, should be resolved as a Docker reference.
						// This is a map, so the container value is nested a bit deeper.
						if subKey.Value == "services" {
							servicesMap := subValue
							for _, subMap := range servicesMap.Content {
								if subMap.Kind != yaml.MappingNode {
									continue
								}

								for propKey, propValue := range mapPairs(subMap) {
									if propKey.Value == "image" {
										image := propValue

										// Ignore interpolations, since we cannot resolve most of
										// their values.
										if strings.Contains(image.Value, "${{") {
											continue
										}

										ref := resolver.NormalizeContainerRef(image.Value)
										refs.Add(ref, image)
										break
									}
								}
							}
						}

						// List of steps, iterate over each step and find the "uses" clause.
						if subKey.Value == "steps" {
							steps := subValue
							for _, step := range steps.Content {
								a.processStep(refs, step)
							}
						}

						// Top-level uses, likely for a reusable workflow.
						if subKey.Value == "uses" {
							uses := subValue

							// Ignore interpolations, since we cannot resolve most of
							// their values.
							if strings.Contains(uses.Value, "${{") {
								continue
							}

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
