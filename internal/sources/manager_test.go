package sources

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMirrorEnvOverridesPrimary(t *testing.T) {
	// Mirror returns success; primary would fail if hit.
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mirror"))
	}))
	defer mirror.Close()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primary.Close()

	t.Setenv("BUNDLE_MIRROR", mirror.URL)

	mgr := NewManager([]SourceConfig{{URL: primary.URL}}, nil)

	var buf bytes.Buffer
	status, _, err := mgr.DownloadGemWithHeaders(context.Background(), "test-0.0.1.gem", &buf, nil)
	if err != nil {
		t.Fatalf("download via mirror failed: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200 from mirror, got %d", status)
	}
	if got := buf.String(); got != "mirror" {
		t.Fatalf("expected mirror response body, got %q", got)
	}
}

func TestFallbackUsedOnRetryableError(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer primary.Close()

	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fallback"))
	}))
	defer fallback.Close()

	mgr := NewManager([]SourceConfig{{URL: primary.URL, Fallback: fallback.URL}}, nil)

	var buf bytes.Buffer
	status, _, err := mgr.DownloadGemWithHeaders(context.Background(), "test-0.0.1.gem", &buf, nil)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected fallback status 200, got %d", status)
	}
	if got := buf.String(); got != "fallback" {
		t.Fatalf("expected fallback response, got %q", got)
	}
}

func TestDownloadGemWithHeadersNotModified(t *testing.T) {
	etag := `"abc123"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()

	mgr := NewManager([]SourceConfig{{URL: server.URL}}, nil)

	var buf bytes.Buffer
	// First request to populate ETag
	status, headers, err := mgr.DownloadGemWithHeaders(context.Background(), "test-0.0.1.gem", &buf, nil)
	if err != nil {
		t.Fatalf("initial download failed: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	gotETag := headers.Get("ETag")
	if gotETag == "" {
		t.Fatalf("expected ETag header")
	}

	// Second request with If-None-Match should get 304 and not write body
	buf.Reset()
	status, _, err = mgr.DownloadGemWithHeaders(context.Background(), "test-0.0.1.gem", &buf, map[string]string{"If-None-Match": gotETag})
	if err != nil {
		t.Fatalf("conditional request failed: %v", err)
	}
	if status != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", status)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no body on 304, got %d bytes", buf.Len())
	}
}

// Ensure mirror mapping uses Bundler-style env var names.
func TestApplyMirrorBundlerEnv(t *testing.T) {
	t.Setenv("BUNDLE_MIRROR", "")
	t.Setenv("BUNDLE_MIRROR__example__com", "https://mirror.example.com")

	mirrors := resolveMirrorMap()
	if got := applyMirror("https://example.com", mirrors); got != "https://mirror.example.com" {
		t.Fatalf("expected mirror to be applied, got %s", got)
	}
	if _, ok := mirrors["*"]; ok {
		t.Fatalf("did not expect wildcard mirror when only host mirror set")
	}
}
