package resolver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v73/github"
	"golang.org/x/oauth2"
)

var (
	ActionsBaseURL   = os.Getenv("ACTIONS_BASE_URL")
	ActionsToken     = coalesce(os.Getenv("ACTIONS_TOKEN"), os.Getenv("GITHUB_TOKEN"))
	ActionsUploadURL = os.Getenv("ACTIONS_UPLOAD_URL")
)

func NormalizeActionsRef(in string) string {
	return ActionsProtocol + in
}

// Actions resolves GitHub references.
type Actions struct {
	client *github.Client
	policy Policy
}

// NewActions creates a new resolver for GitHub Actions.
func NewActions(ctx context.Context, policy Policy) (*Actions, error) {
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
		policy: policy,
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

	if g.policy.enabled() {
		commit, _, err := g.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
		if err != nil {
			return "", fmt.Errorf("failed to get commit: %w", err)
		}
		now := time.Now()
		if !commitOlderThan(commit, g.policy.MinReleaseAge, now) {
			commitTime := commitDate(commit)
			ago := now.Sub(commitTime).Round(time.Minute)
			return "", fmt.Errorf(
				"%s/%s@%s points to commit %s (%s ago), younger than min-release-age %s; wait or use -min-release-age 0",
				owner, repo, ref, sha[:7], ago, g.policy.MinReleaseAge,
			)
		}
	}

	name := owner + "/" + repo
	if path != "" {
		name = name + "/" + path
	}

	return fmt.Sprintf("%s@%s", name, sha), nil
}

func (g *Actions) LatestVersion(ctx context.Context, value string) (string, error) {
	githubRef, err := ParseActionRef(value)
	if err != nil {
		return "", fmt.Errorf("failed to parse github ref: %w", err)
	}
	owner := githubRef.owner
	repo := githubRef.repo
	path := githubRef.path
	ref := githubRef.ref
	branchRef := "heads/" + ref

	// Fetching the Git Ref allows us to determine if the ref is for a branch
	// or tag. We must explicitly format for either `tags/` or `heads/`
	// (branches). We arbitrarily check if the ref is for a branch, therefore
	// we expect 404s for Tag references.
	fullRef, resp, err := g.client.Git.GetRef(ctx, owner, repo, branchRef)
	if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
		return "", fmt.Errorf("failed to fetch ref %s: %w", ref, err)
	}

	// Do not upgrade branch refs.
	if fullRef != nil {
		return value, nil
	}

	name := owner + "/" + repo
	if path != "" {
		name = name + "/" + path
	}

	var version string
	if g.policy.enabled() {
		releases, err := g.listReleases(ctx, owner, repo)
		if err != nil {
			return "", err
		}
		now := time.Now()
		var ok bool
		version, ok = selectEligibleRelease(releases, g.policy.MinReleaseAge, ref, now)
		if !ok {
			return "", fmt.Errorf(
				"no release for %s/%s published at least %s ago matching %q",
				owner, repo, g.policy.MinReleaseAge, ref,
			)
		}
	} else {
		release, _, err := g.client.Repositories.GetLatestRelease(ctx, owner, repo)
		if err != nil {
			return "", fmt.Errorf("failed to get latest release: %w", err)
		}
		version = trimReleaseVersion(*release.TagName, ref)
	}

	result := fmt.Sprintf("%s@%s", name, version)
	return result, nil
}

func (g *Actions) listReleases(ctx context.Context, owner, repo string) ([]*github.RepositoryRelease, error) {
	opts := &github.ListOptions{PerPage: 100}
	var all []*github.RepositoryRelease
	for {
		releases, resp, err := g.client.Repositories.ListReleases(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list releases: %w", err)
		}
		all = append(all, releases...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
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

func coalesce(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
