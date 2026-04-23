package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	baseURL   = "https://registry.terraform.io/v1/providers"
	v2BaseURL = "https://registry.terraform.io/v2/providers"
	v2RootURL = "https://registry.terraform.io/v2"
)

// Tier values returned by the Terraform Registry v2 API.
const (
	TierOfficial  = "official"
	TierPartner   = "partner"
	TierCommunity = "community"
)

// Provider is a lightweight representation of a provider listed in the
// Terraform Registry, suitable for browsing.
type Provider struct {
	Namespace   string
	Name        string
	Tier        string
	Downloads   int
	Description string
}

// FullName returns "namespace/name".
func (p Provider) FullName() string { return p.Namespace + "/" + p.Name }

// ParseProvider splits an input string of the form "namespace/name" into its parts.
// If no namespace is provided, it defaults to "hashicorp".
func ParseProvider(input string) (namespace, name string) {
	if idx := strings.Index(input, "/"); idx != -1 {
		return input[:idx], input[idx+1:]
	}
	return "hashicorp", input
}

type VersionInfo struct {
	Version   string `json:"version"`
	Published string `json:"published_at"`
}

type ProviderResponse struct {
	ID       string        `json:"id"`
	Source   string        `json:"full_name"`
	Versions []VersionInfo `json:"versions"`
}

type VersionDetail struct {
	Version string `json:"tag"`
	Body    string `json:"body"`
}

// GetLatestVersion returns the latest version string for the given provider
// by querying the provider metadata endpoint which includes the current latest version.
func GetLatestVersion(namespace, providerName string) (string, string, error) {
	source := fmt.Sprintf("%s/%s", namespace, providerName)

	url := fmt.Sprintf("%s/%s/%s", baseURL, namespace, providerName)
	resp, err := http.Get(url)
	if err != nil {
		return "", source, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", source, fmt.Errorf("provider %q not found in the Terraform registry under %q namespace", providerName, namespace)
	}
	if resp.StatusCode != 200 {
		return "", source, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var data struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", source, fmt.Errorf("failed to decode response: %w", err)
	}

	if data.Version == "" {
		return "", source, fmt.Errorf("no version found for provider %q", providerName)
	}

	return data.Version, source, nil
}

// GetVersions returns all available versions for the given provider under the given namespace.
func GetVersions(namespace, providerName string) ([]VersionInfo, string, error) {
	source := fmt.Sprintf("%s/%s", namespace, providerName)

	url := fmt.Sprintf("%s/%s/%s/versions", baseURL, namespace, providerName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, source, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, source, fmt.Errorf("provider %q not found", providerName)
	}
	if resp.StatusCode != 200 {
		return nil, source, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var data ProviderResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, source, fmt.Errorf("failed to decode response: %w", err)
	}

	sort.Slice(data.Versions, func(i, j int) bool {
		return semverGT(data.Versions[i].Version, data.Versions[j].Version)
	})

	dates, _ := GetVersionPublishedDates(namespace, providerName)
	for i := range data.Versions {
		if d, ok := dates[data.Versions[i].Version]; ok {
			data.Versions[i].Published = d
		}
	}

	return data.Versions, source, nil
}

// GetVersionPublishedDates fetches published dates for provider versions from the
// GitHub releases API and returns a map of version string to formatted date.
func GetVersionPublishedDates(namespace, providerName string) (map[string]string, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/terraform-provider-%s/releases?per_page=100",
		namespace,
		providerName,
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	dates := make(map[string]string, len(releases))
	for _, r := range releases {
		version := strings.TrimPrefix(r.TagName, "v")
		if t, err := time.Parse(time.RFC3339, r.PublishedAt); err == nil {
			dates[version] = t.Format("Jan 2, 2006")
		}
	}
	return dates, nil
}

var semverDigits = regexp.MustCompile(`\d+`)

// semverGT reports whether version string a is greater than b using numeric part comparison.
func semverGT(a, b string) bool {
	aParts := semverDigits.FindAllString(a, -1)
	bParts := semverDigits.FindAllString(b, -1)
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		ai, _ := strconv.Atoi(aParts[i])
		bi, _ := strconv.Atoi(bParts[i])
		if ai != bi {
			return ai > bi
		}
	}
	return len(aParts) > len(bParts)
}

// providersV2Page is the JSON shape returned by GET /v2/providers.
type providersV2Page struct {
	Data []struct {
		Attributes struct {
			Namespace   string `json:"namespace"`
			Name        string `json:"name"`
			Tier        string `json:"tier"`
			Downloads   int    `json:"downloads"`
			Description string `json:"description"`
		} `json:"attributes"`
	} `json:"data"`
	Meta struct {
		Pagination struct {
			NextPage *int `json:"next-page"`
		} `json:"pagination"`
	} `json:"meta"`
}

// GetProvidersByTier returns all providers in the given tier (official,
// partner, or community) sorted by download count descending.
func GetProvidersByTier(tier string) ([]Provider, error) {
	const pageSize = 100
	var providers []Provider

	page := 1
	for {
		url := fmt.Sprintf("%s?filter%%5Btier%%5D=%s&page%%5Bsize%%5D=%d&page%%5Bnumber%%5D=%d", v2BaseURL, tier, pageSize, page)
		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		var data providersV2Page
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		for _, d := range data.Data {
			providers = append(providers, Provider{
				Namespace:   d.Attributes.Namespace,
				Name:        d.Attributes.Name,
				Tier:        d.Attributes.Tier,
				Downloads:   d.Attributes.Downloads,
				Description: d.Attributes.Description,
			})
		}

		if data.Meta.Pagination.NextPage == nil {
			break
		}
		page = *data.Meta.Pagination.NextPage
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Downloads > providers[j].Downloads
	})
	return providers, nil
}

