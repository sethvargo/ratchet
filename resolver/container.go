package resolver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func NormalizeContainerRef(in string) string {
	in = strings.TrimSpace(in)
	in = strings.TrimPrefix(in, "docker://")
	return ContainerProtocol + in
}

// Container resolves Container registry references.
type Container struct {
	client *http.Client
}

// NewContainer creates a new resolver for Container registries.
func NewContainer(ctx context.Context) (*Container, error) {
	return &Container{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (g *Container) Resolve(ctx context.Context, value string) (string, error) {
	ref, err := name.ParseReference(value)
	if err != nil {
		return "", fmt.Errorf("failed to parse Container ref: %w", err)
	}

	resp, err := remote.Head(ref,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", fmt.Errorf("failed to lookup container ref %q: %w", ref, err)
	}

	return fmt.Sprintf("%s@%s", ref.Context().Name(), resp.Digest.String()), nil
}
