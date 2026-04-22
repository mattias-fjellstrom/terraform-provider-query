package registry

import (
	"encoding/json"
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
