package resolver

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v73/github"
)

// Policy configures resolution behavior. Zero value disables optional checks.
type Policy struct {
	MinReleaseAge time.Duration // 0 = off
}

func (p Policy) enabled() bool {
	return p.MinReleaseAge > 0
}

// MinReleaseAgeFromEnv reads RATCHET_MIN_RELEASE_AGE (e.g. "24h", "1440m").
// Returns 0 if unset.
func MinReleaseAgeFromEnv() (time.Duration, error) {
	v := os.Getenv("RATCHET_MIN_RELEASE_AGE")
	if v == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid RATCHET_MIN_RELEASE_AGE %q: %w", v, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid RATCHET_MIN_RELEASE_AGE %q: must be non-negative", v)
	}
	return d, nil
}

func commitDate(commit *github.RepositoryCommit) time.Time {
	if commit == nil {
		return time.Time{}
	}
	c := commit.GetCommit()
	if c == nil {
		return time.Time{}
	}
	if committer := c.GetCommitter(); committer != nil {
		if d := committer.GetDate(); !d.Time.IsZero() {
			return d.Time
		}
	}
	if author := c.GetAuthor(); author != nil {
		if d := author.GetDate(); !d.Time.IsZero() {
			return d.Time
		}
	}
	return time.Time{}
}

// commitOlderThan reports whether the commit is at least minAge old relative to now.
func commitOlderThan(commit *github.RepositoryCommit, minAge time.Duration, now time.Time) bool {
	t := commitDate(commit)
	if t.IsZero() {
		return true
	}
	return now.Sub(t) >= minAge
}

func trimReleaseVersion(version, ref string) string {
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

// selectEligibleRelease returns the tag name of the newest release that satisfies
// minAge and semver trimming for ref.
func selectEligibleRelease(releases []*github.RepositoryRelease, minAge time.Duration, ref string, now time.Time) (string, bool) {
	for _, rel := range releases {
		if rel.GetDraft() || rel.GetPrerelease() {
			continue
		}
		published := rel.GetPublishedAt()
		if published.Time.IsZero() {
			continue
		}
		if now.Sub(published.Time) < minAge {
			continue
		}
		tag := rel.GetTagName()
		if tag == "" {
			continue
		}
		return trimReleaseVersion(tag, ref), true
	}
	return "", false
}
