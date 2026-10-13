package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newCountingServer starts a test server that serves both the registry and
// GitHub endpoints used by the client, and counts requests per path.
func newCountingServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, map[string]*atomic.Int32) {
	t.Helper()
	counts := map[string]*atomic.Int32{}
	paths := []string{
		"/hashicorp/aws/versions",
		"/repos/hashicorp/terraform-provider-aws/releases",
		"/repos/hashicorp/terraform-provider-aws/releases/tags/v5.98.0",
		"/",
	}
	for _, p := range paths {
		counts[p] = &atomic.Int32{}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := counts[r.URL.Path]; ok {
			c.Add(1)
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	origBase, origV2, origGH := baseURL, v2BaseURL, githubAPIBaseURL
	baseURL, v2BaseURL, githubAPIBaseURL = srv.URL, srv.URL, srv.URL
	t.Cleanup(func() { baseURL, v2BaseURL, githubAPIBaseURL = origBase, origV2, origGH })

	ResetCache()
	t.Cleanup(ResetCache)
	return srv, counts
}

func TestGetVersionsIsCached(t *testing.T) {
	_, counts := newCountingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hashicorp/aws/versions":
			json.NewEncoder(w).Encode(ProviderResponse{Versions: []VersionInfo{{Version: "5.97.0"}, {Version: "5.98.0"}}})
		case "/repos/hashicorp/terraform-provider-aws/releases":
			json.NewEncoder(w).Encode([]map[string]string{{"tag_name": "v5.98.0", "published_at": "2025-01-02T00:00:00Z"}})
		default:
			http.NotFound(w, r)
		}
	})

	// Keys are case-insensitive: only the first call reaches the server.
	for _, name := range []string{"aws", "AWS", "Aws"} {
		versions, _, err := GetVersions("hashicorp", name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if versions[0].Version != "5.98.0" || versions[0].Published != "Jan 2, 2025" {
			t.Fatalf("unexpected first version: %+v", versions[0])
		}
		// Mutating the returned slice must not affect the cache.
		versions[0].Version = "mutated"
	}

	if got := counts["/hashicorp/aws/versions"].Load(); got != 1 {
		t.Errorf("registry versions requests: got %d, want 1", got)
	}
	if got := counts["/repos/hashicorp/terraform-provider-aws/releases"].Load(); got != 1 {
		t.Errorf("GitHub releases requests: got %d, want 1", got)
	}
}

func TestGetVersionsRetriesFailedPublishedDates(t *testing.T) {
	var ghFail atomic.Bool
	ghFail.Store(true)
	_, counts := newCountingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hashicorp/aws/versions":
			json.NewEncoder(w).Encode(ProviderResponse{Versions: []VersionInfo{{Version: "5.98.0"}}})
		case "/repos/hashicorp/terraform-provider-aws/releases":
			if ghFail.Load() {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			json.NewEncoder(w).Encode([]map[string]string{{"tag_name": "v5.98.0", "published_at": "2025-01-02T00:00:00Z"}})
		default:
			http.NotFound(w, r)
		}
	})

	versions, _, err := GetVersions("hashicorp", "aws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if versions[0].Published != "" {
		t.Fatalf("expected no published date on GitHub failure, got %q", versions[0].Published)
	}

	ghFail.Store(false)
	versions, _, err = GetVersions("hashicorp", "aws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if versions[0].Published != "Jan 2, 2025" {
		t.Errorf("expected published date after retry, got %q", versions[0].Published)
	}
	if got := counts["/hashicorp/aws/versions"].Load(); got != 1 {
		t.Errorf("registry versions requests: got %d, want 1", got)
	}
	if got := counts["/repos/hashicorp/terraform-provider-aws/releases"].Load(); got != 2 {
		t.Errorf("GitHub releases requests: got %d, want 2", got)
	}
}

func TestGetVersionsErrorsAreNotCached(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	_, counts := newCountingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hashicorp/aws/versions":
			if fail.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(ProviderResponse{Versions: []VersionInfo{{Version: "5.98.0"}}})
		default:
			http.NotFound(w, r)
		}
	})

	if _, _, err := GetVersions("hashicorp", "aws"); err == nil {
		t.Fatal("expected error")
	}
	fail.Store(false)
	if _, _, err := GetVersions("hashicorp", "aws"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := counts["/hashicorp/aws/versions"].Load(); got != 2 {
		t.Errorf("registry versions requests: got %d, want 2", got)
	}
}

func TestGetProvidersByTierIsCached(t *testing.T) {
	_, counts := newCountingServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"attributes": map[string]any{"namespace": "hashicorp", "name": "aws", "tier": "official", "downloads": 1}},
			},
			"meta": map[string]any{"pagination": map[string]any{"next-page": nil}},
		})
	})

	for i := 0; i < 3; i++ {
		got, err := GetProvidersByTier(TierOfficial)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].Name != "aws" {
			t.Fatalf("unexpected providers: %+v", got)
		}
		got[0].Name = "mutated"
	}
	if got := counts["/"].Load(); got != 1 {
		t.Errorf("provider list requests: got %d, want 1", got)
	}
}

func TestGetReleaseNotesCaching(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusForbidden)
	_, counts := newCountingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/hashicorp/terraform-provider-aws/releases/tags/v5.98.0" {
			http.NotFound(w, r)
			return
		}
		if s := int(status.Load()); s != http.StatusOK {
			w.WriteHeader(s)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"body": "## 5.98.0"})
	})
	const path = "/repos/hashicorp/terraform-provider-aws/releases/tags/v5.98.0"

	// Transient failures (e.g. rate limiting) are not cached.
	notes, err := GetReleaseNotes("hashicorp", "aws", "5.98.0")
	if err != nil || notes != noReleaseNotes {
		t.Fatalf("got (%q, %v), want placeholder", notes, err)
	}

	status.Store(http.StatusOK)
	for i := 0; i < 3; i++ {
		notes, err = GetReleaseNotes("hashicorp", "aws", "5.98.0")
		if err != nil || notes != "## 5.98.0" {
			t.Fatalf("got (%q, %v), want release notes", notes, err)
		}
	}
	if got := counts[path].Load(); got != 2 {
		t.Errorf("release notes requests: got %d, want 2", got)
	}
}

func TestResetCache(t *testing.T) {
	_, counts := newCountingServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"body": "notes"})
	})
	const path = "/repos/hashicorp/terraform-provider-aws/releases/tags/v5.98.0"

	GetReleaseNotes("hashicorp", "aws", "5.98.0")
	ResetCache()
	GetReleaseNotes("hashicorp", "aws", "5.98.0")
	if got := counts[path].Load(); got != 2 {
		t.Errorf("release notes requests: got %d, want 2", got)
	}
}
