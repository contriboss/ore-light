package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Authentication holds authentication information extracted from URLs
type Authentication struct {
	Username string
	Password string
	Token    string
}

// Header returns the appropriate Authorization header value
func (a *Authentication) Header() string {
	if a.Token != "" {
		return "Bearer " + a.Token
	}
	// For basic auth, the client will handle it via URL
	return ""
}

// Source represents a gem source with optional fallback
type Source struct {
	URL          string
	FallbackURL  string
	auth         *Authentication
	fallbackAuth *Authentication
}

// extractAuth extracts authentication from URL and returns clean URL and auth
func extractAuth(sourceURL string) (string, *Authentication) {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return sourceURL, nil
	}

	if parsed.User == nil {
		return sourceURL, nil
	}

	auth := &Authentication{}
	username := parsed.User.Username()
	password, hasPassword := parsed.User.Password()

	// Check if it's token auth (token:@ or token:x-oauth-basic@)
	if username != "" && (!hasPassword || password == "" || password == "x-oauth-basic") {
		auth.Token = username
	} else {
		auth.Username = username
		auth.Password = password
	}

	// Remove auth from URL
	parsed.User = nil
	return parsed.String(), auth
}

// NewSource creates a new Source with authentication extraction
func NewSource(url, fallback string) *Source {
	cleanURL, auth := extractAuth(url)
	cleanFallback, fallbackAuth := extractAuth(fallback)

	return &Source{
		URL:          cleanURL,
		FallbackURL:  cleanFallback,
		auth:         auth,
		fallbackAuth: fallbackAuth,
	}
}

// Manager manages multiple gem sources with fallback support
type Manager struct {
	sources      []*Source
	client       *http.Client
	healthStatus map[string]bool
	mu           sync.RWMutex
}

// NewManager creates a new source manager
func NewManager(sourceConfigs []SourceConfig, client *http.Client) *Manager {
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	sources := make([]*Source, 0, len(sourceConfigs))
	for _, config := range sourceConfigs {
		sources = append(sources, NewSource(config.URL, config.Fallback))
	}

	return &Manager{
		sources:      sources,
		client:       client,
		healthStatus: make(map[string]bool),
	}
}

// SourceConfig represents a source configuration
type SourceConfig struct {
	URL      string
	Fallback string
}

// CheckHealth performs pre-flight health checks on all sources
func (m *Manager) CheckHealth(ctx context.Context) {
	var wg sync.WaitGroup

	checkSource := func(url string) {
		if url == "" {
			return
		}

		wg.Go(func() {
			// Try to fetch a small gem to test the source
			// Using rake as it's commonly available
			testURL := fmt.Sprintf("%s/downloads/rake-13.0.6.gem", url)
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, testURL, nil)
			if err != nil {
				m.setHealthStatus(url, false)
				return
			}

			resp, err := m.client.Do(req)
			if err != nil {
				m.setHealthStatus(url, false)
				return
			}
			_ = resp.Body.Close()

			// Consider 200 or 404 as healthy (404 means source works, gem doesn't exist)
			healthy := resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound
			m.setHealthStatus(url, healthy)
		})
	}

	// Check all sources and their fallbacks
	for _, source := range m.sources {
		checkSource(source.URL)
		checkSource(source.FallbackURL)
	}

	wg.Wait()
}

func (m *Manager) setHealthStatus(url string, healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthStatus[url] = healthy
}

func (m *Manager) isHealthy(url string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, exists := m.healthStatus[url]
	return !exists || status // Default to healthy if not checked
}

// DownloadGem downloads a gem from configured sources with fallback
func (m *Manager) DownloadGem(ctx context.Context, gemName string, writer io.Writer) error {
	if len(m.sources) == 0 {
		return errors.New("no gem sources configured")
	}

	var lastErr error

	for _, source := range m.sources {
		// Try primary source
		downloadURL := fmt.Sprintf("%s/downloads/%s", source.URL, gemName)
		err := m.download(ctx, downloadURL, source.auth, writer)

		if err == nil {
			return nil // Success!
		}

		lastErr = err

		// Check if error is retryable and we have a fallback
		if isRetryableError(err) && source.FallbackURL != "" {
			fallbackURL := fmt.Sprintf("%s/downloads/%s", source.FallbackURL, gemName)
			fmt.Printf("Primary source %s failed, trying fallback %s\n", source.URL, source.FallbackURL)

			err = m.download(ctx, fallbackURL, source.fallbackAuth, writer)
			if err == nil {
				return nil // Fallback succeeded!
			}
			lastErr = err
		}

		// If error is not retryable (404, auth failure), stop trying other sources
		if !isRetryableError(err) {
			return err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("all sources failed: %w", lastErr)
	}

	return errors.New("no sources available")
}

func (m *Manager) download(ctx context.Context, url string, auth *Authentication, writer io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if present
	if auth != nil {
		if auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		} else if auth.Username != "" {
			req.SetBasicAuth(auth.Username, auth.Password)
		}
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{StatusCode: resp.StatusCode, URL: url}
	}

	_, err = io.Copy(writer, resp.Body)
	return err
}

// HTTPError represents an HTTP error response
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d from %s", e.StatusCode, e.URL)
}

// isRetryableError determines if an error should trigger a fallback
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are retryable
	if strings.Contains(err.Error(), "network error") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "timeout") {
		return true
	}

	// Check HTTP errors
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
			http.StatusTooManyRequests:
			return true
		case http.StatusNotFound,
			http.StatusUnauthorized,
			http.StatusForbidden:
			return false // These are not retryable
		default:
			return httpErr.StatusCode >= 500 // All 5xx are retryable
		}
	}

	return false
}

// GetSources returns all configured sources for display/debugging
func (m *Manager) GetSources() []SourceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]SourceInfo, 0, len(m.sources))
	for _, source := range m.sources {
		info := SourceInfo{
			URL:      source.URL,
			Fallback: source.FallbackURL,
			Healthy:  m.isHealthy(source.URL),
		}
		if source.FallbackURL != "" {
			info.FallbackHealthy = m.isHealthy(source.FallbackURL)
		}
		infos = append(infos, info)
	}
	return infos
}

// SourceInfo provides information about a configured source
type SourceInfo struct {
	URL             string
	Fallback        string
	Healthy         bool
	FallbackHealthy bool
}
