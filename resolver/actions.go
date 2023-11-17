package resolver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v56/github"
	"golang.org/x/oauth2"
)

var (
	ActionsBaseURL   = os.Getenv("ACTIONS_BASE_URL")
	ActionsToken     = os.Getenv("ACTIONS_TOKEN")
	ActionsUploadURL = os.Getenv("ACTIONS_UPLOAD_URL")
)

func NormalizeActionsRef(in string) string {
	return ActionsProtocol + in
}

// Actions resolves GitHub references.
type Actions struct {
	client *github.Client
}

// NewActions creates a new resolver for GitHub Actions.
func NewActions(ctx context.Context) (*Actions, error) {
	httpClient := &http.Client{}
	if ActionsToken != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: ActionsToken})
		httpClient = oauth2.NewClient(ctx, ts)
	}
	httpClient.Timeout = 10 * time.Second

	client := github.NewClient(httpClient)
	if ActionsBaseURL != "" {
		var err error
		client, err = client.WithEnterpriseURLs(ActionsBaseURL, ActionsUploadURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create enterprise github client: %w", err)
		}
	}

	return &Actions{
		client: client,
	}, nil
}

func (g *Actions) Resolve(ctx context.Context, value string) (string, error) {
	githubRef, err := ParseActionRef(value)
	if err != nil {
		return "", fmt.Errorf("failed to parse github ref: %w", err)
	}
	owner := githubRef.owner
	repo := githubRef.repo
	path := githubRef.path
	ref := githubRef.ref

	sha, _, err := g.client.Repositories.GetCommitSHA1(ctx, owner, repo, ref, "")
	if err != nil {
		return "", fmt.Errorf("failed to get commit sha: %w", err)
	}

	name := owner + "/" + repo
	if path != "" {
		name = name + "/" + path
	}

	return fmt.Sprintf("%s@%s", name, sha), nil
}

func ParseActionRef(s string) (*GitHubRef, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("missing owner/repo in actions reference: %q", s)
	}
	owner, rest := parts[0], parts[1]

	smallerParts := strings.SplitN(rest, "@", 2)
	if len(smallerParts) < 2 {
		return nil, fmt.Errorf("missing @ in actions reference: %q", s)
	}
	ref := smallerParts[1]

	evenSmallerParts := strings.SplitN(smallerParts[0], "/", 2)
	repo := evenSmallerParts[0]

	var path string
	if len(evenSmallerParts) > 1 {
		path = evenSmallerParts[1]
	}

	return &GitHubRef{
		owner: owner,
		repo:  repo,
		path:  path,
		ref:   ref,
	}, nil
}

type GitHubRef struct {
	owner string
	repo  string
	path  string
	ref   string
}
