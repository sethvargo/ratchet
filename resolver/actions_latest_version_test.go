package resolver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v89/github"
)

// testActionsClient returns a github client pointed at the given test server.
func testActionsClient(tb testing.TB, rawURL string) *github.Client {
	tb.Helper()

	client, err := github.NewClient(github.WithEnterpriseURLs(rawURL, rawURL))
	if err != nil {
		tb.Fatalf("failed to create github client: %v", err)
	}
	return client
}

func TestHighestVersionTag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []string
		exp  string
	}{
		{
			name: "empty",
			in:   nil,
			exp:  "",
		},
		{
			name: "no_matching",
			in:   []string{"codeql-bundle-v2.25.6", "vNext", "v1.2.3-alpha"},
			exp:  "",
		},
		{
			name: "ignores_prerelease_and_build_metadata",
			in:   []string{"v1.2.3-alpha", "v1.2.3+build"},
			exp:  "",
		},
		{
			name: "mixed",
			in:   []string{"v1", "v2.1.1", "v3", "v3.30.4", "vNext", "codeql-bundle-v2.25.6"},
			exp:  "v3.30.4",
		},
		{
			name: "numeric_not_lexical",
			in:   []string{"v9.9.9", "v10.0.0"},
			exp:  "v10.0.0",
		},
		{
			name: "precision_tiebreak",
			in:   []string{"v3", "v3.0", "v3.0.0"},
			exp:  "v3.0.0",
		},
		{
			name: "crosses_major",
			in:   []string{"v3", "v3.30.4", "v4.0.0"},
			exp:  "v4.0.0",
		},
		{
			name: "crosses_minor",
			in:   []string{"v3.30", "v3.30.4", "v3.31.0"},
			exp:  "v3.31.0",
		},
		{
			name: "loose_date_style",
			in:   []string{"v2024.06.30", "v2024.07.01"},
			exp:  "v2024.07.01",
		},
		{
			name: "loose_four_part",
			in:   []string{"v1.2.3", "v1.2.3.4"},
			exp:  "v1.2.3.4",
		},
		{
			name: "loose_four_part_zero",
			in:   []string{"v2.0.0", "v2.0.0.1"},
			exp:  "v2.0.0.1",
		},
		{
			name: "loose_leading_zero_major",
			in:   []string{"v01"},
			exp:  "v01",
		},
		{
			name: "loose_leading_zero_minor",
			in:   []string{"v1.02.3"},
			exp:  "v1.02.3",
		},
		{
			name: "loose_leading_zero_patch",
			in:   []string{"v1.2.03"},
			exp:  "v1.2.03",
		},
		{
			name: "semver_date_style",
			in:   []string{"v2024.7.1"},
			exp:  "v2024.7.1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := highestVersionTag(tc.in); got != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, got)
			}
		})
	}
}

func TestEmbeddedActionVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{
			name: "bare",
			in:   "v3.1.0",
			exp:  "v3.1.0",
		},
		{
			name: "prefixed",
			in:   "codeql-bundle-v2.25.6",
			exp:  "v2.25.6",
		},
		{
			name: "requires_starting_boundary",
			in:   "rev2-v3.1.0",
			exp:  "v3.1.0",
		},
		{
			name: "requires_ending_boundary",
			in:   "release-v1beta-v2.1.0",
			exp:  "v2.1.0",
		},
		{
			name: "no_version",
			in:   "release-2024",
			exp:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := embeddedActionVersion(tc.in); got != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, got)
			}
		})
	}
}

