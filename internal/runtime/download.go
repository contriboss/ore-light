package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/ruby"
	"github.com/contriboss/ore-light/internal/sources"
	"golang.org/x/sync/errgroup"
)

const (
	// DEFAULT_RUBY_VERSION is the fallback Ruby version when detection fails
	DEFAULT_RUBY_VERSION = "3.4.8"
)

type DownloadManager struct {
	cacheDir      string
	sourceManager *sources.Manager
	workers       int
}

type DownloadReport struct {
	Total      int
	Downloaded int
	Skipped    int
	mu         sync.Mutex
}

func newDownloadManager(cacheDir string, sourceConfigs []SourceConfig, client *http.Client, workers int) (*DownloadManager, error) {
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

	return &DownloadManager{
		cacheDir:      cacheDir,
		sourceManager: sources.NewManager(sourceConfigs, client),
		workers:       workers,
	}, nil
}

func (m *DownloadManager) DownloadAll(ctx context.Context, gems []lockfile.GemSpec, force bool) (*DownloadReport, error) {
	report := &DownloadReport{}
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

func (m *DownloadManager) downloadGem(ctx context.Context, gem lockfile.GemSpec, force bool) (bool, error) {
	cachePath := m.cachePathFor(gem)
	metaPath := cachePath + ".meta"
	if !force {
		if foundPath := m.findInCaches(gem); foundPath != "" {
			if foundPath != cachePath {
				if err := copyFile(foundPath, cachePath); err != nil {
					fmt.Fprintf(os.Stderr, "Note: Using %s from system cache (copy failed: %v)\n", gem.FullName(), err)
				}
			}
			return false, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return false, fmt.Errorf("failed to prepare cache dir: %w", err)
	}

	existingMeta, _ := loadCacheMetadata(metaPath)

	tempFile, err := os.CreateTemp(filepath.Dir(cachePath), "ore-*.gem")
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	headers := map[string]string{}
	if existingMeta != nil {
		if existingMeta.ETag != "" {
			headers["If-None-Match"] = existingMeta.ETag
		}
		if existingMeta.LastModified != "" {
			headers["If-Modified-Since"] = existingMeta.LastModified
		}
	}

	gemName := gemFileName(gem)
	status, respHeaders, err := m.sourceManager.DownloadGemWithHeaders(ctx, gemName, tempFile, headers)
	if err != nil {
		return false, fmt.Errorf("failed to download %s: %w", gem.FullName(), err)
	}
	if status == http.StatusNotModified {
		return false, nil
	}

	if err := tempFile.Close(); err != nil {
		return false, fmt.Errorf("failed to close temp file for %s: %w", gem.FullName(), err)
	}

	if err := os.Rename(tempFile.Name(), cachePath); err != nil {
		return false, fmt.Errorf("failed to finalize download for %s: %w", gem.FullName(), err)
	}

	if respHeaders != nil {
		meta := cacheMetadata{
			ETag:         respHeaders.Get("ETag"),
			LastModified: respHeaders.Get("Last-Modified"),
		}
		_ = saveCacheMetadata(metaPath, meta)
	}

	fmt.Printf("Fetched %s\n", gem.FullName())
	return true, nil
}

func (m *DownloadManager) cachePathFor(gem lockfile.GemSpec) string {
	return filepath.Join(m.cacheDir, gemFileName(gem))
}

func (m *DownloadManager) CheckSourceHealth(ctx context.Context) {
	fmt.Println("Checking gem source availability...")
	m.sourceManager.CheckHealth(ctx)

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

func (m *DownloadManager) cacheLocations() []string {
	locations := []string{m.cacheDir}

	if gemPaths := tryGetGemPaths(); len(gemPaths) > 0 {
		for _, gemPath := range gemPaths {
			cacheDir := filepath.Join(gemPath, "cache")
			locations = append(locations, cacheDir)
		}
	}

	return locations
}

func tryGetGemPaths() []string {
	cmd := exec.Command("gem", "environment", "gempath")
	output, err := cmd.Output()
	if err == nil {
		pathsStr := strings.TrimSpace(string(output))
		if pathsStr != "" {
			return strings.Split(pathsStr, string(filepath.ListSeparator))
		}
	}

	var defaultPaths []string
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultPaths
	}

	rubyVer := detectRubyVersion()
	if rubyVer == "" {
		return defaultPaths
	}

	commonLocations := []string{
		filepath.Join(home, ".gem", "ruby", rubyVer),
		filepath.Join(home, ".local", "share", "gem", "ruby", rubyVer),
	}

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

func (m *DownloadManager) findInCaches(gem lockfile.GemSpec) string {
	fileName := gemFileName(gem)
	for _, cacheDir := range m.cacheLocations() {
		path := filepath.Join(cacheDir, fileName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

func (m *DownloadManager) CacheDir() string {
	return m.cacheDir
}

func gemFileName(gem lockfile.GemSpec) string {
	return fmt.Sprintf("%s.gem", gem.FullName())
}

type cacheMetadata struct {
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
}

func loadCacheMetadata(path string) (*cacheMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta cacheMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func saveCacheMetadata(path string, meta cacheMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

// NewDownloadManager creates a download manager with configured sources
func NewDownloadManager(cfg *Config, workers int) (*DownloadManager, error) {
	cacheDir, err := config.DefaultCacheDir(configAdapter(cfg))
	if err != nil {
		return nil, err
	}

	sourceConfigs := getGemSources(cfg)
	client := defaultHTTPClient()

	return newDownloadManager(cacheDir, sourceConfigs, client, workers)
}

func getGemSources(cfg *Config) []SourceConfig {
	if cfg != nil && len(cfg.GemSources) > 0 {
		return cfg.GemSources
	}

	return []SourceConfig{
		{
			URL:      "https://rubygems.org",
			Fallback: "",
		},
	}
}

func configAdapter(c *Config) *config.Config {
	if c == nil {
		return nil
	}
	return &config.Config{
		VendorDir: c.VendorDir,
		CacheDir:  c.CacheDir,
		Gemfile:   c.Gemfile,
	}
}

func detectRubyVersion() string {
	return ruby.DetectRubyVersion(defaultLockfilePath(), defaultGemfilePath(), config.ToMajorMinor, DEFAULT_RUBY_VERSION)
}

func defaultLockfilePath() string {
	return config.DefaultLockfilePath()
}

func defaultGemfilePath() string {
	return config.DefaultGemfilePath(nil)
}
