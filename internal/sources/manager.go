package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
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

type retryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
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
	retry        retryPolicy
	randMu       sync.Mutex
	rand         *rand.Rand
}

const (
	defaultRetryMaxAttempts = 3
	defaultRetryBaseDelay   = 200 * time.Millisecond
	defaultRetryMaxDelay    = 2 * time.Second
)

func defaultRetryPolicy() retryPolicy {
	return retryPolicy{
		MaxAttempts: defaultRetryMaxAttempts,
		BaseDelay:   defaultRetryBaseDelay,
		MaxDelay:    defaultRetryMaxDelay,
	}
}

// NewManager creates a new source manager
func NewManager(sourceConfigs []SourceConfig, client *http.Client) *Manager {
	if client == nil {
		client = &http.Client{
			Timeout:   30 * time.Second,
			Transport: defaultHTTPTransport(16),
		}
	}

	mirrors := resolveMirrorMap()

	sources := make([]*Source, 0, len(sourceConfigs))
	for _, config := range sourceConfigs {
		primary := applyMirror(config.URL, mirrors)
		fallback := config.Fallback
		// If we swapped to a mirror, fall back to the original unless user provided one
		if primary != config.URL && fallback == "" {
			fallback = config.URL
		}
		sources = append(sources, NewSource(primary, fallback))
	}

	return &Manager{
		sources:      sources,
		client:       client,
		healthStatus: make(map[string]bool),
		retry:        defaultRetryPolicy(),
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SourceConfig represents a source configuration
type SourceConfig struct {
	URL      string `toml:"url"`
	Fallback string `toml:"fallback,omitempty"`
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
	_, _, err := m.DownloadGemWithHeaders(ctx, gemName, writer, nil)
	return err
}

// DownloadGemWithHeaders downloads a gem and allows conditional headers.
// Returns the HTTP status code and response headers (for caching/ETag handling).
func (m *Manager) DownloadGemWithHeaders(ctx context.Context, gemName string, writer io.Writer, headers map[string]string) (int, http.Header, error) {
	if len(m.sources) == 0 {
		return 0, nil, errors.New("no gem sources configured")
	}

	var lastErr error

	for _, source := range m.sources {
		// Try primary source
		downloadURL := fmt.Sprintf("%s/downloads/%s", source.URL, gemName)
		status, respHeaders, err := m.downloadWithRetry(ctx, downloadURL, source.auth, writer, headers)

		if err == nil {
			return status, respHeaders, nil // Success or not-modified
		}

		lastErr = err

		// Check if error is retryable and we have a fallback
		if isRetryableError(err) && source.FallbackURL != "" {
			fallbackURL := fmt.Sprintf("%s/downloads/%s", source.FallbackURL, gemName)
			fmt.Printf("Primary source %s failed, trying fallback %s\n", source.URL, source.FallbackURL)

			status, respHeaders, err = m.downloadWithRetry(ctx, fallbackURL, source.fallbackAuth, writer, headers)
			if err == nil {
				return status, respHeaders, nil // Fallback succeeded
			}
			lastErr = err
		}

		// If error is not retryable (404, auth failure), stop trying other sources
		if !isRetryableError(err) {
			return 0, nil, err
		}
	}

	if lastErr != nil {
		return 0, nil, fmt.Errorf("all sources failed: %w", lastErr)
	}

	return 0, nil, errors.New("no sources available")
}

func (m *Manager) downloadWithRetry(ctx context.Context, url string, auth *Authentication, writer io.Writer, headers map[string]string) (int, http.Header, error) {
	attempts := m.retry.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			if err := m.sleepWithBackoff(ctx, attempt-1); err != nil {
				return 0, nil, err
			}
		}

		status, respHeaders, written, err := m.download(ctx, url, auth, writer, headers)
		if err == nil {
			return status, respHeaders, nil
		}

		lastErr = err

		if written > 0 {
			if !resetWriter(writer) {
				return 0, nil, &partialWriteError{err: err}
			}
		}

		if !isRetryableError(err) {
			return 0, nil, err
		}
	}

	if lastErr != nil {
		return 0, nil, lastErr
	}
	return 0, nil, errors.New("download failed")
}

