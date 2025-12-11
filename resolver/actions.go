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
	return g.ResolveWithOptions(ctx, value, nil)
}

func (g *Actions) ResolveWithOptions(ctx context.Context, value string, opts *ResolverOptions) (string, error) {
	if opts == nil {
		opts = DefaultResolverOptions()
	}

	githubRef, err := ParseActionRef(value)
	if err != nil {
		return "", fmt.Errorf("failed to parse github ref: %w", err)
	}
	owner := githubRef.owner
	repo := githubRef.repo
	path := githubRef.path
	ref := githubRef.ref

	// If cooldown is set, check if the tag/ref meets the cooldown requirement.
	// We do this by fetching the commit the ref points to and checking its date.
	if opts.Cooldown > 0 {
		commit, _, err := g.client.Repositories.GetCommit(ctx, owner, repo, ref, nil)
		if err != nil {
			return "", fmt.Errorf("failed to get commit for cooldown check: %w", err)
		}

		var commitDate time.Time
		if commit.Commit != nil && commit.Commit.Committer != nil && commit.Commit.Committer.Date != nil {
			commitDate = commit.Commit.Committer.Date.Time
		}

		if !commitDate.IsZero() {
			age := time.Since(commitDate)
			if age < opts.Cooldown {
				return "", &ErrCooldownNotMet{
					Ref:         value,
					PublishedAt: commitDate,
					Cooldown:    opts.Cooldown,
				}
			}
		}
	}

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

func (g *Actions) LatestVersion(ctx context.Context, value string) (string, error) {
	return g.LatestVersionWithOptions(ctx, value, nil)
}

func (g *Actions) LatestVersionWithOptions(ctx context.Context, value string, opts *ResolverOptions) (string, error) {
	if opts == nil {
		opts = DefaultResolverOptions()
	}

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

	// If cooldown is set, we need to find a release that meets the cooldown requirement.
	if opts.Cooldown > 0 {
		return g.findReleaseWithCooldown(ctx, owner, repo, path, ref, opts.Cooldown)
	}

	release, _, err := g.client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %w", err)
	}

	name := owner + "/" + repo
	if path != "" {
		name = name + "/" + path
	}
	version := *release.TagName
	if strings.HasPrefix(ref, "v") {
		refPrecision := strings.Count(githubRef.ref, ".")
		versionParts := strings.Split(*release.TagName, ".")
		version = strings.Join(versionParts[:refPrecision+1], ".")
	}

	result := fmt.Sprintf("%s@%s", name, version)
	return result, nil
}

// findReleaseWithCooldown finds the most recent release that meets the cooldown requirement.
func (g *Actions) findReleaseWithCooldown(ctx context.Context, owner, repo, path, ref string, cooldown time.Duration) (string, error) {
	// List releases and find the most recent one that meets the cooldown requirement.
	releases, _, err := g.client.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{
		PerPage: 50, // Get recent releases to find one within cooldown
	})
	if err != nil {
		return "", fmt.Errorf("failed to list releases: %w", err)
	}

	now := time.Now()
	name := owner + "/" + repo
	if path != "" {
		name = name + "/" + path
	}

	for _, release := range releases {
		if release.Draft != nil && *release.Draft {
			continue
		}
		if release.Prerelease != nil && *release.Prerelease {
			continue
		}

		var publishedAt time.Time
		if release.PublishedAt != nil {
			publishedAt = release.PublishedAt.Time
		} else if release.CreatedAt != nil {
			publishedAt = release.CreatedAt.Time
		}

		// Check if this release meets the cooldown requirement
		if !publishedAt.IsZero() && now.Sub(publishedAt) < cooldown {
			continue
		}

		// This release meets the cooldown requirement
		version := *release.TagName
		if strings.HasPrefix(ref, "v") {
			refPrecision := strings.Count(ref, ".")
			versionParts := strings.Split(*release.TagName, ".")
			if len(versionParts) > refPrecision {
				version = strings.Join(versionParts[:refPrecision+1], ".")
			}
		}

		result := fmt.Sprintf("%s@%s", name, version)
		return result, nil
	}

	// No release meets the cooldown requirement
	return "", &ErrCooldownNotMet{
		Ref:      ref,
		Cooldown: cooldown,
	}
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
