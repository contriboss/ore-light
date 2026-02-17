package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
)

// TestLoadOrGenerateLockfileWithTestGemfile tests generating TestGemfile.lock
func TestLoadOrGenerateLockfileWithTestGemfile(t *testing.T) {
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
			_ = os.Unsetenv("BUNDLE_GEMFILE")
		} else {
			_ = os.Setenv("BUNDLE_GEMFILE", oldEnv)
		}
	}()
	_ = os.Setenv("BUNDLE_GEMFILE", testGemfilePath)

	// The lockfile path that ore will use
	lockfilePath := testGemfilePath + ".lock"

	// Call loadOrGenerateLockfile - this should generate TestGemfile.lock
	parsed, err := loadOrGenerateLockfile(lockfilePath, false, "")
	if err != nil {
		t.Fatalf("loadOrGenerateLockfile() failed: %v", err)
	}

	if parsed == nil {
		t.Fatal("Expected lockfile to be parsed, got nil")
	}

	// Verify the lockfile was created
	if _, err := os.Stat(lockfilePath); os.IsNotExist(err) {
		t.Errorf("Expected %s to be created, but it doesn't exist", lockfilePath)
	}

	// Now simulate what happens in CI: ore list and ore check resolve the lockfile
	// via the Gemfile (respecting BUNDLE_GEMFILE) and lockfile discovery helpers.
	t.Run("FindLockfileOnly", func(t *testing.T) {
		foundLockfile, err := lockfile.FindLockfileOnly()
		if err != nil {
			t.Fatalf("lockfile.FindLockfileOnly() failed: %v", err)
		}

		if filepath.Base(foundLockfile) != "TestGemfile.lock" {
			t.Errorf("Expected FindLockfileOnly to return TestGemfile.lock, got %s", foundLockfile)
		}
	})

	// Test that loadLockfile works with the default path
	t.Run("LoadLockfileFromDefault", func(t *testing.T) {
		defaultPath := defaultLockfilePath()
		t.Logf("defaultLockfilePath() returned: %s", defaultPath)

		if filepath.Base(defaultPath) != "TestGemfile.lock" {
			t.Errorf("Expected defaultLockfilePath() to return TestGemfile.lock, got %s", defaultPath)
		}

		// Try to load it
		loaded, err := loadLockfile(defaultPath)
		if err != nil {
			t.Fatalf("loadLockfile(%s) failed: %v", defaultPath, err)
		}

		if loaded == nil {
			t.Fatal("Expected loaded lockfile, got nil")
		}
	})
}
