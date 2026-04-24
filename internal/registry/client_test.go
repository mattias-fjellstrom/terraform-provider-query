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