func TestActions_LatestVersion_latestReleaseRef404FallbackLooseNumericTag(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/example/date-action/git/ref/heads/v2023.06.01", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/example/date-action/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"release-2024"}`)
	})
	mux.HandleFunc("/api/v3/repos/example/date-action/git/ref/tags/release-2024.0.0", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/example/date-action/git/matching-refs/tags/v", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"ref":"refs/tags/v1.2.3.4","object":{"type":"commit","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
			{"ref":"refs/tags/v2024.06.30","object":{"type":"commit","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
			{"ref":"refs/tags/v2024.07.01","object":{"type":"commit","sha":"cccccccccccccccccccccccccccccccccccccccc"}}
		]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resolver := &Actions{client: testActionsClient(t, srv.URL)}

	got, err := resolver.LatestVersion(context.Background(), "example/date-action@v2023.06.01")
	if err != nil {
		t.Fatal(err)
	}
	if exp := "example/date-action@v2024.07.01"; got != exp {
		t.Errorf("expected %q, got %q", exp, got)
	}
}

func TestActions_LatestVersion_latestReleaseRefExistsWithMatchingMajorDoesNotFallback(t *testing.T) {
	t.Parallel()

	tagListCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/heads/v2", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"codeql-bundle-v2.25.6"}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/codeql-bundle-v2", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ref":"refs/tags/codeql-bundle-v2","object":{"type":"commit","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/matching-refs/tags/v", func(w http.ResponseWriter, r *http.Request) {
		tagListCalled = true
		fmt.Fprint(w, `[]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resolver := &Actions{client: testActionsClient(t, srv.URL)}

	got, err := resolver.LatestVersion(context.Background(), "github/codeql-action/init@v2")
	if err != nil {
		t.Fatal(err)
	}
	if exp := "github/codeql-action/init@codeql-bundle-v2"; got != exp {
		t.Errorf("expected %q, got %q", exp, got)
	}
	if tagListCalled {
		t.Fatal("expected tag list fallback not to be called")
	}
}

func TestActions_LatestVersion_latestReleaseRefIgnoresEmbeddedVersionWithoutBoundary(t *testing.T) {
	t.Parallel()

	tagListCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/example/nonstandard/git/ref/heads/v3.0.0", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/example/nonstandard/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"rev2-v3.1.0"}`)
	})
	mux.HandleFunc("/api/v3/repos/example/nonstandard/git/ref/tags/rev2-v3.1.0", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ref":"refs/tags/rev2-v3.1.0","object":{"type":"commit","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
	})
	mux.HandleFunc("/api/v3/repos/example/nonstandard/git/matching-refs/tags/v", func(w http.ResponseWriter, r *http.Request) {
		tagListCalled = true
		fmt.Fprint(w, `[]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resolver := &Actions{client: testActionsClient(t, srv.URL)}

	got, err := resolver.LatestVersion(context.Background(), "example/nonstandard@v3.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if exp := "example/nonstandard@rev2-v3.1.0"; got != exp {
		t.Errorf("expected %q, got %q", exp, got)
	}
	if tagListCalled {
		t.Fatal("expected tag list fallback not to be called")
	}
}

func TestActions_LatestVersion_latestReleaseRefKeepsValidPrefixedRelease(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		ref               string
		release           string
		fallback          string
		expectTagListCall bool
	}{
		{
			name:              "cross-major release newer than bare tag",
			ref:               "v1.0.0",
			release:           "release-v2.0.0",
			fallback:          "v1.5.0",
			expectTagListCall: true,
		},
		{
			name:              "version-like prefix without ending boundary",
			ref:               "v2.0.0",
			release:           "release-v1beta-v2.1.0",
			fallback:          "v2.0.0",
			expectTagListCall: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tagListCalled := false
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v3/repos/example/prefixed/git/ref/heads/"+tc.ref, func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			})
			mux.HandleFunc("/api/v3/repos/example/prefixed/releases/latest", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"tag_name":%q}`, tc.release)
			})
			mux.HandleFunc("/api/v3/repos/example/prefixed/git/ref/tags/"+tc.release, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"ref":"refs/tags/%s","object":{"type":"commit","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`, tc.release)
			})
			mux.HandleFunc("/api/v3/repos/example/prefixed/git/matching-refs/tags/v", func(w http.ResponseWriter, r *http.Request) {
				tagListCalled = true
				fmt.Fprintf(w, `[{"ref":"refs/tags/%s","object":{"type":"commit","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}]`, tc.fallback)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			resolver := &Actions{client: testActionsClient(t, srv.URL)}

			got, err := resolver.LatestVersion(context.Background(), "example/prefixed@"+tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			if exp := "example/prefixed@" + tc.release; got != exp {
				t.Errorf("expected %q, got %q", exp, got)
			}
			if tagListCalled != tc.expectTagListCall {
				t.Errorf("expected tag list called to be %t, got %t", tc.expectTagListCall, tagListCalled)
			}
		})
	}
}

func TestActions_LatestVersion_latestReleaseRefExistsMismatchedMajorFallback(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/heads/v3", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/heads/v3.28.0", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"codeql-bundle-v2.25.6"}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/codeql-bundle-v2", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ref":"refs/tags/codeql-bundle-v2","object":{"type":"commit","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/codeql-bundle-v2.25.6", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ref":"refs/tags/codeql-bundle-v2.25.6","object":{"type":"commit","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/v4", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ref":"refs/tags/v4","object":{"type":"commit","sha":"cccccccccccccccccccccccccccccccccccccccc"}}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/matching-refs/tags/v", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"ref":"refs/tags/v3.30.4","object":{"type":"commit","sha":"dddddddddddddddddddddddddddddddddddddddd"}},
			{"ref":"refs/tags/v4.0.0","object":{"type":"commit","sha":"ffffffffffffffffffffffffffffffffffffffff"}}
		]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resolver := &Actions{client: testActionsClient(t, srv.URL)}

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{
			name: "major",
			in:   "github/codeql-action/init@v3",
			exp:  "github/codeql-action/init@v4",
		},
		{
			name: "patch",
			in:   "github/codeql-action/init@v3.28.0",
			exp:  "github/codeql-action/init@v4.0.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolver.LatestVersion(context.Background(), tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, got)
			}
		})
	}
}

