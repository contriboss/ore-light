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
		Name:     "nokogiri",
		Version:  "1.19.0",
		Platform: "x86_64-linux-gnu", // Platform-specific = precompiled!
	}

	// Create the gem directory as if it was extracted from cache
	// (cache restore extracts gems but doesn't build extensions)
	gemDir := filepath.Join(vendorDir, "gems", platformGem.FullName())
	if err := os.MkdirAll(gemDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a fake gemspec file (precompiled gems have these)
	gemspecPath := filepath.Join(gemDir, platformGem.Name+".gemspec")
	gemspecContent := `# encoding: utf-8
# stub: nokogiri 1.19.0 x86_64-linux-gnu lib

Gem::Specification.new do |s|
  s.name = "nokogiri".freeze
  s.version = "1.19.0"
  s.platform = Gem::Platform.new("x86_64-linux-gnu")
  s.require_paths = ["lib".freeze]
  s.extensions = []  # Precompiled gems have no extensions to build!
end
`
	if err := os.WriteFile(gemspecPath, []byte(gemspecContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to "install" this gem (which is already extracted from cache)
	ctx := context.Background()
	extConfig := &extensions.BuildConfig{
		Verbose: true,
	}

	report, err := installFromCache(ctx, cacheDir, vendorDir, []lockfile.GemSpec{platformGem}, false, false, extConfig)

	if err != nil {
		t.Fatalf("installFromCache failed: %v", err)
	}

	// Verify the gem was skipped (already installed) and no extensions were attempted
	if report.Installed != 0 {
		t.Errorf("Expected 0 gems installed, got %d", report.Installed)
	}

	if report.Skipped != 1 {
		t.Errorf("Expected 1 gem skipped, got %d", report.Skipped)
	}

	// Most importantly: no extension builds should have been attempted!
	if report.ExtensionsBuilt != 0 {
		t.Errorf("Expected 0 extensions built for precompiled gem, got %d", report.ExtensionsBuilt)
	}

	if report.ExtensionsFailed != 0 {
		t.Errorf("Expected 0 extension failures for precompiled gem, got %d", report.ExtensionsFailed)
	}
}
