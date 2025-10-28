package commands

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultGemfilePath returns the path to the Gemfile to use.
// Supports both Gemfile and gems.rb naming conventions.
//
// Priority:
// 1. ORE_GEMFILE environment variable
// 2. gems.rb (if exists)
// 3. Gemfile (default)
func defaultGemfilePath() string {
	if env := os.Getenv("ORE_GEMFILE"); env != "" {
		return env
	}

	// Check for gems.rb first (newer Bundler 2.0+ convention)
	if _, err := os.Stat("gems.rb"); err == nil {
		return "gems.rb"
	}

	// Default to Gemfile
	return "Gemfile"
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
	if base == "gems.rb" {
		fallbackName = "Gemfile.lock"
	} else if base == "Gemfile" {
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
