package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/compactindex"
	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/ruby"
	"github.com/contriboss/ore-light/internal/sources"
	"golang.org/x/sync/errgroup"
)

type DownloadManager struct {
	cacheDir      string
	sourceManager *sources.Manager
	compactIndex  *compactindex.Client
	workers       int
	detectRuby    func() string
	checksumMu    sync.Mutex
	checksums     map[string]checksumEntry
}

type DownloadReport struct {
	Total      int
	Downloaded int
	Skipped    int
	mu         sync.Mutex
}

type checksumEntry struct {
	checksum string
	ok       bool
}

func newDownloadManager(cacheDir string, sourceConfigs []SourceConfig, client *http.Client, workers int, detectRuby func() string) (*DownloadManager, error) {
	if cacheDir == "" {
		return nil, fmt.Errorf("cache directory must be provided")
	}
	if len(sourceConfigs) == 0 {
		return nil, fmt.Errorf("at least one gem source must be configured")
	}
	if workers <= 0 {
		workers = config.DefaultDownloadWorkers(nil)
	}
	if client == nil {
		client = defaultHTTPClient(workers)
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	sourceManager := sources.NewManager(sourceConfigs, client)

	return &DownloadManager{
		cacheDir:      cacheDir,
		sourceManager: sourceManager,
		compactIndex:  newCompactIndexClient(sourceManager),
		workers:       workers,
		detectRuby:    detectRuby,
		checksums:     make(map[string]checksumEntry),
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
	expectedChecksum, _ := m.expectedChecksum(ctx, gem)
	if !force {
		if foundPath := m.findInCaches(gem); foundPath != "" {
			if foundPath == cachePath {
				ok, err := verifyCacheChecksum(cachePath, metaPath, expectedChecksum)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: checksum check failed for %s: %v\n", gem.FullName(), err)
				}
				if !ok {
					fmt.Fprintf(os.Stderr, "Warning: cached gem %s failed checksum; re-downloading\n", gem.FullName())
					_ = os.Remove(cachePath)
					_ = os.Remove(metaPath)
				} else {
					_ = ensureCacheChecksum(cachePath, metaPath)
					return false, nil
				}
			} else {
				sha, err := copyFile(foundPath, cachePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Note: Using %s from system cache (copy failed: %v)\n", gem.FullName(), err)
				} else if sha != "" {
					if expectedChecksum != "" && sha != expectedChecksum {
						fmt.Fprintf(os.Stderr, "Warning: cached gem %s failed checksum; re-downloading\n", gem.FullName())
						_ = os.Remove(cachePath)
						_ = os.Remove(metaPath)
					} else {
						_ = saveCacheMetadata(metaPath, cacheMetadata{SHA256: sha})
						return false, nil
					}
				}
			}
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
		ok, err := verifyCacheChecksum(cachePath, metaPath, expectedChecksum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: checksum check failed for %s: %v\n", gem.FullName(), err)
		}
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: cached gem %s failed checksum; re-downloading\n", gem.FullName())
			_ = os.Remove(cachePath)
			_ = os.Remove(metaPath)
			return m.downloadGem(ctx, gem, true)
		}
		_ = ensureCacheChecksum(cachePath, metaPath)
		return false, nil
	}

	if err := tempFile.Close(); err != nil {
		return false, fmt.Errorf("failed to close temp file for %s: %w", gem.FullName(), err)
	}

	if err := os.Rename(tempFile.Name(), cachePath); err != nil {
		return false, fmt.Errorf("failed to finalize download for %s: %w", gem.FullName(), err)
	}

	sha, shaErr := computeSHA256(cachePath)
	if shaErr != nil {
		if expectedChecksum != "" {
			return false, fmt.Errorf("failed to compute checksum for %s: %w", gem.FullName(), shaErr)
		}
		fmt.Fprintf(os.Stderr, "Warning: failed to compute checksum for %s: %v\n", gem.FullName(), shaErr)
	}
	if expectedChecksum != "" && sha != "" && sha != expectedChecksum {
		_ = os.Remove(cachePath)
		_ = os.Remove(metaPath)
		return false, fmt.Errorf("checksum mismatch for %s", gem.FullName())
	}

	if respHeaders != nil {
		meta := cacheMetadata{
			ETag:         respHeaders.Get("ETag"),
			LastModified: respHeaders.Get("Last-Modified"),
			SHA256:       sha,
		}
		_ = saveCacheMetadata(metaPath, meta)
	} else if sha != "" {
		_ = saveCacheMetadata(metaPath, cacheMetadata{SHA256: sha})
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

	if gemPaths := tryGetGemPaths(m.detectRuby); len(gemPaths) > 0 {
		for _, gemPath := range gemPaths {
			cacheDir := filepath.Join(gemPath, "cache")
			locations = append(locations, cacheDir)
		}
	}

	return locations
}

func tryGetGemPaths(detectRubyVersion func() string) []string {
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

func copyFile(src, dst string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}

	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = out.Close()
	}()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, hasher), in); err != nil {
		return "", err
	}

	if err := out.Close(); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (m *DownloadManager) CacheDir() string {
	return m.cacheDir
}

