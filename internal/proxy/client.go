package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const defaultMaxConcurrent = 10

func escapePath(path string) string {
	var result []byte
	for _, r := range path {
		if unicode.IsUpper(r) {
			result = append(result, '!')
			result = append(result, byte(unicode.ToLower(r)))
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}

// Client is a Go module proxy client
type Client struct {
	baseURL string
	http    *http.Client
	cache   Cache
	sem     chan struct{}
}


// VersionInfo represents module version metadata
type VersionInfo struct {
	Version string    `json:"Version"`
	Time    time.Time `json:"Time"`
}

// NewClient creates a new proxy client
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://proxy.golang.org"
	}
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: NewMemoryCache(),
		sem:   make(chan struct{}, defaultMaxConcurrent),
	}
}

// WithCache sets a custom cache implementation
func (c *Client) WithCache(cache Cache) *Client {
	c.cache = cache
	return c
}

func (c *Client) doRequest(ctx context.Context, url string) ([]byte, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("proxy returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// Latest fetches the latest version info for a module
func (c *Client) Latest(ctx context.Context, modulePath string) (*VersionInfo, error) {
	cacheKey := modulePath + "@latest"
	if cached, ok := c.cache.Get(cacheKey); ok {
		if info, ok := cached.(*VersionInfo); ok {
			return info, nil
		}
	}

	url := fmt.Sprintf("%s/%s/@latest", c.baseURL, escapePath(modulePath))
	body, err := c.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	var info VersionInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	c.cache.Set(cacheKey, &info, 5*time.Minute)

	return &info, nil
}

// Versions fetches all available versions for a module
func (c *Client) Versions(ctx context.Context, modulePath string) ([]string, error) {
	cacheKey := modulePath + "@list"
	if cached, ok := c.cache.Get(cacheKey); ok {
		if versions, ok := cached.([]string); ok {
			return versions, nil
		}
	}

	url := fmt.Sprintf("%s/%s/@v/list", c.baseURL, escapePath(modulePath))
	body, err := c.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	versions := strings.Split(strings.TrimSpace(string(body)), "\n")
	c.cache.Set(cacheKey, versions, 5*time.Minute)

	return versions, nil
}

// Info fetches version info for a specific module version
func (c *Client) Info(ctx context.Context, modulePath, version string) (*VersionInfo, error) {
	cacheKey := modulePath + "@" + version
	if cached, ok := c.cache.Get(cacheKey); ok {
		if info, ok := cached.(*VersionInfo); ok {
			return info, nil
		}
	}

	url := fmt.Sprintf("%s/%s/@v/%s.info", c.baseURL, escapePath(modulePath), version)
	body, err := c.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	var info VersionInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	c.cache.Set(cacheKey, &info, 1*time.Hour)

	return &info, nil
}

// GetModFile fetches the go.mod file for a specific module version
func (c *Client) GetModFile(ctx context.Context, modulePath, version string) ([]byte, error) {
	cacheKey := modulePath + "@" + version + ".mod"
	if cached, ok := c.cache.Get(cacheKey); ok {
		if data, ok := cached.([]byte); ok {
			return data, nil
		}
	}

	url := fmt.Sprintf("%s/%s/@v/%s.mod", c.baseURL, escapePath(modulePath), version)
	data, err := c.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, data, 1*time.Hour)

	return data, nil
}

