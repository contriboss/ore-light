package config

import (
	"os"
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
