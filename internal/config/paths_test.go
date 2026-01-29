package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToMajorMinor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"3.4.7", "3.4.0"},
		{"3.1", "3.1.0"},
		{"3", "3.0.0"},
		{"2.7.6", "2.7.0"},
		{"3.3.0", "3.3.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToMajorMinor(tt.input)
			if result != tt.expected {
				t.Errorf("ToMajorMinor(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestResolveGemfilePath_BundleGemfile tests that BUNDLE_GEMFILE is respected
// This is a regression test for the bug where ore-light ignored BUNDLE_GEMFILE
// and always fell back to "Gemfile", breaking Appraisal and CI workflows.
func TestResolveGemfilePath_BundleGemfile(t *testing.T) {
	// Save original env var
	originalBundleGemfile := os.Getenv("BUNDLE_GEMFILE")
	defer func() {
		if originalBundleGemfile != "" {
			os.Setenv("BUNDLE_GEMFILE", originalBundleGemfile)
		} else {
			os.Unsetenv("BUNDLE_GEMFILE")
		}
	}()

	tests := []struct {
		name           string
		bundleGemfile  string
		configGemfile  string
		expectedPath   string
		expectedSource string
		description    string
	}{
		{
			name:           "BUNDLE_GEMFILE takes highest priority",
			bundleGemfile:  "Appraisal.root.gemfile",
			configGemfile:  "config.gemfile",
			expectedPath:   "Appraisal.root.gemfile",
			expectedSource: "env:BUNDLE_GEMFILE",
			description:    "When BUNDLE_GEMFILE is set, it should take precedence over everything",
		},
		{
			name:           "Config Gemfile is second priority",
			bundleGemfile:  "",
			configGemfile:  "config.gemfile",
			expectedPath:   "config.gemfile",
			expectedSource: "config:ore",
			description:    "When BUNDLE_GEMFILE is not set, config file should be used",
		},
		{
			name:           "Falls back to Gemfile when nothing is set",
			bundleGemfile:  "",
			configGemfile:  "",
			expectedPath:   "Gemfile",
			expectedSource: "default:Gemfile",
			description:    "When no configuration is provided, default to Gemfile",
		},
		{
			name:           "Real-world Appraisal use case",
			bundleGemfile:  "gemfiles/style.gemfile",
			configGemfile:  "",
			expectedPath:   "gemfiles/style.gemfile",
			expectedSource: "env:BUNDLE_GEMFILE",
			description:    "Appraisal typically sets BUNDLE_GEMFILE to gemfiles/*.gemfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env var first
			os.Unsetenv("BUNDLE_GEMFILE")

			// Set test env var
			if tt.bundleGemfile != "" {
				os.Setenv("BUNDLE_GEMFILE", tt.bundleGemfile)
			}

			// Create config
			var cfg *Config
			if tt.configGemfile != "" {
				cfg = &Config{
					Gemfile: tt.configGemfile,
				}
			}

			// Call ResolveGemfilePath
			path, source, err := ResolveGemfilePath(cfg)

			// Verify no error
			if err != nil {
				t.Fatalf("ResolveGemfilePath() returned error: %v", err)
			}

			// Verify path
			if path != tt.expectedPath {
				t.Errorf("ResolveGemfilePath() path = %q, expected %q\nDescription: %s",
					path, tt.expectedPath, tt.description)
			}

			// Verify source
			if source != tt.expectedSource {
				t.Errorf("ResolveGemfilePath() source = %q, expected %q\nDescription: %s",
					source, tt.expectedSource, tt.description)
			}
		})
	}
}

// TestResolveGemfilePath_BundleGemfile_Regression is a specific regression test
// for the original bug report: ore-light installing wrong dependencies in CI
func TestResolveGemfilePath_BundleGemfile_Regression(t *testing.T) {
	// Save original env
	originalBundleGemfile := os.Getenv("BUNDLE_GEMFILE")
	defer func() {
		if originalBundleGemfile != "" {
			os.Setenv("BUNDLE_GEMFILE", originalBundleGemfile)
		} else {
			os.Unsetenv("BUNDLE_GEMFILE")
		}
	}()

	// Simulate CI environment with Appraisal
	// This is the exact scenario that was failing in tree_haver CI
	os.Setenv("BUNDLE_GEMFILE", "Appraisal.root.gemfile")

	path, source, err := ResolveGemfilePath(nil)

	if err != nil {
		t.Fatalf("ResolveGemfilePath() returned error: %v", err)
	}

	// BEFORE THE FIX: This would return "Gemfile" with source "default:Gemfile"
	// AFTER THE FIX: This returns "Appraisal.root.gemfile" with source "env:BUNDLE_GEMFILE"
	if path != "Appraisal.root.gemfile" {
		t.Errorf("REGRESSION: ResolveGemfilePath() ignored BUNDLE_GEMFILE\n"+
			"Got path = %q (source: %q)\n"+
			"Expected path = %q (source: env:BUNDLE_GEMFILE)\n"+
			"This breaks Appraisal and CI workflows that set BUNDLE_GEMFILE",
			path, source, "Appraisal.root.gemfile")
	}

	if source != "env:BUNDLE_GEMFILE" {
		t.Errorf("REGRESSION: ResolveGemfilePath() used wrong source\n"+
			"Got source = %q\n"+
			"Expected source = %q\n"+
			"BUNDLE_GEMFILE should be the highest priority",
			source, "env:BUNDLE_GEMFILE")
	}
}

// TestDefaultLockfilePath_BundleGemfile tests that BUNDLE_GEMFILE is respected
// for lockfile path resolution. This is the critical regression test for the bug
// where ore-light would fall back to Gemfile.lock even when BUNDLE_GEMFILE pointed
// to a different Gemfile (e.g., Appraisal.root.gemfile).
func TestDefaultLockfilePath_BundleGemfile(t *testing.T) {
	// Save original env var
	originalBundleGemfile := os.Getenv("BUNDLE_GEMFILE")
	defer func() {
		if originalBundleGemfile != "" {
			os.Setenv("BUNDLE_GEMFILE", originalBundleGemfile)
		} else {
			os.Unsetenv("BUNDLE_GEMFILE")
		}
	}()

	tests := []struct {
		name             string
		bundleGemfile    string
		expectedLockfile string
		description      string
	}{
		{
			name:             "Appraisal.root.gemfile derives Appraisal.root.gemfile.lock",
			bundleGemfile:    "Appraisal.root.gemfile",
			expectedLockfile: "Appraisal.root.gemfile.lock",
			description:      "When BUNDLE_GEMFILE=Appraisal.root.gemfile, lockfile should be Appraisal.root.gemfile.lock",
		},
		{
			name:             "gemfiles/style.gemfile derives gemfiles/style.gemfile.lock",
			bundleGemfile:    "gemfiles/style.gemfile",
			expectedLockfile: "gemfiles/style.gemfile.lock",
			description:      "When BUNDLE_GEMFILE=gemfiles/style.gemfile, lockfile should be gemfiles/style.gemfile.lock",
		},
		{
			name:             "custom/path/TestGemfile derives custom/path/TestGemfile.lock",
			bundleGemfile:    "custom/path/TestGemfile",
			expectedLockfile: "custom/path/TestGemfile.lock",
			description:      "When BUNDLE_GEMFILE=custom/path/TestGemfile, lockfile should be custom/path/TestGemfile.lock",
		},
		{
			name:             "gems.rb derives gems.locked",
			bundleGemfile:    "gems.rb",
			expectedLockfile: "gems.locked",
			description:      "When BUNDLE_GEMFILE=gems.rb, lockfile should be gems.locked (newer Bundler convention)",
		},
		{
			name:             "path/to/gems.rb derives path/to/gems.locked",
			bundleGemfile:    "path/to/gems.rb",
			expectedLockfile: "path/to/gems.locked",
			description:      "When BUNDLE_GEMFILE=path/to/gems.rb, lockfile should be path/to/gems.locked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set BUNDLE_GEMFILE
			os.Setenv("BUNDLE_GEMFILE", tt.bundleGemfile)

			// Call DefaultLockfilePath
			lockfilePath := DefaultLockfilePath()

			// Verify lockfile path
			if lockfilePath != tt.expectedLockfile {
				t.Errorf("DefaultLockfilePath() = %q, expected %q\n"+
					"BUNDLE_GEMFILE = %q\n"+
					"Description: %s",
					lockfilePath, tt.expectedLockfile, tt.bundleGemfile, tt.description)
			}
		})
	}
}

func TestResolveVendorDir_BundlePathGemHome(t *testing.T) {
	originalBundlePath := os.Getenv("BUNDLE_PATH")
	defer func() {
		if originalBundlePath != "" {
			os.Setenv("BUNDLE_PATH", originalBundlePath)
		} else {
			os.Unsetenv("BUNDLE_PATH")
		}
	}()

	gemHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gemHome, "specifications"), 0755); err != nil {
		t.Fatalf("failed to create specifications dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gemHome, "gems"), 0755); err != nil {
		t.Fatalf("failed to create gems dir: %v", err)
	}

	if err := os.Setenv("BUNDLE_PATH", gemHome); err != nil {
		t.Fatalf("failed to set BUNDLE_PATH: %v", err)
	}

	got, source, err := ResolveVendorDir(nil, func() string { return "4.0.0" }, func() string { return "/system" })
	if err != nil {
		t.Fatalf("ResolveVendorDir() returned error: %v", err)
	}
	if got != gemHome {
		t.Fatalf("ResolveVendorDir() = %q, want %q", got, gemHome)
	}
	if source != "env:BUNDLE_PATH" {
		t.Fatalf("ResolveVendorDir() source = %q, want env:BUNDLE_PATH", source)
	}
}