func (m *Manager) download(ctx context.Context, url string, auth *Authentication, writer io.Writer, headers map[string]string) (int, http.Header, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if present
	if auth != nil {
		if auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		} else if auth.Username != "" {
			req.SetBasicAuth(auth.Username, auth.Password)
		}
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return 0, nil, 0, fmt.Errorf("network error: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotModified {
		return resp.StatusCode, resp.Header.Clone(), 0, nil
	}

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, resp.Header.Clone(), 0, &HTTPError{StatusCode: resp.StatusCode, URL: url}
	}

	counting := &countingWriter{writer: writer}
	_, err = io.Copy(counting, resp.Body)
	return resp.StatusCode, resp.Header.Clone(), counting.n, err
}

type countingWriter struct {
	writer io.Writer
	n      int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.writer.Write(p)
	c.n += int64(n)
	return n, err
}

type resettableWriter interface {
	io.Seeker
	Truncate(size int64) error
}

func resetWriter(writer io.Writer) bool {
	resettable, ok := writer.(resettableWriter)
	if !ok {
		return false
	}
	if err := resettable.Truncate(0); err != nil {
		return false
	}
	if _, err := resettable.Seek(0, io.SeekStart); err != nil {
		return false
	}
	return true
}

func (m *Manager) sleepWithBackoff(ctx context.Context, attempt int) error {
	delay := m.retry.BaseDelay
	if delay <= 0 {
		delay = defaultRetryBaseDelay
	}
	if attempt > 1 {
		delay = delay * time.Duration(1<<uint(attempt-1))
	}
	if m.retry.MaxDelay > 0 && delay > m.retry.MaxDelay {
		delay = m.retry.MaxDelay
	}
	delay = m.jitterDelay(delay)
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (m *Manager) jitterDelay(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	minDelay := delay / 2
	if minDelay <= 0 {
		return delay
	}
	jitterRange := delay - minDelay
	if jitterRange <= 0 {
		return delay
	}

	m.randMu.Lock()
	defer m.randMu.Unlock()
	if m.rand == nil {
		return delay
	}
	jitter := time.Duration(m.rand.Int63n(int64(jitterRange) + 1))
	return minDelay + jitter
}

type partialWriteError struct {
	err error
}

func (e *partialWriteError) Error() string {
	return e.err.Error()
}

func (e *partialWriteError) Unwrap() error {
	return e.err
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

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var partial *partialWriteError
	if errors.As(err, &partial) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
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
		case http.StatusRequestTimeout,
			http.StatusInternalServerError,
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

func defaultHTTPTransport(maxConnsPerHost int) *http.Transport {
	maxConnsPerHost = clampInt(maxConnsPerHost, 4, 32)
	return &http.Transport{
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

// resolveMirrorMap builds a map of host -> mirror URL from environment variables.
// Supports:
//   - BUNDLE_MIRROR (applies to all sources)
//   - BUNDLE_MIRROR__<host> (Bundler-style mirror env, where dots are written as "__")
func resolveMirrorMap() map[string]string {
	mirrors := make(map[string]string)

	if all := strings.TrimSpace(os.Getenv("BUNDLE_MIRROR")); all != "" {
		mirrors["*"] = strings.TrimSuffix(all, "/")
	}

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "BUNDLE_MIRROR__") {
			continue
		}
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimPrefix(parts[0], "BUNDLE_MIRROR__")
		val := strings.TrimSpace(parts[1])
		if val == "" {
			continue
		}
		host := strings.ReplaceAll(key, "__", ".")
		mirrors[host] = strings.TrimSuffix(val, "/")
	}

	return mirrors
}

// applyMirror returns a mirrored URL if configured.
func applyMirror(sourceURL string, mirrors map[string]string) string {
	if len(mirrors) == 0 {
		return sourceURL
	}
	if all, ok := mirrors["*"]; ok {
		return all
	}

	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return sourceURL
	}
	host := parsed.Hostname()
	if mirror, ok := mirrors[host]; ok {
		return mirror
	}
	return sourceURL
}
