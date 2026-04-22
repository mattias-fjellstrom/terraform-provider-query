package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
)

const baseURL = "https://registry.terraform.io/v1/providers"

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

// GetLatestVersion returns the latest version string for the given provider name
// by querying the provider metadata endpoint which includes the current latest version.
func GetLatestVersion(providerName string) (string, string, error) {
	namespace := "hashicorp"
	source := fmt.Sprintf("%s/%s", namespace, providerName)

	url := fmt.Sprintf("%s/%s/%s", baseURL, namespace, providerName)
	resp, err := http.Get(url)
	if err != nil {
		return "", source, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", source, fmt.Errorf("provider %q not found in the Terraform registry under hashicorp namespace", providerName)
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

// GetVersions returns all available versions for the given provider under hashicorp namespace.
func GetVersions(providerName string) ([]VersionInfo, string, error) {
	namespace := "hashicorp"
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

	return data.Versions, source, nil
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

// GetReleaseNotes fetches the changelog/release notes for a provider version from GitHub.
func GetReleaseNotes(providerName, version string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/hashicorp/terraform-provider-%s/releases/tags/v%s", providerName, version)
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
