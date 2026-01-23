package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOreCheckWithCustomGemfile tests that ore check works with BUNDLE_GEMFILE set
func TestOreCheckWithCustomGemfile(t *testing.T) {
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
	oldEnv := os.Getenv("BUNDLE_GEMFILE")
	defer func() {
		if oldEnv == "" {
			os.Unsetenv("BUNDLE_GEMFILE")
		} else {
			os.Setenv("BUNDLE_GEMFILE", oldEnv)
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

	// Verify lockfile was created
	if _, err := os.Stat(lockfilePath); os.IsNotExist(err) {
		t.Fatalf("Expected %s to be created", lockfilePath)
	}

	// Now test that ore check can find the lockfile
	t.Run("CheckFindsLockfile", func(t *testing.T) {
		// Get the default gemfile path (should respect BUNDLE_GEMFILE)
		gemfilePath := defaultGemfilePath()
		t.Logf("defaultGemfilePath() returned: %s", gemfilePath)

		if filepath.Base(gemfilePath) != "TestGemfile" {
			t.Errorf("Expected defaultGemfilePath() to return TestGemfile, got %s", gemfilePath)
		}

		// Try to find lockfile from gemfile
		foundLockfile, err := findLockfilePath(gemfilePath)
		if err != nil {
			t.Fatalf("findLockfilePath(%s) failed: %v", gemfilePath, err)
		}

		if filepath.Base(foundLockfile) != "TestGemfile.lock" {
			t.Errorf("Expected findLockfilePath() to return TestGemfile.lock, got %s", foundLockfile)
		}
	})

	// Test that defaultVendorDir works correctly
	t.Run("CheckVendorDir", func(t *testing.T) {
		vendorDir := defaultVendorDir()
		t.Logf("defaultVendorDir() returned: %s", vendorDir)

		// The vendor dir should be consistent between install and check
		// This test verifies the Ruby version detection is working
		if vendorDir == "" {
			t.Error("defaultVendorDir() returned empty string")
		}
	})
}
