package resolver

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v89/github"
	"golang.org/x/oauth2"
)

var (
	ActionsBaseURL   = os.Getenv("ACTIONS_BASE_URL")
	ActionsToken     = cmp.Or(os.Getenv("ACTIONS_TOKEN"), os.Getenv("GITHUB_TOKEN"))
	ActionsUploadURL = os.Getenv("ACTIONS_UPLOAD_URL")

	actionVersionRegex         = regexp.MustCompile(`^v\d+(\.\d+)*$`)
	embeddedActionVersionRegex = regexp.MustCompile(`(^|[^A-Za-z0-9])(v\d+(\.\d+)*)([^A-Za-z0-9]|$)`)
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

	opts := []github.ClientOptionsFunc{
		github.WithHTTPClient(httpClient),
	}
	if ActionsBaseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(ActionsBaseURL, ActionsUploadURL))
	}

	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create github client: %w", err)
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

	release, _, err := g.client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get latest release: %w", err)
	}

	name := owner + "/" + repo
	if path != "" {
		name = name + "/" + path
	}
	version := versionWithPrecision(release.GetTagName(), ref)

	if strings.HasPrefix(ref, "v") && !isActionVersionTag(version) {
		version, err = g.selectActionVersion(ctx, owner, repo, version, ref)
		if err != nil {
			return "", err
		}
	}

	result := fmt.Sprintf("%s@%s", name, version)
	return result, nil
}

func (g *Actions) selectActionVersion(ctx context.Context, owner, repo, releaseVersion, ref string) (string, error) {
	ok, err := g.refExists(ctx, owner, repo, releaseVersion)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release ref %s: %w", releaseVersion, err)
	}

	shouldFallback := !ok
	majorMismatch := mismatchedActionMajor(releaseVersion, ref)
	fallbackVersion := ""
	if shouldFallback || majorMismatch {
		tags, err := g.listVersionTags(ctx, owner, repo)
		if err != nil {
			return "", fmt.Errorf("failed to list tags: %w", err)
		}
		fallbackVersion = highestVersionTag(tags)
	}

	if ok && majorMismatch {
		if compareActionVersions(releaseVersion, ref) <= 0 && compareActionVersions(fallbackVersion, ref) <= 0 {
			return ref, nil
		}
		shouldFallback = compareActionVersions(fallbackVersion, releaseVersion) > 0
	}
	if !shouldFallback {
		return releaseVersion, nil
	}
	if fallbackVersion == "" {
		// No tags match the reference format - do not upgrade.
		return ref, nil
	}

	trimmed := versionWithPrecision(fallbackVersion, ref)
	if trimmed == fallbackVersion {
		return fallbackVersion, nil
	}
	ok, err = g.refExists(ctx, owner, repo, trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to fetch fallback ref %s: %w", trimmed, err)
	}
	if ok {
		return trimmed, nil
	}
	return fallbackVersion, nil
}

func versionWithPrecision(version, ref string) string {
	if !strings.HasPrefix(ref, "v") {
		return version
	}

	refPrecision := strings.Count(ref, ".")
	for strings.Count(version, ".") < refPrecision {
		version += ".0"
	}
	versionParts := strings.Split(version, ".")
	return strings.Join(versionParts[:refPrecision+1], ".")
}

func (g *Actions) refExists(ctx context.Context, owner, repo, ref string) (bool, error) {
	_, resp, err := g.client.Git.GetRef(ctx, owner, repo, "tags/"+ref)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

// listVersionTags returns all "v"-prefixed tag names, following pagination.
func (g *Actions) listVersionTags(ctx context.Context, owner, repo string) ([]string, error) {
	var tags []string
	for page := 1; ; {
		// v89's typed ListMatchingRefs no longer accepts pagination options and
		// only returns the first page, so issue the request directly to page
		// through all matching tags.
		u := fmt.Sprintf("repos/%s/%s/git/matching-refs/tags/v?per_page=100&page=%d", owner, repo, page)
		req, err := g.client.NewRequest(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}

		var refs []*github.Reference
		resp, err := g.client.Do(req, &refs)
		if err != nil {
			return nil, err
		}

		for _, r := range refs {
			tags = append(tags, strings.TrimPrefix(r.GetRef(), "refs/tags/"))
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return tags, nil
}

// highestVersionTag returns the highest action-style version tag.
func highestVersionTag(tags []string) string {
	best := ""
	var bestParts []int
	for _, tag := range tags {
		if !isActionVersionTag(tag) {
			continue
		}
		parts := versionParts(tag)
		cmp := compareVersionParts(parts, bestParts)
		if best == "" || cmp > 0 || (cmp == 0 && len(parts) > len(bestParts)) {
			best = tag
			bestParts = parts
		}
	}
	return best
}

func isActionVersionTag(tag string) bool {
	return actionVersionRegex.MatchString(tag)
}

func mismatchedActionMajor(version, ref string) bool {
	versionMajor := actionVersionMajor(version)
	refMajor := actionVersionMajor(ref)
	return versionMajor != "" && refMajor != "" && versionMajor != refMajor
}

func actionVersionMajor(version string) string {
	parts := versionParts(embeddedActionVersion(version))
	if len(parts) > 0 {
		return fmt.Sprintf("v%d", parts[0])
	}
	return ""
}

func embeddedActionVersion(version string) string {
	match := embeddedActionVersionRegex.FindStringSubmatch(version)
	if match == nil {
		return ""
	}
	return match[2]
}

func compareActionVersions(a, b string) int {
	return compareVersionParts(versionParts(embeddedActionVersion(a)), versionParts(embeddedActionVersion(b)))
}

func versionParts(tag string) []int {
	segments := strings.Split(strings.TrimPrefix(tag, "v"), ".")
	parts := make([]int, 0, len(segments))
	for _, s := range segments {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		parts = append(parts, i)
	}
	return parts
}

func compareVersionParts(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		switch {
		case av > bv:
			return 1
		case av < bv:
			return -1
		}
	}
	return 0
}
