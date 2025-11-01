package compactindex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client is a compact index HTTP client that maintains a Bundler-compatible cache.
type Client struct {
	baseURL    string
	cacheDir   string
	httpClient *http.Client
}

// NewClient creates a new compact index client.
// It writes to Bundler's cache location: ~/.bundle/cache/compact_index/{server_slug}/
func NewClient(baseURL string) (*Client, error) {
	// Compute Bundler cache path
	cacheDir, err := GetBundlerCachePath(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache path: %w", err)
	}

	// Ensure cache directories exist
	if err := EnsureCacheDirectories(cacheDir); err != nil {
		return nil, fmt.Errorf("failed to create cache directories: %w", err)
	}

	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		cacheDir:   cacheDir,
		httpClient: &http.Client{},
	}, nil
}

// GetVersions fetches and caches the versions file.
// Returns the parsed entries.
func (c *Client) GetVersions(ctx context.Context) ([]VersionsEntry, error) {
	localPath := filepath.Join(c.cacheDir, "versions")
	remotePath := "/versions"

	// Update local cache
	if err := c.updateFile(ctx, localPath, remotePath); err != nil {
		return nil, fmt.Errorf("failed to update versions file: %w", err)
	}

	// Parse and return
	return ParseVersionsFile(localPath)
}

// GetGemInfo fetches and caches the info file for a specific gem.
// Returns the parsed version information.
func (c *Client) GetGemInfo(ctx context.Context, gemName string) ([]VersionInfo, error) {
	localPath := GetInfoPath(c.cacheDir, gemName)
	remotePath := fmt.Sprintf("/info/%s", gemName)

	// Update local cache
	if err := c.updateFile(ctx, localPath, remotePath); err != nil {
		return nil, fmt.Errorf("failed to update info file for %s: %w", gemName, err)
	}

	// Parse and return
	return ParseInfoFile(localPath)
}

// updateFile updates a local cache file using HTTP with ETag and Range support.
// This implements Bundler's caching strategy.
func (c *Client) updateFile(ctx context.Context, localPath, remotePath string) error {
	// Check if local file exists
	localInfo, localErr := os.Stat(localPath)

	// Skip update if file is fresh (modified within last hour)
	// This matches Bundler's behavior and avoids unnecessary network + MD5 overhead
	if localErr == nil && localInfo.Size() > 0 {
		fileAge := time.Since(localInfo.ModTime())
		if fileAge < 1*time.Hour {
			// Cache is fresh, skip network request entirely
			return nil
		}
	}

	// Build request
	url := c.baseURL + remotePath
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	var localSize int64
	if localErr == nil {
		localSize = localInfo.Size()
	}

	// Add ETag header if we have a cached file
	if localErr == nil && localSize > 0 {
		// Compute current ETag (MD5 of file content)
		etag, err := ComputeInfoFileChecksum(localPath)
		if err == nil {
			req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, etag))
		}

		// Add Range header for incremental update
		// Bundler subtracts 1 to ensure non-empty range
		rangeStart := localSize - 1
		if rangeStart < 0 {
			rangeStart = 0
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", rangeStart))
	}

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		// Cache is fresh, nothing to do
		return nil
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read response body
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Handle 206 Partial Content (Range response)
	if resp.StatusCode == http.StatusPartialContent {
		return c.appendToFile(localPath, content)
	}

	// Handle 200 OK (Full content)
	return c.writeFile(localPath, content)
}

// writeFile writes content to a file atomically (using temp file + rename).
func (c *Client) writeFile(path string, content []byte) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Write to temp file
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath) // Clean up
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// appendToFile appends content to a file, skipping the first byte (Bundler's convention).
func (c *Client) appendToFile(path string, content []byte) error {
	// Open file for append
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Skip first byte (overlapping byte from Range request)
	if len(content) > 1 {
		content = content[1:]
	}

	// Append content
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("failed to append to file: %w", err)
	}

	return nil
}

// GetCacheDir returns the Bundler cache directory being used.
func (c *Client) GetCacheDir() string {
	return c.cacheDir
}
