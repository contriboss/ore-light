package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/gemfile-go/lockfile"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultVendorSubdir is the default subdirectory name under vendor/
	// Using "bundle" for Bundler compatibility
	DefaultVendorSubdir = "bundle"
)

// Config represents the application configuration
type Config struct {
	VendorDir string
	CacheDir  string
	Gemfile   string
}

// DefaultLockfilePath returns the default lockfile path
func DefaultLockfilePath() string {
	// Try to auto-detect Gemfile.lock or gems.locked
	// This respects BUNDLE_GEMFILE if set
	lockPath, err := lockfile.FindLockfileOnly()
	if err == nil {
		return lockPath
	}

	// Fallback to Gemfile.lock for backward compatibility
	return "Gemfile.lock"
}

// DefaultGemfilePath returns the default Gemfile path
// Supports both Gemfile and gems.rb naming conventions
func DefaultGemfilePath(cfg *Config) string {
	if env := os.Getenv("ORE_GEMFILE"); env != "" {
		return env
	}
	if cfg != nil && cfg.Gemfile != "" {
		return cfg.Gemfile
	}

	// Check for gems.rb first (newer Bundler 2.0+ convention)
	if _, err := os.Stat("gems.rb"); err == nil {
		return "gems.rb"
	}

	// Default to Gemfile
	return "Gemfile"
}

// DefaultCacheDir returns the default cache directory
func DefaultCacheDir(cfg *Config) (string, error) {
	if cache := os.Getenv("ORE_CACHE_DIR"); cache != "" {
		return cache, nil
	}
	if cache := os.Getenv("ORE_LIGHT_CACHE_DIR"); cache != "" {
		return cache, nil
	}
	if cfg != nil && cfg.CacheDir != "" {
		return cfg.CacheDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}

	return filepath.Join(home, ".cache", "ore", "gems"), nil
}

// DefaultVendorDir returns the default vendor directory
// It requires Ruby detection functions which will be moved to internal/ruby
func DefaultVendorDir(cfg *Config, detectRubyVersion func() string, getSystemGemDir func() string) string {
	// Priority 1: Environment variables
	if env := os.Getenv("ORE_VENDOR_DIR"); env != "" {
		return env
	}
	if env := os.Getenv("ORE_LIGHT_VENDOR_DIR"); env != "" {
		return env
	}

	// Priority 2: Ore config file
	if cfg != nil && cfg.VendorDir != "" {
		return cfg.VendorDir
	}

	// Priority 3: Bundler .bundle/config
	if bundlePath := ReadBundleConfigPath(); bundlePath != "" {
		rubyVersion := detectRubyVersion()
		if rubyVersion != "" {
			return filepath.Join(bundlePath, "ruby", rubyVersion)
		}
		return bundlePath
	}

	// Priority 4: System gem directory (default - like `gem install`)
	// This makes ore behave like gem install by default (no isolation)
	// Users can set BUNDLE_PATH or use --path flag for isolated installs
	return getSystemGemDir()
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
