package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/ruby"
)

// defaultGemfilePath returns the path to the Gemfile to use.
// Supports both Gemfile and gems.rb naming conventions.
//
// Priority:
// 1. ORE_GEMFILE environment variable
// 2. gems.rb (if exists)
// 3. Gemfile (default)
func defaultGemfilePath() string {
	return config.DefaultGemfilePath(nil)
}

// findLockfilePath finds the lockfile for a given Gemfile path.
// Supports both Gemfile.lock/gems.locked naming conventions.
//
// Ruby developers: This handles both the traditional Gemfile/Gemfile.lock
// and the newer gems.rb/gems.locked naming conventions (introduced in Bundler 2.0+)
func findLockfilePath(gemfilePath string) (string, error) {
	dir := filepath.Dir(gemfilePath)
	base := filepath.Base(gemfilePath)

	// Determine lockfile name based on Gemfile name
	var lockfileName string
	if base == "gems.rb" {
		lockfileName = "gems.locked"
	} else {
		lockfileName = base + ".lock"
	}

	lockfilePath := filepath.Join(dir, lockfileName)

	// Check if it exists
	if _, err := os.Stat(lockfilePath); err == nil {
		return lockfilePath, nil
	}

	// Fallback: try the other convention
	var fallbackName string
	switch base {
	case "gems.rb":
		fallbackName = "Gemfile.lock"
	case "Gemfile":
		fallbackName = "gems.locked"
	}

	if fallbackName != "" {
		fallbackPath := filepath.Join(dir, fallbackName)
		if _, err := os.Stat(fallbackPath); err == nil {
			return fallbackPath, nil
		}
	}

	return "", fmt.Errorf("no lockfile found for %s (looked for %s)", gemfilePath, lockfileName)
}

// defaultVendorDir returns the default vendor directory
// Respects project configuration and detects version managers (mise/asdf/rbenv)
//
// Priority:
// 1. ORE_VENDOR_DIR or ORE_LIGHT_VENDOR_DIR environment variable
// 2. BUNDLE_PATH environment variable
// 3. .bundle/config BUNDLE_PATH
// 4. System gem directory (with version manager detection)
func defaultVendorDir() string {
	// Note: In the commands package, we don't have access to appConfig from main
	// So we pass nil and let DefaultVendorDir handle env vars and .bundle/config
	return config.DefaultVendorDir(nil, getRubyVersion, getSystemGemDir)
}

// getRubyVersion detects the Ruby version to use (wrapper for ruby package)
func getRubyVersion() string {
	lockfilePath := config.DefaultLockfilePath()
	gemfilePath := defaultGemfilePath()
	return ruby.DetectRubyVersion(lockfilePath, gemfilePath, config.ToMajorMinor, "3.0")
}

// getSystemGemDir returns the system gem directory with version manager detection
func getSystemGemDir() string {
	return ruby.GetSystemGemDir(getRubyVersion)
}