// TestActions_LatestVersion_latestReleaseRef404Fallback covers issue #137.
func TestActions_LatestVersion_latestReleaseRef404Fallback(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/heads/v3", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"codeql-bundle-v2.25.6"}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/codeql-bundle-v2", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/v4", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ref":"refs/tags/v4","object":{"type":"commit","sha":"9999999999999999999999999999999999999999"}}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/matching-refs/tags/v", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"ref":"refs/tags/v1","object":{"type":"commit","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
			{"ref":"refs/tags/v2.1.1","object":{"type":"commit","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
			{"ref":"refs/tags/v3","object":{"type":"commit","sha":"cccccccccccccccccccccccccccccccccccccccc"}},
			{"ref":"refs/tags/v3.30.4","object":{"type":"commit","sha":"dddddddddddddddddddddddddddddddddddddddd"}},
			{"ref":"refs/tags/v4.0.0","object":{"type":"commit","sha":"ffffffffffffffffffffffffffffffffffffffff"}},
			{"ref":"refs/tags/vNext","object":{"type":"commit","sha":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}}
		]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resolver := &Actions{client: testActionsClient(t, srv.URL)}

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{
			name: "init",
			in:   "github/codeql-action/init@v3",
			exp:  "github/codeql-action/init@v4",
		},
		{
			name: "analyze",
			in:   "github/codeql-action/analyze@v3",
			exp:  "github/codeql-action/analyze@v4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolver.LatestVersion(context.Background(), tc.in)
			if err != nil {
				t.Fatal(err)
			}

			// The chosen highest tag is trimmed back to the input precision.
			if got != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, got)
			}
		})
	}
}

func TestActions_LatestVersion_latestReleaseRef404FallbackKeepsConcreteTag(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/heads/v3", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"codeql-bundle-v2.25.6"}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/codeql-bundle-v2", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/v4", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/matching-refs/tags/v", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"ref":"refs/tags/v3.30.4","object":{"type":"commit","sha":"dddddddddddddddddddddddddddddddddddddddd"}},
			{"ref":"refs/tags/v4.0.0","object":{"type":"commit","sha":"ffffffffffffffffffffffffffffffffffffffff"}}
		]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resolver := &Actions{client: testActionsClient(t, srv.URL)}

	got, err := resolver.LatestVersion(context.Background(), "github/codeql-action/init@v3")
	if err != nil {
		t.Fatal(err)
	}
	if exp := "github/codeql-action/init@v4.0.0"; got != exp {
		t.Errorf("expected %q, got %q", exp, got)
	}
}

func TestActions_LatestVersion_latestReleaseRefNon404DoesNotFallback(t *testing.T) {
	t.Parallel()

	tagListCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/heads/v2", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"codeql-bundle-v2.25.6"}`)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/tags/codeql-bundle-v2", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"internal error"}`, http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/matching-refs/tags/v", func(w http.ResponseWriter, r *http.Request) {
		tagListCalled = true
		fmt.Fprint(w, `[]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resolver := &Actions{client: testActionsClient(t, srv.URL)}

	_, err := resolver.LatestVersion(context.Background(), "github/codeql-action/init@v2")
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "failed to fetch latest release ref codeql-bundle-v2"; !strings.Contains(got, want) {
		t.Errorf("expected %q to contain %q", got, want)
	}
	if tagListCalled {
		t.Fatal("expected tag list fallback not to be called")
	}
}

func TestActions_Resolve_CodeQLActionPathsShareRef(t *testing.T) {
	t.Parallel()

	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/github/codeql-action/commits/v3", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, sha)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resolver := &Actions{client: testActionsClient(t, srv.URL)}

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{
			name: "init",
			in:   "github/codeql-action/init@v3",
			exp:  "github/codeql-action/init@" + sha,
		},
		{
			name: "analyze",
			in:   "github/codeql-action/analyze@v3",
			exp:  "github/codeql-action/analyze@" + sha,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolver.Resolve(context.Background(), tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, got)
			}
		})
	}
}
