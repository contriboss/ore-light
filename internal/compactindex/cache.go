package compactindex

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

// GetBundlerCachePath computes the Bundler-compatible cache directory path
// for a given gem server URL.
//
// Bundler's algorithm:
// ~/.bundle/cache/compact_index/{host}.{port}.{md5(url)[0:32]}
func GetBundlerCachePath(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", baseURL, err)
	}

	// Compute MD5 hash of the full URL string
	hash := md5.Sum([]byte(u.String()))
	hexHash := fmt.Sprintf("%x", hash)

	// Get port (default to 443 for HTTPS, 80 for HTTP)
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// Server slug: {host}.{port}.{md5}
	serverSlug := fmt.Sprintf("%s.%s.%s", u.Hostname(), port, hexHash)

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Full cache path
	cachePath := filepath.Join(homeDir, ".bundle", "cache", "compact_index", serverSlug)

	return cachePath, nil
}

// GetInfoPath returns the path to the info file for a given gem name.
// Handles special characters by appending MD5 hash and using special directory.
//
// Bundler's rules:
// - Standard names (lowercase alphanumeric, hyphen, underscore): info/{name}
// - Special chars (anything else): info-special-characters/{name}-{md5(name)}
func GetInfoPath(cacheDir, gemName string) string {
	// Check if name contains special characters (anything except lowercase alphanumeric, hyphen, underscore)
	hasSpecialChars := regexp.MustCompile(`[^a-z0-9\-_]`).MatchString(gemName)

	if hasSpecialChars {
		// Compute MD5 hash of gem name
		hash := md5.Sum([]byte(gemName))
		hexHash := fmt.Sprintf("%x", hash)

		// Use info-special-characters directory
		return filepath.Join(cacheDir, "info-special-characters", fmt.Sprintf("%s-%s", gemName, hexHash))
	}

	// Standard info directory
	return filepath.Join(cacheDir, "info", gemName)
}

// EnsureCacheDirectories creates the necessary cache directory structure.
func EnsureCacheDirectories(cacheDir string) error {
	// Create main cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create info directory
	infoDir := filepath.Join(cacheDir, "info")
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return fmt.Errorf("failed to create info directory: %w", err)
	}

	// Create info-special-characters directory
	specialDir := filepath.Join(cacheDir, "info-special-characters")
	if err := os.MkdirAll(specialDir, 0755); err != nil {
		return fmt.Errorf("failed to create info-special-characters directory: %w", err)
	}

	return nil
}
