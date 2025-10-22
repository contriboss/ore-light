package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/contriboss/gemfile-go/lockfile"
	"golang.org/x/sync/errgroup"
)

type downloadManager struct {
	cacheDir string
	baseURL  string
	client   *http.Client
	workers  int
}

type downloadReport struct {
	Total      int
	Downloaded int
	Skipped    int
	mu         sync.Mutex
}

func newDownloadManager(cacheDir, baseURL string, client *http.Client, workers int) (*downloadManager, error) {
	if cacheDir == "" {
		return nil, fmt.Errorf("cache directory must be provided")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("download base URL must be provided")
	}
	if client == nil {
		client = defaultHTTPClient()
	}
	if workers <= 0 {
		workers = 1
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &downloadManager{
		cacheDir: cacheDir,
		baseURL:  strings.TrimRight(baseURL, "/"),
		client:   client,
		workers:  workers,
	}, nil
}

func (m *downloadManager) DownloadAll(ctx context.Context, gems []lockfile.GemSpec, force bool) (downloadReport, error) {
	var report downloadReport
	report.Total = len(gems)

	g, ctx := errgroup.WithContext(ctx)
	semaphore := make(chan struct{}, m.workers)

	for _, gem := range gems {
		gem := gem

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			}

			downloaded, err := m.downloadGem(ctx, gem, force)
			if err != nil {
				return err
			}

			report.mu.Lock()
			if downloaded {
				report.Downloaded++
			} else {
				report.Skipped++
			}
			report.mu.Unlock()
			return nil
		})
	}

	err := g.Wait()
	return report, err
}

func (m *downloadManager) downloadGem(ctx context.Context, gem lockfile.GemSpec, force bool) (bool, error) {
	cachePath := m.cachePathFor(gem)
	if !force {
		// Check all cache locations (ore cache + system RubyGems cache)
		if foundPath := m.findInCaches(gem); foundPath != "" {
			// Gem found in cache, copy to primary cache if not there already
			if foundPath != cachePath {
				if err := copyFile(foundPath, cachePath); err != nil {
					// Non-fatal: we can still use the gem from system cache
					// but log the copy failure for visibility
					fmt.Fprintf(os.Stderr, "Note: Using %s from system cache (copy failed: %v)\n", gem.FullName(), err)
				}
			}
			return false, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return false, fmt.Errorf("failed to prepare cache dir: %w", err)
	}

	url := fmt.Sprintf("%s/%s", m.baseURL, gemFileName(gem))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request for %s: %w", gem.FullName(), err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to download %s: %w", gem.FullName(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d while downloading %s", resp.StatusCode, gem.FullName())
	}

	tempFile, err := os.CreateTemp(filepath.Dir(cachePath), "ore-*.gem")
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return false, fmt.Errorf("failed to write gem %s: %w", gem.FullName(), err)
	}

	if err := tempFile.Close(); err != nil {
		return false, fmt.Errorf("failed to close temp file for %s: %w", gem.FullName(), err)
	}

	if err := os.Rename(tempFile.Name(), cachePath); err != nil {
		return false, fmt.Errorf("failed to finalize download for %s: %w", gem.FullName(), err)
	}

	fmt.Printf("Fetched %s\n", gem.FullName())
	return true, nil
}

func (m *downloadManager) cachePathFor(gem lockfile.GemSpec) string {
	return filepath.Join(m.cacheDir, gemFileName(gem))
}

// cacheLocations returns all cache directories to check for gems
func (m *downloadManager) cacheLocations() []string {
	locations := []string{m.cacheDir} // Ore cache first

	// Try to get system RubyGems caches (only if Ruby is available)
	if gemPaths := tryGetGemPaths(); len(gemPaths) > 0 {
		for _, gemPath := range gemPaths {
			cacheDir := filepath.Join(gemPath, "cache")
			locations = append(locations, cacheDir)
		}
	}

	return locations
}

// tryGetGemPaths attempts to get gem paths, returns empty if Ruby not available
func tryGetGemPaths() []string {
	// Try using `gem environment gempath` if Ruby is available
	cmd := exec.Command("gem", "environment", "gempath")
	output, err := cmd.Output()
	if err == nil {
		pathsStr := strings.TrimSpace(string(output))
		if pathsStr != "" {
			return strings.Split(pathsStr, string(filepath.ListSeparator))
		}
	}

	// Ruby not available - try common default locations based on detected version
	var defaultPaths []string
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultPaths
	}

	rubyVer := detectRubyVersionForVendor()
	if rubyVer == "" {
		return defaultPaths
	}

	// Common RubyGems cache locations (check if they exist)
	commonLocations := []string{
		filepath.Join(home, ".gem", "ruby", rubyVer),
		filepath.Join(home, ".local", "share", "gem", "ruby", rubyVer),
	}

	// Check glob patterns for version managers
	globPatterns := []string{
		filepath.Join(home, ".rbenv", "versions", "*", "lib", "ruby", "gems", rubyVer),
		filepath.Join(home, ".asdf", "installs", "ruby", "*", "lib", "ruby", "gems", rubyVer),
		filepath.Join(home, ".local", "share", "mise", "installs", "ruby", "*", "lib", "ruby", "gems", rubyVer),
	}

	for _, pattern := range globPatterns {
		if matches, err := filepath.Glob(pattern); err == nil {
			defaultPaths = append(defaultPaths, matches...)
		}
	}

	for _, location := range commonLocations {
		if _, err := os.Stat(location); err == nil {
			defaultPaths = append(defaultPaths, location)
		}
	}

	return defaultPaths
}

// findInCaches searches all cache locations for a gem
func (m *downloadManager) findInCaches(gem lockfile.GemSpec) string {
	fileName := gemFileName(gem)
	for _, cacheDir := range m.cacheLocations() {
		path := filepath.Join(cacheDir, fileName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

func (m *downloadManager) CacheDir() string {
	return m.cacheDir
}

func gemFileName(gem lockfile.GemSpec) string {
	return fmt.Sprintf("%s.gem", gem.FullName())
}
