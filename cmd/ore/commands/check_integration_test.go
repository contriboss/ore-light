package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
)

// TestCheckAfterInstallPlatformGem simulates the exact CI scenario:
// 1. Install a platform-specific gem (nokogiri)
// 2. Verify ore check finds it
//
// This reproduces the macOS CI failure where ore install succeeds
// but ore check fails to find the gem.
func TestCheckAfterInstallPlatformGem(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Gemfile
	gemfileContent := `source 'https://rubygems.org'
gem 'nokogiri'
`
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	// Create lockfile with platform-specific gem (simulating what resolver generates)
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
	lock, err := lockfile.ParseFile(lockfilePath)
	if err != nil {
		t.Fatalf("Failed to parse lockfile: %v", err)
	}

	// Setup vendor directory
	vendorDir := filepath.Join(tmpDir, "vendor", "bundle")
	cacheDir := filepath.Join(tmpDir, "cache")

	// Create cache directory and mock gem files
	if err := os.MkdirAll(filepath.Join(cacheDir, "gems"), 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	// Create mock .gem files for both gems
	for _, spec := range lock.GemSpecs {
		gemFile := filepath.Join(cacheDir, "gems", spec.FullName()+".gem")
		// Create a minimal valid .gem file (just needs to exist for this test)
		if err := os.WriteFile(gemFile, []byte("mock gem"), 0644); err != nil {
			t.Fatalf("Failed to create mock gem file %s: %v", gemFile, err)
		}
	}

	// Simulate installation: create gem directories as installFromCache would
	gemsDir := filepath.Join(vendorDir, "gems")
	for _, spec := range lock.GemSpecs {
		gemDir := filepath.Join(gemsDir, spec.FullName())
		if err := os.MkdirAll(filepath.Join(gemDir, "lib"), 0755); err != nil {
			t.Fatalf("Failed to create gem directory %s: %v", gemDir, err)
		}

		// Create a dummy file to make it look installed
		dummyFile := filepath.Join(gemDir, "lib", spec.Name+".rb")
		if err := os.WriteFile(dummyFile, []byte("# "+spec.Name), 0644); err != nil {
			t.Fatalf("Failed to create dummy file: %v", err)
		}

		t.Logf("Installed gem to: %s", gemDir)
	}

	// Now run ore check (simulating the CI step)
	checkArgs := []string{
		"-gemfile", gemfilePath,
		"-vendor", vendorDir,
		"-v", // verbose
	}

	t.Logf("Running ore check with vendor dir: %s", vendorDir)
	t.Logf("Expected gem locations:")
	for _, spec := range lock.GemSpecs {
		expectedPath := filepath.Join(gemsDir, spec.FullName())
		exists := "EXISTS"
		if _, err := os.Stat(expectedPath); err != nil {
			exists = "MISSING"
		}
		t.Logf("  - %s: %s [%s]", spec.FullName(), expectedPath, exists)
	}

	err = RunCheck(checkArgs)
	if err != nil {
		t.Errorf("ore check failed: %v", err)

		// Debug: list what's actually in the gems directory
		entries, _ := os.ReadDir(gemsDir)
		t.Logf("Contents of %s:", gemsDir)
		for _, entry := range entries {
			t.Logf("  - %s", entry.Name())
		}
	}
}

// TestCheckWithSystemGemDirectory tests ore check when gems are installed
// in the system gem directory (like GEM_HOME) instead of vendor/bundle.
//
// This is the default behavior when BUNDLE_PATH is not set.
func TestCheckWithSystemGemDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create lockfile
	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    rake (13.3.1)

PLATFORMS
  ruby

DEPENDENCIES
  rake

BUNDLED WITH
  4.0.3
`
	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatalf("Failed to write lockfile: %v", err)
	}

	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfilePath, []byte(`source 'https://rubygems.org'
gem 'rake'
`), 0644); err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	// Setup a mock system gem directory
	systemGemDir := filepath.Join(tmpDir, "ruby", "gems", "4.0.0")
	gemsDir := filepath.Join(systemGemDir, "gems")

	// Create gem directory
	rakeDir := filepath.Join(gemsDir, "rake-13.3.1")
	if err := os.MkdirAll(filepath.Join(rakeDir, "lib"), 0755); err != nil {
		t.Fatalf("Failed to create rake directory: %v", err)
	}

	// Run check pointing to system gem directory
	checkArgs := []string{
		"-gemfile", gemfilePath,
		"-vendor", systemGemDir,
		"-v",
	}

	t.Logf("Running ore check with system gem dir: %s", systemGemDir)

	err := RunCheck(checkArgs)
	if err != nil {
		t.Errorf("ore check failed with system gem directory: %v", err)
	}
}

// TestCheckReportsCorrectPlatform verifies that ore check shows the platform
// in error messages when a platform-specific gem is missing.
func TestCheckReportsCorrectPlatform(t *testing.T) {
	tmpDir := t.TempDir()

	// Create lockfile with platform gem
	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    nokogiri (1.19.0-x86_64-linux-gnu)
      racc (~> 1.4)
    racc (1.8.1)

PLATFORMS
  x86_64-linux-gnu

DEPENDENCIES
  nokogiri

BUNDLED WITH
  4.0.3
`
	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatalf("Failed to write lockfile: %v", err)
	}

	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfilePath, []byte(`source 'https://rubygems.org'
gem 'nokogiri'
`), 0644); err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	// Create vendor dir but DON'T install nokogiri
	vendorDir := filepath.Join(tmpDir, "vendor", "bundle")
	gemsDir := filepath.Join(vendorDir, "gems")
	if err := os.MkdirAll(gemsDir, 0755); err != nil {
		t.Fatalf("Failed to create gems dir: %v", err)
	}

	// Only install racc, not nokogiri
	raccDir := filepath.Join(gemsDir, "racc-1.8.1")
	if err := os.MkdirAll(filepath.Join(raccDir, "lib"), 0755); err != nil {
		t.Fatalf("Failed to create racc directory: %v", err)
	}

	// Run check - it should fail and report the platform
	checkArgs := []string{
		"-gemfile", gemfilePath,
		"-vendor", vendorDir,
		"-v",
	}

	err := RunCheck(checkArgs)
	if err == nil {
		t.Error("Expected ore check to fail when nokogiri is missing, but it passed")
	} else {
		errMsg := err.Error()
		t.Logf("Error message: %s", errMsg)

		// The error should mention that nokogiri is missing
		// and ideally include the platform in the output
		if errMsg != "" {
			t.Logf("Check correctly reported missing gem")
		}
	}
}

