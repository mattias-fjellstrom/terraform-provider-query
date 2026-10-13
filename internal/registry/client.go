package registry

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	baseURL          = "https://registry.terraform.io/v1/providers"
	v2BaseURL        = "https://registry.terraform.io/v2/providers"
	githubAPIBaseURL = "https://api.github.com"
)

// Tier values returned by the Terraform Registry v2 API.
const (
	TierOfficial       = "official"
	TierPartnerPremier = "partner-premier"
	TierPartner        = "partner"
	TierCommunity      = "community"
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

// GetVersions returns all available versions for the given provider under the given namespace.
// Successful results are cached for the lifetime of the process.
func GetVersions(namespace, providerName string) ([]VersionInfo, string, error) {
	source := fmt.Sprintf("%s/%s", namespace, providerName)

	cached, err := getRegistryVersions(namespace, providerName)
	if err != nil {
		return nil, source, err
	}

	// Copy so callers can't mutate the cached slice.
	versions := slices.Clone(cached)

	// Published dates are fetched (and cached) separately so a transient
	// GitHub failure doesn't prevent dates from being filled in later.
	dates, _ := GetVersionPublishedDates(namespace, providerName)
	for i := range versions {
		if d, ok := dates[versions[i].Version]; ok {
			versions[i].Published = d
		}
	}

	return versions, source, nil
}

// getRegistryVersions fetches the provider's versions from the Terraform
// Registry, sorted newest first, using the session cache when possible.
func getRegistryVersions(namespace, providerName string) ([]VersionInfo, error) {
	key := cacheKey(namespace, providerName)
	if v, ok := versionsCache.get(key); ok {
		return v, nil
	}

	url := fmt.Sprintf("%s/%s/%s/versions", baseURL, namespace, providerName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("provider %q not found", providerName)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var data ProviderResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	sort.Slice(data.Versions, func(i, j int) bool {
		return semverGT(data.Versions[i].Version, data.Versions[j].Version)
	})

	versionsCache.set(key, data.Versions)
	return data.Versions, nil
}

// GetVersionPublishedDates fetches published dates for provider versions from the
// GitHub releases API and returns a map of version string to formatted date.
// Successful results are cached for the lifetime of the process.
func GetVersionPublishedDates(namespace, providerName string) (map[string]string, error) {
	key := cacheKey(namespace, providerName)
	if d, ok := publishedDatesCache.get(key); ok {
		return maps.Clone(d), nil
	}

	url := fmt.Sprintf(
		"%s/repos/%s/terraform-provider-%s/releases?per_page=100",
		githubAPIBaseURL,
		namespace,
		providerName,
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token, ok := gitHubToken(); ok {
		req.Header.Set("Authorization", "Bearer "+token)
	}

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
	publishedDatesCache.set(key, dates)
	return maps.Clone(dates), nil
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
// partner-premier, partner, or community) sorted by download count descending.
// Successful results are cached for the lifetime of the process.
func GetProvidersByTier(tier string) ([]Provider, error) {
	key := cacheKey(tier)
	if p, ok := providersCache.get(key); ok {
		return slices.Clone(p), nil
	}

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
	providersCache.set(key, providers)
	return slices.Clone(providers), nil
}

const noReleaseNotes = "(no release notes available)"

// GetReleaseNotes fetches the changelog/release notes for a provider version from GitHub.
// Found (and definitively missing) release notes are cached for the lifetime
// of the process; transient failures such as rate limiting are not.
func GetReleaseNotes(namespace, providerName, version string) (string, error) {
	key := cacheKey(namespace, providerName, version)
	if n, ok := releaseNotesCache.get(key); ok {
		return n, nil
	}

	url := fmt.Sprintf("%s/repos/%s/terraform-provider-%s/releases/tags/v%s", githubAPIBaseURL, namespace, providerName, version)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token, ok := gitHubToken(); ok {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		releaseNotesCache.set(key, noReleaseNotes)
		return noReleaseNotes, nil
	}
	if resp.StatusCode != 200 {
		return noReleaseNotes, nil
	}

	var release struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	notes := release.Body
	if notes == "" {
		notes = noReleaseNotes
	}
	releaseNotesCache.set(key, notes)
	return notes, nil
}
