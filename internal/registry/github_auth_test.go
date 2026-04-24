package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// withGitHubTokenLookup overrides the token lookup for the duration of a test.
func withGitHubTokenLookup(t *testing.T, fn func() (string, bool)) {
	t.Helper()
	origLookup := githubTokenLookup
	origToken, origOK, origSet := cachedGitHubToken, cachedGitHubTokenOK, cachedGitHubTokenSet
	githubTokenLookup = fn
	// Reset the cache so the override is picked up.
	githubTokenOnce = sync.Once{}
	cachedGitHubToken, cachedGitHubTokenOK, cachedGitHubTokenSet = "", false, false
	t.Cleanup(func() {
		githubTokenLookup = origLookup
		githubTokenOnce = sync.Once{}
		cachedGitHubToken, cachedGitHubTokenOK, cachedGitHubTokenSet = origToken, origOK, origSet
	})
}

func TestGetReleaseNotes_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"body": "notes"})
	}))
	defer srv.Close()

	origAPI := githubAPIBaseURL
	githubAPIBaseURL = srv.URL
	defer func() { githubAPIBaseURL = origAPI }()

	withGitHubTokenLookup(t, func() (string, bool) { return "secret-token", true })

	body, err := GetReleaseNotes("hashicorp", "aws", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "notes" {
		t.Errorf("body: got %q, want %q", body, "notes")
	}
	if want := "Bearer secret-token"; gotAuth != want {
		t.Errorf("Authorization header: got %q, want %q", gotAuth, want)
	}
}

func TestGetReleaseNotes_NoAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"body": "notes"})
	}))
	defer srv.Close()

	origAPI := githubAPIBaseURL
	githubAPIBaseURL = srv.URL
	defer func() { githubAPIBaseURL = origAPI }()

	withGitHubTokenLookup(t, func() (string, bool) { return "", false })

	if _, err := GetReleaseNotes("hashicorp", "aws", "1.0.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization header should be empty, got %q", gotAuth)
	}
}

func TestResolveGitHubToken_EnvVars(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "from-github-token")
	t.Setenv("GH_TOKEN", "from-gh-token")
	tok, ok := resolveGitHubToken()
	if !ok || tok != "from-github-token" {
		t.Errorf("GITHUB_TOKEN should win: got (%q, %v)", tok, ok)
	}

	t.Setenv("GITHUB_TOKEN", "")
	tok, ok = resolveGitHubToken()
	if !ok || tok != "from-gh-token" {
		t.Errorf("GH_TOKEN fallback: got (%q, %v)", tok, ok)
	}
}

// (no extra exports needed)