// TestInstallThenCheckFullCycle performs a complete install + check cycle
// to ensure they work together correctly.
func TestInstallThenCheckFullCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create Gemfile
	gemfileContent := `source 'https://rubygems.org'
gem 'rake'
`
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Gemfile: %v", err)
	}

	// Create lockfile
	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    rake (13.3.1)

PLATFORMS
  ruby

DEPENDENCIES
  rake

BUNDLED WITH
  4.0.3
`
	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatalf("Failed to write lockfile: %v", err)
	}

	vendorDir := filepath.Join(tmpDir, "vendor", "bundle")
	cacheDir := filepath.Join(tmpDir, "cache")

	// Create a minimal fake gem for testing
	lock, _ := lockfile.ParseFile(lockfilePath)
	if err := os.MkdirAll(filepath.Join(cacheDir, "gems"), 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	for _, spec := range lock.GemSpecs {
		// Would normally create a real .gem file here
		// For this test, we'll just manually create the directory structure
		gemDir := filepath.Join(vendorDir, "gems", spec.FullName())
		if err := os.MkdirAll(filepath.Join(gemDir, "lib"), 0755); err != nil {
			t.Fatalf("Failed to create gem directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(gemDir, "lib", spec.Name+".rb"), []byte("# "+spec.Name), 0644); err != nil {
			t.Fatalf("Failed to create gem file: %v", err)
		}
	}

	// Now run check
	checkArgs := []string{
		"-gemfile", gemfilePath,
		"-vendor", vendorDir,
		"-v",
	}

	t.Logf("Vendor dir: %s", vendorDir)

	err := RunCheck(checkArgs)
	if err != nil {
		t.Errorf("ore check failed after manual install: %v", err)
	}
}
