package registry

import (
	"strings"
	"sync"
)

// memo is a concurrency-safe, in-memory key/value cache. Registry and GitHub
// data changes slowly, so successful responses are kept for the lifetime of
// the process to avoid refetching the same data within a session.
type memo[V any] struct {
	mu sync.Mutex
	m  map[string]V
}

func (c *memo[V]) get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[key]
	return v, ok
}

func (c *memo[V]) set(key string, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]V)
	}
	c.m[key] = v
}

func (c *memo[V]) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = nil
}

// cacheKey builds a case-insensitive cache key, since registry namespaces and
// provider names are case-insensitive.
func cacheKey(parts ...string) string {
	return strings.ToLower(strings.Join(parts, "/"))
}

var (
	versionsCache       memo[[]VersionInfo]
	publishedDatesCache memo[map[string]string]
	providersCache      memo[[]Provider]
	releaseNotesCache   memo[string]
)

// ResetCache clears all cached registry and GitHub data, forcing subsequent
// calls to fetch fresh data.
func ResetCache() {
	versionsCache.reset()
	publishedDatesCache.reset()
	providersCache.reset()
	releaseNotesCache.reset()
}
