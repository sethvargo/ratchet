package resolver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v73/github"
)

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

func TestActions_LatestVersion_latestReleaseRefExistsDoesNotFallback(t *testing.T) {
	t.Parallel()

	tagListCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/heads/v3", func(w http.ResponseWriter, r *http.Request) {
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

	client, err := github.NewClient(nil).WithEnterpriseURLs(srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &Actions{client: client}

	got, err := resolver.LatestVersion(context.Background(), "github/codeql-action/init@v3")
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

	client, err := github.NewClient(nil).WithEnterpriseURLs(srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &Actions{client: client}

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

func TestActions_LatestVersion_latestReleaseRefNon404DoesNotFallback(t *testing.T) {
	t.Parallel()

	tagListCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/github/codeql-action/git/ref/heads/v3", func(w http.ResponseWriter, r *http.Request) {
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

	client, err := github.NewClient(nil).WithEnterpriseURLs(srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &Actions{client: client}

	_, err = resolver.LatestVersion(context.Background(), "github/codeql-action/init@v3")
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

	client, err := github.NewClient(nil).WithEnterpriseURLs(srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &Actions{client: client}

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
