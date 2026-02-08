package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultLockfilePathWithBundleGemfile tests that DefaultLockfilePath
// correctly derives the lockfile path when BUNDLE_GEMFILE is set,
// even when the lockfile doesn't exist yet.
func TestDefaultLockfilePathWithBundleGemfile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create TestGemfile (but NOT TestGemfile.lock)
	testGemfilePath := filepath.Join(tmpDir, "TestGemfile")
	if err := os.WriteFile(testGemfilePath, []byte("source 'https://gem.coop'\ngem 'rake'"), 0600); err != nil {
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

	// Call DefaultLockfilePath - it should return TestGemfile.lock
	// even though that file doesn't exist yet
	lockfilePath := DefaultLockfilePath()

	expected := "TestGemfile.lock"
	actual := filepath.Base(lockfilePath)

	if actual != expected {
		t.Errorf("BUG REPRODUCED! DefaultLockfilePath() returned %s, expected %s", actual, expected)
		t.Logf("This causes ore install to look for the wrong lockfile when BUNDLE_GEMFILE is set")
	}
}
