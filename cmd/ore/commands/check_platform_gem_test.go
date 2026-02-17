package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
)

// TestCheckFindsPlatformSpecificGem tests that ore check correctly finds
// platform-specific gems that were installed with their platform suffix.
//
// Regression test for: ore install skips platform gem (already cached),
// but ore check fails to find it.
func TestCheckFindsPlatformSpecificGem(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Create lockfile with platform-specific gem
	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    nokogiri (1.19.0-arm64-darwin)
      racc (~> 1.4)
    racc (1.8.1)

PLATFORMS
  arm64-darwin-23

DEPENDENCIES
  nokogiri

CHECKSUMS
  nokogiri (1.19.0-arm64-darwin) sha256=0811dfd936d5f6dd3f6d32ef790568bf29b2b7bead9ba68866847b33c9cf5810
  racc (1.8.1) sha256=4a7f6929691dbec8b5209a0b373bc2614882b55fc5d2e447a21aaa691303d62f

BUNDLED WITH
  4.0.3
`

	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatalf("Failed to write lockfile: %v", err)
	}

	// Parse lockfile
	lock, err := lockfile.ParseLockfile(lockfilePath)
	if err != nil {
		t.Fatalf("Failed to parse lockfile: %v", err)
	}

	// Verify parsing extracted platform correctly
	if len(lock.GemSpecs) < 2 {
		t.Fatalf("Expected at least 2 gems, got %d", len(lock.GemSpecs))
	}

	nokogiri := lock.GemSpecs[0]
	if nokogiri.Name != "nokogiri" {
		t.Fatalf("Expected first gem to be nokogiri, got %s", nokogiri.Name)
	}
	if nokogiri.Version != "1.19.0" {
		t.Fatalf("Expected nokogiri version 1.19.0, got %s", nokogiri.Version)
	}
	if nokogiri.Platform != "arm64-darwin" {
		t.Fatalf("Expected nokogiri platform arm64-darwin, got %s", nokogiri.Platform)
	}

	// Verify FullName() constructs correct directory name
	expectedFullName := "nokogiri-1.19.0-arm64-darwin"
	if nokogiri.FullName() != expectedFullName {
		t.Fatalf("Expected FullName %s, got %s", expectedFullName, nokogiri.FullName())
	}

	// Create vendor directory structure with the platform-specific gem
	vendorDir := filepath.Join(tmpDir, "vendor", "bundle")
	gemsDir := filepath.Join(vendorDir, "gems")

	// Create gem directory using FullName() (same as install does)
	nokogiriDir := filepath.Join(gemsDir, nokogiri.FullName())
	if err := os.MkdirAll(nokogiriDir, 0755); err != nil {
		t.Fatalf("Failed to create gem directory: %v", err)
	}

	// Create a dummy file to make it a valid gem directory
	dummyFile := filepath.Join(nokogiriDir, "lib", "nokogiri.rb")
	if err := os.MkdirAll(filepath.Dir(dummyFile), 0755); err != nil {
		t.Fatalf("Failed to create lib directory: %v", err)
	}
	if err := os.WriteFile(dummyFile, []byte("# nokogiri"), 0644); err != nil {
		t.Fatalf("Failed to write dummy file: %v", err)
	}

	// Also create racc gem
	raccDir := filepath.Join(gemsDir, "racc-1.8.1")
	if err := os.MkdirAll(filepath.Join(raccDir, "lib"), 0755); err != nil {
		t.Fatalf("Failed to create racc directory: %v", err)
	}

	// Now run check logic manually (simulating what RunCheck does)
	missing := []string{}
	installed := 0

	for _, spec := range lock.GemSpecs {
		gemPath := filepath.Join(gemsDir, spec.FullName())
		if _, err := os.Stat(gemPath); err != nil {
			missing = append(missing, spec.Name)
			t.Logf("✗ %s (%s) - not found at %s", spec.Name, spec.Version, gemPath)
		} else {
			installed++
			t.Logf("✓ %s (%s) found at %s", spec.Name, spec.Version, gemPath)
		}
	}

	// Verify check found both gems
	if len(missing) > 0 {
		t.Errorf("Check failed to find %d gem(s): %v", len(missing), missing)
	}

	if installed != 2 {
		t.Errorf("Expected 2 gems installed, got %d", installed)
	}
}
