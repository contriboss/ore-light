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
	"github.com/contriboss/ore-light/internal/sources"
	"golang.org/x/sync/errgroup"
)

// Ruby developers: This is like a Ruby class with instance variables
// Go uses structs instead of classes - no inheritance, just composition
type downloadManager struct {
	cacheDir      string
	sourceManager *sources.Manager
	workers       int
}

// This is like a thread-safe Ruby object with attr_accessor methods
// mu (mutex) protects concurrent writes - Ruby's Thread::Mutex equivalent
type downloadReport struct {
	Total      int
	Downloaded int
	Skipped    int
	mu         sync.Mutex
}

func newDownloadManager(cacheDir string, sourceConfigs []SourceConfig, client *http.Client, workers int) (*downloadManager, error) {
	if cacheDir == "" {
		return nil, fmt.Errorf("cache directory must be provided")
	}
	if len(sourceConfigs) == 0 {
		return nil, fmt.Errorf("at least one gem source must be configured")
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

	// Convert our SourceConfig to sources.SourceConfig for the manager
	managerConfigs := make([]sources.SourceConfig, len(sourceConfigs))
	for i, config := range sourceConfigs {
		managerConfigs[i] = sources.SourceConfig{
			URL:      config.URL,
			Fallback: config.Fallback,
		}
	}

	return &downloadManager{
		cacheDir:      cacheDir,
		sourceManager: sources.NewManager(managerConfigs, client),
		workers:       workers,
	}, nil
}

func (m *downloadManager) DownloadAll(ctx context.Context, gems []lockfile.GemSpec, force bool) (downloadReport, error) {
	var report downloadReport
	report.Total = len(gems)

	// Ruby developers: errgroup is like Ruby's concurrent-ruby gem
	// It manages goroutines and collects errors - similar to ThreadPoolExecutor
	// Go's concurrency model: goroutines (lightweight threads) + channels (message passing)
	g, ctx := errgroup.WithContext(ctx)
	// Semaphore pattern using buffered channels - limits concurrent downloads
	// Ruby's Concurrent::Semaphore, but using Go's channel semantics
	semaphore := make(chan struct{}, m.workers) // Buffered channel = max concurrent

	for _, gem := range gems {
		// Ruby developers: This is a Go gotcha! We must capture loop variables
		// Unlike Ruby blocks, Go closures can capture changing variables
		gem := gem

		// g.Go is like spawning a thread/fiber - runs concurrently
		g.Go(func() error {
			// This select is like Ruby's Timeout.timeout but more explicit
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

			// Mutex.Lock/Unlock is like Ruby's synchronize { } block
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

	// Wait for all goroutines - like Thread.join in Ruby
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

	tempFile, err := os.CreateTemp(filepath.Dir(cachePath), "ore-*.gem")
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()

	// Use SourceManager to download with fallback support
	gemName := gemFileName(gem)
	if err := m.sourceManager.DownloadGem(ctx, gemName, tempFile); err != nil {
		return false, fmt.Errorf("failed to download %s: %w", gem.FullName(), err)
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

// CheckSourceHealth performs pre-flight health checks on all configured sources
func (m *downloadManager) CheckSourceHealth(ctx context.Context) {
	fmt.Println("Checking gem source availability...")
	m.sourceManager.CheckHealth(ctx)

	// Print health status
	sources := m.sourceManager.GetSources()
	for _, source := range sources {
		if source.Healthy {
			fmt.Printf("  ✓ %s (healthy)\n", source.URL)
		} else {
			fmt.Printf("  ✗ %s (unavailable)\n", source.URL)
		}
		if source.Fallback != "" {
			if source.FallbackHealthy {
				fmt.Printf("    └─ fallback: %s (healthy)\n", source.Fallback)
			} else {
				fmt.Printf("    └─ fallback: %s (unavailable)\n", source.Fallback)
			}
		}
	}
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