// ProviderDoc is a lightweight representation of a single documentation
// page for a provider version, suitable for listing in a TUI.
type ProviderDoc struct {
	ID          string
	Category    string
	Title       string
	Slug        string
	Subcategory string
	Language    string
	Path        string
}

// ProviderDocContent is a fully-fetched documentation page for a provider
// version, including the markdown body.
type ProviderDocContent struct {
	ID       string
	Title    string
	Category string
	Content  string
}

// providerWithVersionsPayload mirrors GET /v2/providers/{ns}/{name}?include=provider-versions.
type providerWithVersionsPayload struct {
	Included []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Version string `json:"version"`
		} `json:"attributes"`
	} `json:"included"`
}

// GetProviderVersionID resolves a provider version string (e.g. "5.98.0")
// to the Terraform Registry's internal numeric provider-version ID, which
// is required by the documentation endpoints.
func GetProviderVersionID(namespace, providerName, version string) (string, error) {
	url := fmt.Sprintf("%s/%s/%s?include=provider-versions", v2BaseURL, namespace, providerName)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("provider %q/%q not found", namespace, providerName)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var data providerWithVersionsPayload
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	for _, inc := range data.Included {
		if inc.Type == "provider-versions" && inc.Attributes.Version == version {
			return inc.ID, nil
		}
	}
	return "", fmt.Errorf("version %q not found for provider %s/%s", version, namespace, providerName)
}

// providerVersionDocsPayload mirrors GET /v2/provider-versions/{id}?include=provider-docs.
type providerVersionDocsPayload struct {
	Included []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Category    string  `json:"category"`
			Language    string  `json:"language"`
			Path        string  `json:"path"`
			Slug        string  `json:"slug"`
			Subcategory *string `json:"subcategory"`
			Title       string  `json:"title"`
		} `json:"attributes"`
	} `json:"included"`
}

// docCategoryRank orders docs in a sensible reading order.
func docCategoryRank(category string) int {
	switch category {
	case "overview":
		return 0
	case "guides":
		return 1
	case "resources":
		return 2
	case "data-sources":
		return 3
	case "ephemeral-resources":
		return 4
	case "functions":
		return 5
	case "actions":
		return 6
	default:
		return 7
	}
}

// ListProviderDocs returns all documentation pages for the given provider
// version, restricted to the canonical HCL language so framework providers
// don't show duplicate Python/TypeScript variants.
func ListProviderDocs(versionID string) ([]ProviderDoc, error) {
	url := fmt.Sprintf("%s/provider-versions/%s?include=provider-docs", v2RootURL, versionID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("provider version %q not found", versionID)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var data providerVersionDocsPayload
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	docs := make([]ProviderDoc, 0, len(data.Included))
	for _, inc := range data.Included {
		if inc.Type != "provider-docs" {
			continue
		}
		if inc.Attributes.Language != "" && inc.Attributes.Language != "hcl" {
			continue
		}
		sub := ""
		if inc.Attributes.Subcategory != nil {
			sub = *inc.Attributes.Subcategory
		}
		docs = append(docs, ProviderDoc{
			ID:          inc.ID,
			Category:    inc.Attributes.Category,
			Title:       inc.Attributes.Title,
			Slug:        inc.Attributes.Slug,
			Subcategory: sub,
			Language:    inc.Attributes.Language,
			Path:        inc.Attributes.Path,
		})
	}

	sort.SliceStable(docs, func(i, j int) bool {
		ri, rj := docCategoryRank(docs[i].Category), docCategoryRank(docs[j].Category)
		if ri != rj {
			return ri < rj
		}
		if docs[i].Category != docs[j].Category {
			return docs[i].Category < docs[j].Category
		}
		return strings.ToLower(docs[i].Title) < strings.ToLower(docs[j].Title)
	})

	return docs, nil
}

// providerDocPayload mirrors GET /v2/provider-docs/{id}.
type providerDocPayload struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Category string `json:"category"`
			Title    string `json:"title"`
			Content  string `json:"content"`
		} `json:"attributes"`
	} `json:"data"`
}

// GetProviderDoc fetches the markdown body of a single documentation page.
func GetProviderDoc(docID string) (ProviderDocContent, error) {
	url := fmt.Sprintf("%s/provider-docs/%s", v2RootURL, docID)
	resp, err := http.Get(url)
	if err != nil {
		return ProviderDocContent{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return ProviderDocContent{}, fmt.Errorf("doc %q not found", docID)
	}
	if resp.StatusCode != 200 {
		return ProviderDocContent{}, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var data providerDocPayload
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ProviderDocContent{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return ProviderDocContent{
		ID:       data.Data.ID,
		Title:    data.Data.Attributes.Title,
		Category: data.Data.Attributes.Category,
		Content:  data.Data.Attributes.Content,
	}, nil
}

// GetReleaseNotes fetches the changelog/release notes for a provider version from GitHub.
func GetReleaseNotes(namespace, providerName, version string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/terraform-provider-%s/releases/tags/v%s", namespace, providerName, version)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "(no release notes available)", nil
	}

	var release struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.Body == "" {
		return "(no release notes available)", nil
	}
	return release.Body, nil
}
