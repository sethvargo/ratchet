package resolver

import (
	"testing"
	"time"

	"github.com/google/go-github/v73/github"
)

func TestPolicy_enabled(t *testing.T) {
	t.Parallel()

	if (Policy{}).enabled() {
		t.Fatal("zero policy should be disabled")
	}
	p := Policy{MinReleaseAge: time.Hour}
	if !p.enabled() {
		t.Fatal("non-zero min age should be enabled")
	}
}

func TestMinReleaseAgeFromEnv(t *testing.T) {
	t.Setenv("RATCHET_MIN_RELEASE_AGE", "24h")
	d, err := MinReleaseAgeFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if d != 24*time.Hour {
		t.Fatalf("expected 24h, got %v", d)
	}

	t.Setenv("RATCHET_MIN_RELEASE_AGE", "")
	d, err = MinReleaseAgeFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if d != 0 {
		t.Fatalf("expected 0, got %v", d)
	}

	t.Setenv("RATCHET_MIN_RELEASE_AGE", "not-a-duration")
	if _, err := MinReleaseAgeFromEnv(); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func Test_commitOlderThan(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	new := now.Add(-1 * time.Hour)

	oldCommit := &github.RepositoryCommit{
		Commit: &github.Commit{
			Committer: &github.CommitAuthor{Date: &github.Timestamp{Time: old}},
		},
	}
	newCommit := &github.RepositoryCommit{
		Commit: &github.Commit{
			Committer: &github.CommitAuthor{Date: &github.Timestamp{Time: new}},
		},
	}

	if !commitOlderThan(oldCommit, 24*time.Hour, now) {
		t.Fatal("expected old commit to pass")
	}
	if commitOlderThan(newCommit, 24*time.Hour, now) {
		t.Fatal("expected new commit to fail")
	}
}

func Test_selectEligibleRelease(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	releases := []*github.RepositoryRelease{
		{
			TagName:     github.Ptr("v4.2.0"),
			PublishedAt: &github.Timestamp{Time: now.Add(-1 * time.Hour)},
		},
		{
			TagName:     github.Ptr("v4.1.0"),
			PublishedAt: &github.Timestamp{Time: now.Add(-48 * time.Hour)},
		},
	}

	tag, ok := selectEligibleRelease(releases, 24*time.Hour, "v4", now)
	if !ok {
		t.Fatal("expected eligible release")
	}
	if tag != "v4" {
		t.Fatalf("expected v4, got %q", tag)
	}

	_, ok = selectEligibleRelease(releases[:1], 24*time.Hour, "v4", now)
	if ok {
		t.Fatal("expected no eligible release when all are too new")
	}
}

func Test_trimReleaseVersion(t *testing.T) {
	t.Parallel()

	if got := trimReleaseVersion("v4.2.1", "v4"); got != "v4" {
		t.Fatalf("expected v4, got %q", got)
	}
	if got := trimReleaseVersion("v4.2.1", "v4.2"); got != "v4.2" {
		t.Fatalf("expected v4.2, got %q", got)
	}
	if got := trimReleaseVersion("codeql-bundle-v2.15.0", "v1"); got != "codeql-bundle-v2" {
		t.Fatalf("expected codeql-bundle-v2, got %q", got)
	}
}
