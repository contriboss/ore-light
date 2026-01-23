package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOreCheckWithCustomGemfileAndGemHome tests that ore check works when:
// 1. BUNDLE_GEMFILE is set to a custom name (TestGemfile)
// 2. GEM_HOME is set to where gems are actually installed
func TestOreCheckWithCustomGemfileAndGemHome(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create TestGemfile
	testGemfilePath := filepath.Join(tmpDir, "TestGemfile")
	testGemfileContent := `source 'https://gem.coop'
gem 'rake'
`
	if err := os.WriteFile(testGemfilePath, []byte(testGemfileContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Set BUNDLE_GEMFILE
	oldBundleGemfile := os.Getenv("BUNDLE_GEMFILE")
	defer func() {
		if oldBundleGemfile == "" {
			os.Unsetenv("BUNDLE_GEMFILE")
		} else {
			os.Setenv("BUNDLE_GEMFILE", oldBundleGemfile)
		}
	}()
	os.Setenv("BUNDLE_GEMFILE", testGemfilePath)

	// Generate lockfile
	lockfilePath := testGemfilePath + ".lock"
	parsed, err := loadOrGenerateLockfile(lockfilePath, false)
	if err != nil {
		t.Fatalf("loadOrGenerateLockfile() failed: %v", err)
	}

	if parsed == nil {
		t.Fatal("Expected lockfile to be parsed, got nil")
	}

	// Simulate GEM_HOME being set (like rv sets it)
	gemHomeDir := filepath.Join(tmpDir, "gems", "4.0.0")
	gemsDir := filepath.Join(gemHomeDir, "gems")
	if err := os.MkdirAll(gemsDir, 0755); err != nil {
		t.Fatal(err)
	}

	oldGemHome := os.Getenv("GEM_HOME")
	defer func() {
		if oldGemHome == "" {
			os.Unsetenv("GEM_HOME")
		} else {
			os.Setenv("GEM_HOME", oldGemHome)
		}
	}()
	os.Setenv("GEM_HOME", gemHomeDir)

	// Create fake gem directories to simulate installed gems
	for _, spec := range parsed.GemSpecs {
		gemDir := filepath.Join(gemsDir, spec.FullName())
		if err := os.MkdirAll(gemDir, 0755); err != nil {
			t.Fatal(err)
		}
		t.Logf("Created fake gem dir: %s", gemDir)
	}

	// Now test that ore check can find the gems
	t.Run("CheckFindsGemsViaGemHome", func(t *testing.T) {
		// Test defaultVendorDir uses GEM_HOME
		vendorDir := defaultVendorDir()
		t.Logf("defaultVendorDir() returned: %s", vendorDir)

		// It should return GEM_HOME since we set it
		if vendorDir != gemHomeDir {
			t.Errorf("Expected defaultVendorDir() to return GEM_HOME (%s), got %s", gemHomeDir, vendorDir)
		}

		// Verify check would succeed
		for _, spec := range parsed.GemSpecs {
			gemPath := filepath.Join(vendorDir, "gems", spec.FullName())
			if _, err := os.Stat(gemPath); os.IsNotExist(err) {
				t.Errorf("Expected gem %s to exist at %s", spec.Name, gemPath)
			}
		}
	})
}
