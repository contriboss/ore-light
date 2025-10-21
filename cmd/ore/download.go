package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
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
		if _, err := os.Stat(cachePath); err == nil {
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

func (m *downloadManager) CacheDir() string {
	return m.cacheDir
}

func gemFileName(gem lockfile.GemSpec) string {
	return fmt.Sprintf("%s.gem", gem.FullName())
}
