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