func TestResolveVendorDir_BundlePathGemsDir(t *testing.T) {
	originalBundlePath := os.Getenv("BUNDLE_PATH")
	defer func() {
		if originalBundlePath != "" {
			os.Setenv("BUNDLE_PATH", originalBundlePath)
		} else {
			os.Unsetenv("BUNDLE_PATH")
		}
	}()

	gemHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gemHome, "gems"), 0755); err != nil {
		t.Fatalf("failed to create gems dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gemHome, "cache"), 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	if err := os.Setenv("BUNDLE_PATH", gemHome); err != nil {
		t.Fatalf("failed to set BUNDLE_PATH: %v", err)
	}

	got, source, err := ResolveVendorDir(nil, func() string { return "4.0.0" }, func() string { return "/system" })
	if err != nil {
		t.Fatalf("ResolveVendorDir() returned error: %v", err)
	}
	if got != gemHome {
		t.Fatalf("ResolveVendorDir() = %q, want %q", got, gemHome)
	}
	if source != "env:BUNDLE_PATH" {
		t.Fatalf("ResolveVendorDir() source = %q, want env:BUNDLE_PATH", source)
	}
}

func TestResolveVendorDir_BundlePathBaseDir(t *testing.T) {
	originalBundlePath := os.Getenv("BUNDLE_PATH")
	defer func() {
		if originalBundlePath != "" {
			os.Setenv("BUNDLE_PATH", originalBundlePath)
		} else {
			os.Unsetenv("BUNDLE_PATH")
		}
	}()

	base := t.TempDir()
	if err := os.Setenv("BUNDLE_PATH", base); err != nil {
		t.Fatalf("failed to set BUNDLE_PATH: %v", err)
	}

	got, source, err := ResolveVendorDir(nil, func() string { return "4.0.0" }, func() string { return "/system" })
	if err != nil {
		t.Fatalf("ResolveVendorDir() returned error: %v", err)
	}
	want := filepath.Join(base, "ruby", "4.0.0")
	if got != want {
		t.Fatalf("ResolveVendorDir() = %q, want %q", got, want)
	}
	if source != "env:BUNDLE_PATH" {
		t.Fatalf("ResolveVendorDir() source = %q, want env:BUNDLE_PATH", source)
	}
}