func gemFileName(gem lockfile.GemSpec) string {
	return fmt.Sprintf("%s.gem", gem.FullName())
}

func newCompactIndexClient(manager *sources.Manager) *compactindex.Client {
	if manager == nil {
		return nil
	}

	for _, source := range manager.GetSources() {
		if source.URL == "" {
			continue
		}
		client, err := compactindex.NewClient(source.URL)
		if err == nil {
			return client
		}
	}

	return nil
}

func (m *DownloadManager) expectedChecksum(ctx context.Context, gem lockfile.GemSpec) (string, bool) {
	if m.compactIndex == nil {
		return "", false
	}

	key := gem.FullName()

	m.checksumMu.Lock()
	entry, ok := m.checksums[key]
	m.checksumMu.Unlock()
	if ok {
		return entry.checksum, entry.ok
	}

	infoList, err := m.compactIndex.GetGemInfo(ctx, gem.Name)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "Warning: checksum lookup failed for %s: %v\n", gem.FullName(), err)
		}
		m.checksumMu.Lock()
		m.checksums[key] = checksumEntry{}
		m.checksumMu.Unlock()
		return "", false
	}

	checksum, found := compactindex.FindChecksum(infoList, gem.Version, gem.Platform)
	m.checksumMu.Lock()
	m.checksums[key] = checksumEntry{checksum: checksum, ok: found}
	m.checksumMu.Unlock()
	return checksum, found
}

type cacheMetadata struct {
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
	SHA256       string `json:"sha256"`
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

func ensureCacheChecksum(path, metaPath string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}

	meta, err := loadCacheMetadata(metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			meta = &cacheMetadata{}
		} else {
			return err
		}
	}

	if meta.SHA256 != "" {
		return nil
	}

	sha, err := computeSHA256(path)
	if err != nil {
		return err
	}
	meta.SHA256 = sha
	return saveCacheMetadata(metaPath, *meta)
}

func verifyCacheChecksum(path, metaPath, expectedChecksum string) (bool, error) {
	if expectedChecksum != "" {
		sha, err := computeSHA256(path)
		if err != nil {
			return true, err
		}
		return sha == expectedChecksum, nil
	}

	meta, err := loadCacheMetadata(metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return true, err
	}
	if meta.SHA256 == "" {
		return true, nil
	}
	sha, err := computeSHA256(path)
	if err != nil {
		return true, err
	}
	return sha == meta.SHA256, nil
}

func computeSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func defaultHTTPClient(workers int) *http.Client {
	maxConnsPerHost := clampInt(workers, 4, 32)
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxConnsPerHost * 4,
		MaxIdleConnsPerHost:   maxConnsPerHost,
		MaxConnsPerHost:       maxConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   150 * time.Second, // 2.5 minutes for large gems
		Transport: transport,
	}
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// NewDownloadManager creates a download manager with configured sources
func NewDownloadManager(cfg *Config, workers int) (*DownloadManager, error) {
	cacheDir, err := config.DefaultCacheDir(configAdapter(cfg))
	if err != nil {
		return nil, err
	}

	sourceConfigs := getGemSources(cfg)
	if workers <= 0 {
		workers = config.DefaultDownloadWorkers(configAdapter(cfg))
	}
	client := defaultHTTPClient(workers)

	return newDownloadManager(cacheDir, sourceConfigs, client, workers, makeDetectRubyVersion(cfg))
}

func getGemSources(cfg *Config) []SourceConfig {
	sources, _ := config.ResolveGemSources(configAdapter(cfg))
	return sources
}

func configAdapter(c *Config) *config.Config {
	return c
}

func makeDetectRubyVersion(cfg *Config) func() string {
	return func() string {
		return ruby.DetectRubyVersion(defaultLockfilePath(), defaultGemfilePath(cfg), config.ToMajorMinor, ruby.DefaultRubyVersion)
	}
}

func defaultLockfilePath() string {
	return config.DefaultLockfilePath()
}

func defaultGemfilePath(cfg *Config) string {
	return config.DefaultGemfilePath(configAdapter(cfg))
}
