package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/extensions"
)

// TestPlatformSpecificGemInstallation verifies that platform-specific precompiled gems
// are properly installed and not incorrectly skipped.
//
// Bug: ore-light was not installing platform-specific precompiled gems like
// nokogiri-1.19.0-arm64-darwin even though they were correctly resolved in the lockfile.
// The install command reported "1 skipped" but the gem was never actually installed.
//
// Root cause: The installation logic was incorrectly treating platform-specific gems
// as incompatible or already installed.
func TestPlatformSpecificGemInstallation(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	vendorDir := filepath.Join(tmpDir, "vendor")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Test platform-specific precompiled gem (matches Ruby 4.0.1 on arm64-darwin)
	platformGem := lockfile.GemSpec{
		Name:       "nokogiri",
		Version:    "1.19.0",
		Platform:   "arm64-darwin",
		Extensions: []string{}, // Precompiled gems have NO extensions to build
	}

	// Create fake .gem file with metadata indicating precompiled gem
	gemFileName := platformGem.FullName() + ".gem"
	cachedGemPath := filepath.Join(cacheDir, gemFileName)

	// Precompiled gem metadata - note extensions array is empty
	metadataYAML := []byte(`---
name: nokogiri
version: !ruby/object:Gem::Version
  version: 1.19.0
platform: arm64-darwin
extensions: []
dependencies:
- !ruby/object:Gem::Dependency
  name: racc
  requirement: !ruby/object:Gem::Requirement
    requirements:
    - - "~>"
      - !ruby/object:Gem::Version
        version: '1.4'
  type: :runtime
  prerelease: false
  version_requirements: !ruby/object:Gem::Requirement
    requirements:
    - - "~>"
      - !ruby/object:Gem::Version
        version: '1.4'
`)

	files := map[string][]byte{
		"lib/nokogiri.rb":                  []byte("# Nokogiri"),
		"lib/nokogiri/2.7/nokogiri.bundle": []byte("PRECOMPILED_BINARY"), // Precompiled shared object
	}

	if err := createFakeGemArchive(cachedGemPath, files, metadataYAML); err != nil {
		t.Fatalf("Failed to create fake gem: %v", err)
	}

	// Attempt installation
	ctx := context.Background()
	extConfig := &extensions.BuildConfig{
		SkipExtensions: false,
		Verbose:        true,
	}

	report, err := installFromCache(ctx, cacheDir, vendorDir, []lockfile.GemSpec{platformGem}, false, false, extConfig)
	if err != nil {
		t.Fatalf("installFromCache failed: %v", err)
	}

	// CRITICAL: The gem MUST be installed, not skipped
	if report.Installed != 1 {
		t.Errorf("Expected 1 gem installed, got %d (skipped: %d)", report.Installed, report.Skipped)
		t.Errorf("Platform-specific precompiled gem should be installed, not skipped!")
	}

	if report.Skipped != 0 {
		t.Errorf("Expected 0 gems skipped, got %d", report.Skipped)
	}

	// Verify gem directory exists
	gemDir := filepath.Join(vendorDir, "gems", platformGem.FullName())
	if _, err := os.Stat(gemDir); os.IsNotExist(err) {
		t.Errorf("Gem directory not found: %s", gemDir)
	}

	// Verify no extension builds were attempted (gem is precompiled)
	if report.ExtensionsBuilt != 0 {
		t.Errorf("Expected 0 extensions built for precompiled gem, got %d", report.ExtensionsBuilt)
	}

	if report.ExtensionsFailed != 0 {
		t.Errorf("Expected 0 extension failures, got %d", report.ExtensionsFailed)
	}
}