func TestResolveVendorDir_BundlePathBaseDir_NoRubyVersion(t *testing.T) {
	originalBundlePath := os.Getenv("BUNDLE_PATH")
	defer func() {
		if originalBundlePath != "" {
			os.Setenv("BUNDLE_PATH", originalBundlePath)
		} else {
			os.Unsetenv("BUNDLE_PATH")
		}
	}()

	base := t.TempDir()
	if err := os.Setenv("BUNDLE_PATH", base); err != nil {
		t.Fatalf("failed to set BUNDLE_PATH: %v", err)
	}

	got, source, err := ResolveVendorDir(nil, func() string { return "" }, func() string { return "/system" })
	if err != nil {
		t.Fatalf("ResolveVendorDir() returned error: %v", err)
	}
	if got != base {
		t.Fatalf("ResolveVendorDir() = %q, want %q", got, base)
	}
	if source != "env:BUNDLE_PATH" {
		t.Fatalf("ResolveVendorDir() source = %q, want env:BUNDLE_PATH", source)
	}
}

// TestDefaultLockfilePath_NoFallback_Regression is a specific regression test
// for the critical bug where ore-light would ignore BUNDLE_GEMFILE and fall back
// to Gemfile.lock when the expected lockfile didn't exist.
//
// This bug caused CI failures in tree_haver when using Appraisal because:
// 1. BUNDLE_GEMFILE=Appraisal.root.gemfile (no lockfile committed)
// 2. ore-light would find Gemfile.lock in current directory
// 3. ore-light would install gems from Gemfile.lock (tree_stump, nokogiri, etc.)
// 4. Build would fail because those gems weren't supposed to be installed
func TestDefaultLockfilePath_NoFallback_Regression(t *testing.T) {
	// Save original env var
	originalBundleGemfile := os.Getenv("BUNDLE_GEMFILE")
	defer func() {
		if originalBundleGemfile != "" {
			os.Setenv("BUNDLE_GEMFILE", originalBundleGemfile)
		} else {
			os.Unsetenv("BUNDLE_GEMFILE")
		}
	}()

	// Simulate the EXACT scenario from tree_haver CI failure
	// where BUNDLE_GEMFILE=Appraisal.root.gemfile but that lockfile doesn't exist
	os.Setenv("BUNDLE_GEMFILE", "Appraisal.root.gemfile")

	lockfilePath := DefaultLockfilePath()

	// THE CRITICAL ASSERTION: When BUNDLE_GEMFILE is set, lockfile path
	// must be derived from it, NEVER fall back to Gemfile.lock!
	expectedLockfile := "Appraisal.root.gemfile.lock"
	if lockfilePath != expectedLockfile {
		t.Errorf("CRITICAL REGRESSION: DefaultLockfilePath() fell back to wrong lockfile!\n"+
			"Got lockfilePath = %q\n"+
			"Expected lockfilePath = %q\n"+
			"BUNDLE_GEMFILE = %q\n\n"+
			"When BUNDLE_GEMFILE is set, there must be NO fallback to Gemfile.lock!\n"+
			"This breaks Appraisal and CI workflows that use BUNDLE_GEMFILE without lockfiles.\n"+
			"Even if the lockfile doesn't exist, return the expected path so ore can create it.",
			lockfilePath, expectedLockfile, "Appraisal.root.gemfile")
	}

	// Additional check: ensure we're not returning "Gemfile.lock"
	if lockfilePath == "Gemfile.lock" {
		t.Errorf("CRITICAL BUG: DefaultLockfilePath() returned Gemfile.lock despite BUNDLE_GEMFILE being set!\n"+
			"BUNDLE_GEMFILE = %q\n"+
			"This is exactly the bug that broke tree_haver CI.\n"+
			"When BUNDLE_GEMFILE is set, Gemfile.lock should NEVER be returned.",
			"Appraisal.root.gemfile")
	}
}

