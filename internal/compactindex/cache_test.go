package compactindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetBundlerCachePath(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		wantContain string
		wantErr     bool
	}{
		{
			name:        "https rubygems.org",
			baseURL:     "https://rubygems.org",
			wantContain: "rubygems.org.443.",
			wantErr:     false,
		},
		{
			name:        "http rubygems.org",
			baseURL:     "http://rubygems.org",
			wantContain: "rubygems.org.80.",
			wantErr:     false,
		},
		{
			name:        "custom port",
			baseURL:     "https://gems.example.com:9292",
			wantContain: "gems.example.com.9292.",
			wantErr:     false,
		},
		{
			name:    "invalid URL",
			baseURL: "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetBundlerCachePath(tt.baseURL)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetBundlerCachePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Check that it contains expected parts
			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("GetBundlerCachePath() = %v, want to contain %v", got, tt.wantContain)
			}

			// Check that it contains .bundle/cache/compact_index
			if !strings.Contains(got, filepath.Join(".bundle", "cache", "compact_index")) {
				t.Errorf("GetBundlerCachePath() = %v, want to contain .bundle/cache/compact_index", got)
			}

			// Check that it starts with home directory
			homeDir, _ := os.UserHomeDir()
			if !strings.HasPrefix(got, homeDir) {
				t.Errorf("GetBundlerCachePath() = %v, want to start with %v", got, homeDir)
			}
		})
	}
}

func TestGetInfoPath(t *testing.T) {
	cacheDir := "/tmp/test-cache"

	tests := []struct {
		name        string
		gemName     string
		wantContain string
		wantDir     string
	}{
		{
			name:        "standard name",
			gemName:     "rails",
			wantContain: "info/rails",
			wantDir:     "info",
		},
		{
			name:        "name with hyphen",
			gemName:     "action-pack",
			wantContain: "info/action-pack",
			wantDir:     "info",
		},
		{
			name:        "name with underscore",
			gemName:     "active_record",
			wantContain: "info/active_record",
			wantDir:     "info",
		},
		{
			name:        "name with hyphen (valid)",
			gemName:     "jquery-ui",
			wantContain: "info/jquery-ui",
			wantDir:     "info",
		},
		{
			name:        "name with dot",
			gemName:     "rack.attack",
			wantContain: "info-special-characters/rack.attack-",
			wantDir:     "info-special-characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetInfoPath(cacheDir, tt.gemName)

			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("GetInfoPath() = %v, want to contain %v", got, tt.wantContain)
			}

			if !strings.Contains(got, tt.wantDir) {
				t.Errorf("GetInfoPath() = %v, want to be in %v directory", got, tt.wantDir)
			}
		})
	}
}

func TestEnsureCacheDirectories(t *testing.T) {
	// Create a temp directory for testing
	tempDir := t.TempDir()

	err := EnsureCacheDirectories(tempDir)
	if err != nil {
		t.Fatalf("EnsureCacheDirectories() error = %v", err)
	}

	// Check that directories were created
	dirs := []string{
		"info",
		"info-special-characters",
	}

	for _, dir := range dirs {
		path := filepath.Join(tempDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}
