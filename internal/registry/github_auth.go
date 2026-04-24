package registry

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// githubTokenLookup is the function used to resolve a GitHub token. It is a
// variable so tests can override the lookup behavior.
var githubTokenLookup = resolveGitHubToken

var (
	cachedGitHubToken    string
	cachedGitHubTokenOK  bool
	cachedGitHubTokenSet bool
	githubTokenOnce      sync.Once
)

// gitHubToken returns a GitHub token to authenticate API requests with, if one
// can be discovered in the environment. Discovery order:
//  1. GITHUB_TOKEN environment variable
//  2. GH_TOKEN environment variable
//  3. `gh auth token` (GitHub CLI), if installed and authenticated
//
// The result is cached for the lifetime of the process.
func gitHubToken() (string, bool) {
	githubTokenOnce.Do(func() {
		cachedGitHubToken, cachedGitHubTokenOK = githubTokenLookup()
		cachedGitHubTokenSet = true
	})
	return cachedGitHubToken, cachedGitHubTokenOK
}

func resolveGitHubToken() (string, bool) {
	for _, env := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v, true
		}
	}
	return ghCLIToken()
}

// ghCLIToken shells out to `gh auth token` to obtain a token from the GitHub
// CLI. It returns ("", false) if the CLI is not installed, the user is not
// logged in, or the call fails for any other reason.
func ghCLIToken() (string, bool) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", false
	}
	return token, true
}
