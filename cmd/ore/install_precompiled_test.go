package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/extensions"
)

// TestPrecompiledGemsDoNotNeedBuilding verifies that platform-specific precompiled gems
// (like nokogiri-1.19.0-x86_64-linux-gnu) are not treated as needing native extension builds.
//
// Bug: When a precompiled gem directory already exists (e.g. from cache restore),
// ore was trying to build native extensions for it, which fails because precompiled
// gems don't have source code - they have pre-built binaries.
func TestPrecompiledGemsDoNotNeedBuilding(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	vendorDir := filepath.Join(tmpDir, "vendor")

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Simulate a platform-specific precompiled gem (like from nokogiri)
	platformGem := lockfile.GemSpec{
		Name:       "nokogiri",
		Version:    "1.19.0",
		Platform:   "x86_64-linux-gnu", // Platform-specific = precompiled!
		Extensions: []string{},         // Precompiled gems have NO extensions to build!
	}

	// Create a fake cached .gem file (this is what installFromCache expects)
	gemFileName := platformGem.FullName() + ".gem"
	cachedGemPath := filepath.Join(cacheDir, gemFileName)

	// Precompiled gem has a gemspec in the data.tar.gz
	gemspecContent := []byte(`# encoding: utf-8
# stub: nokogiri 1.19.0 x86_64-linux-gnu lib

Gem::Specification.new do |s|
  s.name = "nokogiri".freeze
  s.version = "1.19.0"
  s.platform = Gem::Platform.new("x86_64-linux-gnu")
  s.require_paths = ["lib".freeze]
  s.extensions = []  # Precompiled gems have no extensions to build!
end
`)

	// Create fake gem archive
	files := map[string][]byte{
		"nokogiri.gemspec": gemspecContent,
		"lib/.keep":        []byte(""),
	}

	// Use nil for metadata - the gemspec in data.tar.gz is what matters
	if err := createFakeGemArchive(cachedGemPath, files, nil); err != nil {
		t.Fatalf("Failed to create fake gem: %v", err)
	}

	// Try to install this precompiled gem
	ctx := context.Background()
	extConfig := &extensions.BuildConfig{
		SkipExtensions: false, // We want to test that even with extensions enabled, precompiled gems are not built
		Verbose:        true,
	}

	report, err := installFromCache(ctx, cacheDir, vendorDir, []lockfile.GemSpec{platformGem}, false, true, extConfig)

	if err != nil {
		t.Fatalf("installFromCache failed: %v", err)
	}

	// Verify the gem was installed
	if report.Installed != 1 {
		t.Errorf("Expected 1 gem installed, got %d", report.Installed)
	}

	// Most importantly: no extension builds should have been attempted for this precompiled gem!
	// The gem has Extensions: [] (empty), so NeedsBuild should return false
	if report.ExtensionsBuilt != 0 {
		t.Errorf("Expected 0 extensions built for precompiled gem, got %d", report.ExtensionsBuilt)
	}

	if report.ExtensionsFailed != 0 {
		t.Errorf("Expected 0 extension failures for precompiled gem, got %d", report.ExtensionsFailed)
	}

	if report.ExtensionsSkipped != 0 {
		t.Errorf("Expected 0 extensions skipped for precompiled gem (should not even check), got %d", report.ExtensionsSkipped)
	}
}
