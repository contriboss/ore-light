package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/sources"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultVendorSubdir is the default subdirectory name under vendor/
	// Using "bundle" for Bundler compatibility
	DefaultVendorSubdir = "bundle"
)

// GetXDGCacheHome returns the XDG cache home directory
// Falls back to ~/.cache if XDG_CACHE_HOME is not set
func GetXDGCacheHome() (string, error) {
	if cache := os.Getenv("XDG_CACHE_HOME"); cache != "" {
		return cache, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return filepath.Join(home, ".cache"), nil
}

// GetXDGDataHome returns the XDG data home directory
// Falls back to ~/.local/share if XDG_DATA_HOME is not set
func GetXDGDataHome() (string, error) {
	if data := os.Getenv("XDG_DATA_HOME"); data != "" {
		return data, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

// Config represents the application configuration
type Config struct {
	VendorDir       string
	CacheDir        string
	GemSources      []sources.SourceConfig
	Gemfile         string
	DownloadWorkers int `toml:"download_workers"`
}

// DefaultLockfilePath returns the default lockfile path
//
// This function MUST handle the case where BUNDLE_GEMFILE is set but the
// lockfile doesn't exist yet (e.g., in appraisal workflows with unlocked deps).
// In such cases, it should return the EXPECTED lockfile path so ore can create it.
//
// The key issue: lockfile.FindLockfileOnly() is too strict - it requires the
// lockfile to exist and returns an error if it doesn't. But ore needs the path
// even when the file doesn't exist yet.
//
// Solution: Call lockfile.FindGemfiles() which respects BUNDLE_GEMFILE and
// returns the lockfile path without requiring it to exist (when BUNDLE_GEMFILE is set).
func DefaultLockfilePath() string {
	// Priority 1: Try FindGemfiles() - it respects BUNDLE_GEMFILE
	paths, err := lockfile.FindGemfiles()
	if err == nil {
		// Success! FindGemfiles found the Gemfile and computed the lockfile path
		// Note: The lockfile might not exist yet, but that's okay - ore can create it
		return paths.GemfileLock
	}

	// Priority 2: If FindGemfiles() failed but BUNDLE_GEMFILE is set,
	// derive the lockfile path manually (for edge cases / tests where Gemfile doesn't exist yet)
	if bundleGemfile := os.Getenv("BUNDLE_GEMFILE"); bundleGemfile != "" {
		dir := filepath.Dir(bundleGemfile)
		base := filepath.Base(bundleGemfile)

		// Determine lockfile name based on Gemfile name
		var lockfileName string
		if base == "gems.rb" {
			lockfileName = "gems.locked"
		} else {
			lockfileName = base + ".lock"
		}

		return filepath.Join(dir, lockfileName)
	}

	// Priority 3: Fallback to Gemfile.lock for backward compatibility
	// This only happens if FindGemfiles() fails completely (no Gemfile found at all)
	return "Gemfile.lock"
}

// DefaultGemfilePath returns the default Gemfile path
// Supports both Gemfile and gems.rb naming conventions
func DefaultGemfilePath(cfg *Config) string {
	path, _, _ := ResolveGemfilePath(cfg)
	return path
}

// DefaultCacheDir returns the default cache directory
func DefaultCacheDir(cfg *Config) (string, error) {
	path, _, err := ResolveCacheDir(cfg)
	return path, err
}

// DefaultVendorDir returns the default vendor directory
// It requires Ruby detection functions which will be moved to internal/ruby
func DefaultVendorDir(cfg *Config, detectRubyVersion func() string, getSystemGemDir func() string) string {
	path, _, _ := ResolveVendorDir(cfg, detectRubyVersion, getSystemGemDir)
	return path
}

// ResolveVendorDir returns the resolved vendor directory and its source.
func ResolveVendorDir(cfg *Config, detectRubyVersion func() string, getSystemGemDir func() string) (string, string, error) {
	// Priority 1: Bundler environment variable (BUNDLE_PATH)
	if bundlePath := os.Getenv("BUNDLE_PATH"); bundlePath != "" {
		if looksLikeGemHome(bundlePath) {
			return bundlePath, "env:BUNDLE_PATH", nil
		}
		rubyVersion := detectRubyVersion()
		if rubyVersion != "" {
			return filepath.Join(bundlePath, "ruby", rubyVersion), "env:BUNDLE_PATH", nil
		}
		return bundlePath, "env:BUNDLE_PATH", nil
	}

	// Priority 2: Ore config file
	if cfg != nil && cfg.VendorDir != "" {
		return cfg.VendorDir, "config:ore", nil
	}

	// Priority 3: Bundler .bundle/config file
	if bundlePath := ReadBundleConfigPath(); bundlePath != "" {
		if looksLikeGemHome(bundlePath) {
			return bundlePath, "bundle-config:BUNDLE_PATH", nil
		}
		rubyVersion := detectRubyVersion()
		if rubyVersion != "" {
			return filepath.Join(bundlePath, "ruby", rubyVersion), "bundle-config:BUNDLE_PATH", nil
		}
		return bundlePath, "bundle-config:BUNDLE_PATH", nil
	}

	// Priority 4: System gem directory (default - like `gem install`)
	return getSystemGemDir(), "system", nil
}

func looksLikeGemHome(path string) bool {
	expectedDirs := []string{
		"specifications",
		"gems",
		"doc",
		"extensions",
		"build_info",
		"cache",
	}

	matchCount := 0
	for _, name := range expectedDirs {
		if _, err := os.Stat(filepath.Join(path, name)); err == nil {
			matchCount++
			if matchCount >= 2 {
				return true
			}
		}
	}

	return false
}

// ResolveCacheDir returns the resolved cache directory and its source.
func ResolveCacheDir(cfg *Config) (string, string, error) {
	// Priority 1: BUNDLE_CACHE_PATH (Bundler environment variable)
	if cache := os.Getenv("BUNDLE_CACHE_PATH"); cache != "" {
		return cache, "env:BUNDLE_CACHE_PATH", nil
	}

	// Priority 2: Ore config file
	if cfg != nil && cfg.CacheDir != "" {
		return cfg.CacheDir, "config:ore", nil
	}

	// Priority 3: XDG cache home
	cacheHome, err := GetXDGCacheHome()
	if err != nil {
		return "", "xdg:cache", err
	}

	return filepath.Join(cacheHome, "ore", "gems"), "xdg:cache", nil
}

// ResolveGemfilePath returns the resolved Gemfile path and its source.
func ResolveGemfilePath(cfg *Config) (string, string, error) {
	// Priority 1: BUNDLE_GEMFILE (standard Bundler environment variable)
	// This is set by appraisal and other Bundler-based tools
	if env := os.Getenv("BUNDLE_GEMFILE"); env != "" {
		return env, "env:BUNDLE_GEMFILE", nil
	}

	// Priority 2: Ore config file
	if cfg != nil && cfg.Gemfile != "" {
		return cfg.Gemfile, "config:ore", nil
	}

	// Priority 3: Auto-detect gems.rb (newer Bundler convention)
	if _, err := os.Stat("gems.rb"); err == nil {
		return "gems.rb", "file:gems.rb", nil
	}

	// Priority 4: Default to Gemfile
	return "Gemfile", "default:Gemfile", nil
}

// ResolveGemSources returns configured gem sources or the default source.
func ResolveGemSources(cfg *Config) ([]sources.SourceConfig, string) {
	if cfg != nil && len(cfg.GemSources) > 0 {
		return cfg.GemSources, "config:ore"
	}

	return []sources.SourceConfig{
		{
			URL:      "https://rubygems.org",
			Fallback: "",
		},
	}, "default"
}

// ReadBundleConfigPath reads the BUNDLE_PATH from .bundle/config
func ReadBundleConfigPath() string {
	data, err := os.ReadFile(".bundle/config")
	if err != nil {
		return ""
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return ""
	}

	if path, ok := config["BUNDLE_PATH"].(string); ok {
		return path
	}

	return ""
}

// WriteBundleConfig writes a .bundle/config file with the given path
// This makes ore compatible with Bundler's configuration system
func WriteBundleConfig(bundlePath string) error {
	// Create .bundle directory if it doesn't exist
	if err := os.MkdirAll(".bundle", 0755); err != nil {
		return fmt.Errorf("failed to create .bundle directory: %w", err)
	}

	// Create YAML config
	config := map[string]string{
		"BUNDLE_PATH": bundlePath,
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to .bundle/config
	if err := os.WriteFile(".bundle/config", data, 0644); err != nil {
		return fmt.Errorf("failed to write .bundle/config: %w", err)
	}

	return nil
}

// ToMajorMinor converts "3.4.7" to "3.4.0" (Bundler convention)
// Handles: "3.4.7" -> "3.4.0", "3.1" -> "3.1.0", "3" -> "3.0.0"
func ToMajorMinor(version string) string {
	parts := []string{}
	current := ""
	for i := 0; i < len(version); i++ {
		if version[i] == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(version[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	// Always return major.minor.0
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1] + ".0"
	} else if len(parts) == 1 {
		return parts[0] + ".0.0"
	}
	return version
}