// TestDefaultLockfilePath_UnsetBundleGemfile tests the fallback behavior
// when BUNDLE_GEMFILE is NOT set (normal operation without appraisal)
func TestDefaultLockfilePath_UnsetBundleGemfile(t *testing.T) {
	// Save original env var
	originalBundleGemfile := os.Getenv("BUNDLE_GEMFILE")
	defer func() {
		if originalBundleGemfile != "" {
			os.Setenv("BUNDLE_GEMFILE", originalBundleGemfile)
		} else {
			os.Unsetenv("BUNDLE_GEMFILE")
		}
	}()

	// Unset BUNDLE_GEMFILE to test default behavior
	os.Unsetenv("BUNDLE_GEMFILE")

	lockfilePath := DefaultLockfilePath()

	// When BUNDLE_GEMFILE is not set, should fall back to auto-detection
	// We expect either "Gemfile.lock" or "gems.locked" depending on what exists
	// For this test, we just verify it returns something reasonable
	if lockfilePath == "" {
		t.Error("DefaultLockfilePath() returned empty string when BUNDLE_GEMFILE not set")
	}

	// Verify it's a reasonable default (should end with .lock or .locked)
	if lockfilePath != "Gemfile.lock" && lockfilePath != "gems.locked" {
		t.Logf("DefaultLockfilePath() = %q (may be fine if auto-detected)", lockfilePath)
	}
}
