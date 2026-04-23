package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseProvider(t *testing.T) {
	tests := []struct {
		input             string
		wantNamespace     string
		wantProviderName  string
	}{
		{"aws", "hashicorp", "aws"},
		{"hashicorp/aws", "hashicorp", "aws"},
		{"integrations/github", "integrations", "github"},
		{"myorg/myprovider", "myorg", "myprovider"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ns, name := ParseProvider(tt.input)
			if ns != tt.wantNamespace {
				t.Errorf("namespace: got %q, want %q", ns, tt.wantNamespace)
			}
			if name != tt.wantProviderName {
				t.Errorf("name: got %q, want %q", name, tt.wantProviderName)
			}
		})
	}
}

func TestSemverGT(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"5.98.0", "5.97.0", true},
		{"5.97.0", "5.98.0", false},
		{"5.98.0", "5.98.0", false},
		{"2.0.0", "1.99.99", true},
		{"1.0.0", "2.0.0", false},
		{"1.10.0", "1.9.0", true},
		{"1.9.0", "1.10.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := semverGT(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("semverGT(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGetLatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hashicorp/aws":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"version": "5.98.0"})
		case "/hashicorp/missing":
			http.NotFound(w, r)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	origBase := baseURL
	baseURL = srv.URL
	defer func() { baseURL = origBase }()

	t.Run("found", func(t *testing.T) {
		ver, source, err := GetLatestVersion("hashicorp", "aws")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ver != "5.98.0" {
			t.Errorf("version: got %q, want %q", ver, "5.98.0")
		}
		if source != "hashicorp/aws" {
			t.Errorf("source: got %q, want %q", source, "hashicorp/aws")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, _, err := GetLatestVersion("hashicorp", "missing")
		if err == nil {
			t.Fatal("expected an error for missing provider, got nil")
		}
	})
}

func TestGetVersions(t *testing.T) {
	payload := ProviderResponse{
		ID:     "hashicorp/aws",
		Source: "hashicorp/aws",
		Versions: []VersionInfo{
			{Version: "5.97.0"},
			{Version: "5.98.0"},
			{Version: "5.96.0"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hashicorp/aws/versions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	origBase := baseURL
	baseURL = srv.URL
	defer func() { baseURL = origBase }()

	versions, source, err := GetVersions("hashicorp", "aws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != "hashicorp/aws" {
		t.Errorf("source: got %q, want %q", source, "hashicorp/aws")
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	// Versions must be sorted descending
	if versions[0].Version != "5.98.0" {
		t.Errorf("first version: got %q, want %q", versions[0].Version, "5.98.0")
	}
	if versions[2].Version != "5.96.0" {
		t.Errorf("last version: got %q, want %q", versions[2].Version, "5.96.0")
	}
}

func TestGetProviderVersionID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hashicorp/aws":
			if got := r.URL.Query().Get("include"); got != "provider-versions" {
				t.Errorf("include: got %q, want provider-versions", got)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"id":"323"},"included":[
				{"type":"provider-versions","id":"100","attributes":{"version":"5.97.0"}},
				{"type":"provider-versions","id":"200","attributes":{"version":"5.98.0"}}
			]}`)
		case "/hashicorp/missing":
			http.NotFound(w, r)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	orig := v2BaseURL
	v2BaseURL = srv.URL
	defer func() { v2BaseURL = orig }()

	t.Run("found", func(t *testing.T) {
		id, err := GetProviderVersionID("hashicorp", "aws", "5.98.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "200" {
			t.Errorf("id: got %q, want %q", id, "200")
		}
	})

	t.Run("version missing", func(t *testing.T) {
		if _, err := GetProviderVersionID("hashicorp", "aws", "9.9.9"); err == nil {
			t.Fatal("expected error for missing version")
		}
	})

	t.Run("provider not found", func(t *testing.T) {
		if _, err := GetProviderVersionID("hashicorp", "missing", "1.0.0"); err == nil {
			t.Fatal("expected error for missing provider")
		}
	})
}

func TestListProviderDocs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/provider-versions/200":
			if got := r.URL.Query().Get("include"); got != "provider-docs" {
				t.Errorf("include: got %q, want provider-docs", got)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"id":"200"},"included":[
				{"type":"provider-docs","id":"1","attributes":{"category":"resources","language":"hcl","title":"bucket","slug":"bucket","path":"docs/resources/bucket.md"}},
				{"type":"provider-docs","id":"2","attributes":{"category":"overview","language":"hcl","title":"overview","slug":"index","path":"docs/index.md"}},
				{"type":"provider-docs","id":"3","attributes":{"category":"resources","language":"python","title":"bucket","slug":"bucket","path":"docs/cdktf/python/resources/bucket.md"}},
				{"type":"provider-docs","id":"4","attributes":{"category":"data-sources","language":"hcl","title":"caller_identity","slug":"caller_identity","path":"docs/data-sources/caller_identity.md"}}
			]}`)
		case "/provider-versions/404":
			http.NotFound(w, r)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	orig := v2RootURL
	v2RootURL = srv.URL
	defer func() { v2RootURL = orig }()

	t.Run("found", func(t *testing.T) {
		docs, err := ListProviderDocs("200")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(docs) != 3 {
			t.Fatalf("expected 3 hcl docs, got %d", len(docs))
		}
		// Order: overview, resources(bucket), data-sources(caller_identity)
		wantOrder := []string{"overview", "bucket", "caller_identity"}
		for i, w := range wantOrder {
			if docs[i].Title != w {
				t.Errorf("idx %d: got %q, want %q", i, docs[i].Title, w)
			}
		}
		// non-hcl filtered out
		for _, d := range docs {
			if d.Language != "" && d.Language != "hcl" {
				t.Errorf("unexpected non-hcl doc: %+v", d)
			}
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := ListProviderDocs("404"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGetProviderDoc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/provider-docs/1":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"id":"1","attributes":{"category":"resources","title":"bucket","content":"# bucket\nbody"}}}`)
		case "/provider-docs/404":
			http.NotFound(w, r)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	orig := v2RootURL
	v2RootURL = srv.URL
	defer func() { v2RootURL = orig }()

	t.Run("found", func(t *testing.T) {
		doc, err := GetProviderDoc("1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.ID != "1" || doc.Title != "bucket" || doc.Category != "resources" {
			t.Errorf("metadata mismatch: %+v", doc)
		}
		if doc.Content != "# bucket\nbody" {
			t.Errorf("content: got %q", doc.Content)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := GetProviderDoc("404"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGetProvidersByTier(t *testing.T) {
	type attr struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Tier      string `json:"tier"`
		Downloads int    `json:"downloads"`
	}
	type item struct {
		Attributes attr `json:"attributes"`
	}
	pages := [][]item{
		{
			{Attributes: attr{Namespace: "hashicorp", Name: "aws", Tier: "official", Downloads: 100}},
			{Attributes: attr{Namespace: "hashicorp", Name: "azurerm", Tier: "official", Downloads: 300}},
		},
		{
			{Attributes: attr{Namespace: "hashicorp", Name: "google", Tier: "official", Downloads: 200}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("filter[tier]"), "official"; got != want {
			t.Errorf("tier filter: got %q, want %q", got, want)
		}
		page := r.URL.Query().Get("page[number]")
		if page == "" {
			page = "1"
		}
		var idx int
		fmt.Sscanf(page, "%d", &idx)
		if idx < 1 || idx > len(pages) {
			http.NotFound(w, r)
			return
		}
		var next *int
		if idx < len(pages) {
			n := idx + 1
			next = &n
		}
		resp := map[string]any{
			"data": pages[idx-1],
			"meta": map[string]any{
				"pagination": map[string]any{"next-page": next},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	orig := v2BaseURL
	v2BaseURL = srv.URL
	defer func() { v2BaseURL = orig }()

	got, err := GetProvidersByTier(TierOfficial)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 providers (pagination followed), got %d", len(got))
	}
	// Sorted by downloads desc: azurerm(300), google(200), aws(100).
	wantOrder := []string{"azurerm", "google", "aws"}
	for i, w := range wantOrder {
		if got[i].Name != w {
			t.Errorf("idx %d: got %q, want %q", i, got[i].Name, w)
		}
	}
	if got[0].FullName() != "hashicorp/azurerm" {
		t.Errorf("FullName: got %q, want %q", got[0].FullName(), "hashicorp/azurerm")
	}
}
