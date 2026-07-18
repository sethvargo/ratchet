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
		return "", fmt.Errorf("failed to lookup container ref: %w", err)
	}
	if value == ref.Name() {
		return fmt.Sprintf("%s@%s", ref.Name(), resp.Digest.String()), nil
	}

	return fmt.Sprintf("%s@%s", ref.Context().Name(), resp.Digest.String()), nil
}

// ResolvedTimestamp reports the image's config "created" time. That value is set
// by the image builder and is not server-authoritative. Reproducible builds
// zero it out (or set it to the Unix epoch), which is reported as nil so the
// caller degrades gracefully.
func (g *Container) ResolvedTimestamp(ctx context.Context, value string) (*Timestamp, error) {
	ref, err := name.ParseReference(value)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Container ref: %w", err)
	}

	img, err := remote.Image(ref,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch container image: %w", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to read container image config: %w", err)
	}

	return imageCreatedTimestamp(cfg.Created.Time), nil
}

// imageCreatedTimestamp wraps an image "created" time, treating the zero value
// and the Unix epoch (both used by reproducible builds) as unknown.
func imageCreatedTimestamp(created time.Time) *Timestamp {
	if created.IsZero() || created.Unix() <= 0 {
		return nil
	}
	return newTimestamp(created, "image")
}
