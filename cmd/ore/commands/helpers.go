package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
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

// defaultCacheDir returns the default cache directory
func defaultCacheDir() (string, error) {
	return config.DefaultCacheDir(nil)
}

// defaultLockfilePath returns the default lockfile path
func defaultLockfilePath() string {
	return config.DefaultLockfilePath()
}

// loadLockfile loads and parses a lockfile
func loadLockfile(lockfilePath string) (*lockfile.Lockfile, error) {
	file, err := os.Open(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open lockfile: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	parsed, err := lockfile.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lockfile: %w", err)
	}

	return parsed, nil
}

// loadGemSpecs loads gem specs from a lockfile
func loadGemSpecs(lockfilePath string) ([]lockfile.GemSpec, error) {
	parsed, err := loadLockfile(lockfilePath)
	if err != nil {
		return nil, err
	}

	return deduplicateGemSpecs(parsed.GemSpecs), nil
}

// deduplicateGemSpecs removes duplicate gem specs
func deduplicateGemSpecs(specs []lockfile.GemSpec) []lockfile.GemSpec {
	if len(specs) == 0 {
		return specs
	}

	seen := make(map[string]bool)
	result := make([]lockfile.GemSpec, 0, len(specs))

	for _, spec := range specs {
		key := spec.FullName()
		if !seen[key] {
			seen[key] = true
			result = append(result, spec)
		}
	}

	return result
}

// detectGemfileFromLock infers the Gemfile path from a lockfile path
func detectGemfileFromLock(lockfilePath string) string {
	if lockfilePath == "" {
		lockfilePath = "Gemfile.lock"
	}

	// Handle gems.locked -> gems.rb
	if strings.HasSuffix(lockfilePath, "gems.locked") {
		candidate := strings.TrimSuffix(lockfilePath, ".locked") + ".rb"
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		return ""
	}

	// Handle Gemfile.lock -> Gemfile (and other .lock files)
	if !strings.HasSuffix(lockfilePath, ".lock") {
		return ""
	}
	candidate := strings.TrimSuffix(lockfilePath, ".lock")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// enrichGemsWithGroups reads the Gemfile and enriches lockfile gems with group information
func enrichGemsWithGroups(gemfilePath string, parsed *lockfile.Lockfile) error {
	parser := gemfile.NewGemfileParser(gemfilePath)
	parsedGemfile, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", err)
	}

	// Create a map of gem name -> groups from the Gemfile
	gemGroups := make(map[string][]string)
	for _, dep := range parsedGemfile.Dependencies {
		if len(dep.Groups) > 0 {
			gemGroups[dep.Name] = dep.Groups
		} else {
			gemGroups[dep.Name] = []string{"default"}
		}
	}

	// Enrich GemSpecs with group information
	for i := range parsed.GemSpecs {
		if groups, found := gemGroups[parsed.GemSpecs[i].Name]; found {
			parsed.GemSpecs[i].Groups = groups
		}
	}

	// Enrich GitGemSpecs with group information
	for i := range parsed.GitSpecs {
		if groups, found := gemGroups[parsed.GitSpecs[i].Name]; found {
			parsed.GitSpecs[i].Groups = groups
		}
	}

	// Enrich PathGemSpecs with group information
	for i := range parsed.PathSpecs {
		if groups, found := gemGroups[parsed.PathSpecs[i].Name]; found {
			parsed.PathSpecs[i].Groups = groups
		}
	}

	return nil
}

// platformMatches checks if a gem platform matches the current platform
func platformMatches(gemPlatform, currentPlatform string) bool {
	// Exact match
	if gemPlatform == currentPlatform {
		return true
	}

	// Platform variants - extract base platform components
	// Examples: arm64-darwin-24 matches arm64-darwin
	//           x86_64-linux-gnu matches x86_64-linux
	gemParts := strings.Split(gemPlatform, "-")
	currentParts := strings.Split(currentPlatform, "-")

	// Need at least arch-os
	if len(gemParts) < 2 || len(currentParts) < 2 {
		return false
	}

	// Match arch and os (first two components)
	return gemParts[0] == currentParts[0] && gemParts[1] == currentParts[1]
}
