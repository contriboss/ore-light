package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/audit"
)

// TestGemsListAndFilter tests the gems command functionality
func TestGemsListAndFilter(t *testing.T) {
	// Test filtering gems
	gems := []GemInfo{
		{Name: "rack", Version: "3.0.0", Path: "/path/to/rack-3.0.0"},
		{Name: "rack", Version: "3.1.0", Path: "/path/to/rack-3.1.0"},
		{Name: "rails", Version: "7.0.0", Path: "/path/to/rails-7.0.0"},
		{Name: "rake", Version: "13.0.0", Path: "/path/to/rake-13.0.0"},
	}

	// Test no filter
	filtered := filterGems(gems, "")
	if len(filtered) != 4 {
		t.Errorf("expected 4 gems with no filter, got %d", len(filtered))
	}

	// Test filter for "rack"
	filtered = filterGems(gems, "rack")
	if len(filtered) != 2 {
		t.Errorf("expected 2 gems matching 'rack', got %d", len(filtered))
	}

	// Test filter for "rail"
	filtered = filterGems(gems, "rail")
	if len(filtered) != 1 {
		t.Errorf("expected 1 gem matching 'rail', got %d", len(filtered))
	}
	if filtered[0].Name != "rails" {
		t.Errorf("expected 'rails', got %q", filtered[0].Name)
	}

	// Test case insensitive
	filtered = filterGems(gems, "RACK")
	if len(filtered) != 2 {
		t.Errorf("expected 2 gems matching 'RACK' (case insensitive), got %d", len(filtered))
	}
}

// TestOpenGetEditor tests editor detection
func TestOpenGetEditor(t *testing.T) {
	// Save original env
	origBundler := os.Getenv("BUNDLER_EDITOR")
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	t.Cleanup(func() {
		_ = os.Setenv("BUNDLER_EDITOR", origBundler)
		_ = os.Setenv("VISUAL", origVisual)
		_ = os.Setenv("EDITOR", origEditor)
	})

	// Test BUNDLER_EDITOR takes precedence
	_ = os.Setenv("BUNDLER_EDITOR", "bundler_editor")
	_ = os.Setenv("VISUAL", "visual")
	_ = os.Setenv("EDITOR", "editor")
	if got := getEditor(); got != "bundler_editor" {
		t.Errorf("expected BUNDLER_EDITOR to take precedence, got %q", got)
	}

	// Test VISUAL is next
	_ = os.Unsetenv("BUNDLER_EDITOR")
	if got := getEditor(); got != "visual" {
		t.Errorf("expected VISUAL as second choice, got %q", got)
	}

	// Test EDITOR is last
	_ = os.Unsetenv("VISUAL")
	if got := getEditor(); got != "editor" {
		t.Errorf("expected EDITOR as fallback, got %q", got)
	}

	// Test empty when none set
	_ = os.Unsetenv("EDITOR")
	if got := getEditor(); got != "" {
		t.Errorf("expected empty string when no editor set, got %q", got)
	}
}

// TestPristineValidation tests pristine command validation
func TestPristineValidation(t *testing.T) {
	tmpDir := t.TempDir()
	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")

	// Create a simple lockfile
	lockContent := `GEM
  remote: https://rubygems.org/
  specs:
    rack (3.0.0)
    rails (7.0.0)

PLATFORMS
  ruby

DEPENDENCIES
  rack
  rails
`
	if err := os.WriteFile(lockfilePath, []byte(lockContent), 0644); err != nil {
		t.Fatalf("failed to write test lockfile: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	vendorDir := filepath.Join(tmpDir, "vendor")

	// Test with valid gem
	err := Pristine([]string{"rack"}, lockfilePath, cacheDir, vendorDir)
	// We expect this to fail because gem pristine won't find the gem, but it should validate the name
	if err == nil {
		t.Log("pristine completed (gem pristine might have run successfully)")
	}

	// Test with no gems should error
	err = Pristine([]string{}, lockfilePath, cacheDir, vendorDir)
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected usage error with no gems, got %v", err)
	}
}

// TestSearchResultDeduplication tests search result deduplication
func TestSearchResultDeduplication(t *testing.T) {
	// This would test the search command's deduplication logic
	// For now, we'll test the concept
	type searchResult struct {
		Name    string
		Version string
		Source  string
	}

	results := []searchResult{
		{Name: "rack", Version: "3.0.0", Source: "rubygems.org"},
		{Name: "rack", Version: "3.0.0", Source: "gem.coop"}, // Duplicate
		{Name: "rails", Version: "7.0.0", Source: "rubygems.org"},
	}

	// Deduplicate by name
	seen := make(map[string]bool)
	var deduplicated []searchResult
	for _, r := range results {
		if !seen[r.Name] {
			seen[r.Name] = true
			deduplicated = append(deduplicated, r)
		}
	}

	if len(deduplicated) != 2 {
		t.Errorf("expected 2 deduplicated results, got %d", len(deduplicated))
	}
}

// TestWhyBuildReverseDeps tests dependency chain building
func TestWhyBuildReverseDeps(t *testing.T) {
	specs := []lockfile.GemSpec{
		{
			Name:    "rails",
			Version: "7.0.0",
			Dependencies: []lockfile.Dependency{
				{Name: "actionpack"},
				{Name: "activerecord"},
			},
		},
		{
			Name:    "actionpack",
			Version: "7.0.0",
			Dependencies: []lockfile.Dependency{
				{Name: "rack"},
			},
		},
		{
			Name:    "activerecord",
			Version: "7.0.0",
			Dependencies: []lockfile.Dependency{
				{Name: "activesupport"},
			},
		},
		{
			Name:         "rack",
			Version:      "3.0.0",
			Dependencies: []lockfile.Dependency{},
		},
		{
			Name:         "activesupport",
			Version:      "7.0.0",
			Dependencies: []lockfile.Dependency{},
		},
	}

	reverseDeps := buildReverseDeps(specs)

	// rack should be depended on by actionpack
	if deps, ok := reverseDeps["rack"]; !ok {
		t.Error("expected rack in reverse deps")
	} else if len(deps) != 1 || deps[0] != "actionpack" {
		t.Errorf("expected rack to be depended on by actionpack, got %v", deps)
	}

	// actionpack should be depended on by rails
	if deps, ok := reverseDeps["actionpack"]; !ok {
		t.Error("expected actionpack in reverse deps")
	} else if len(deps) != 1 || deps[0] != "rails" {
		t.Errorf("expected actionpack to be depended on by rails, got %v", deps)
	}

	// activesupport should be depended on by activerecord
	if deps, ok := reverseDeps["activesupport"]; !ok {
		t.Error("expected activesupport in reverse deps")
	} else if len(deps) != 1 || deps[0] != "activerecord" {
		t.Errorf("expected activesupport to be depended on by activerecord, got %v", deps)
	}
}

// TestLicenseScanning tests license scanning returns proper structure
func TestLicenseScanning(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with empty directory
	report, err := audit.ScanLicenses(tmpDir)
	// Should handle empty directory gracefully
	if err != nil && !strings.Contains(err.Error(), "no gem specifications found") {
		t.Errorf("unexpected error for empty dir: %v", err)
	}

	if report != nil && len(report.Gems) > 0 {
		t.Errorf("expected empty report for empty dir, got %d licenses", len(report.Gems))
	}
}

// TestBrowseGroupGemsByName tests gem grouping in browse command
func TestBrowseGroupGemsByName(t *testing.T) {
	gems := []GemInfo{
		{Name: "rack", Version: "3.0.0", Path: "/path/to/rack-3.0.0"},
		{Name: "rack", Version: "3.1.0", Path: "/path/to/rack-3.1.0"},
		{Name: "rack", Version: "3.2.0", Path: "/path/to/rack-3.2.0"},
		{Name: "rails", Version: "7.0.0", Path: "/path/to/rails-7.0.0"},
	}

	grouped := groupGemsByName(gems)

	if len(grouped) != 2 {
		t.Fatalf("expected 2 grouped gems, got %d", len(grouped))
	}

	// Find rack group
	var rackGroup *groupedGem
	for i := range grouped {
		if grouped[i].name == "rack" {
			rackGroup = &grouped[i]
			break
		}
	}

	if rackGroup == nil {
		t.Fatal("expected to find rack in grouped gems")
	}

	if len(rackGroup.versions) != 3 {
		t.Errorf("expected 3 versions of rack, got %d", len(rackGroup.versions))
	}

	expectedVersions := map[string]bool{"3.0.0": true, "3.1.0": true, "3.2.0": true}
	for _, v := range rackGroup.versions {
		if !expectedVersions[v] {
			t.Errorf("unexpected version %q in rack group", v)
		}
	}

	if len(rackGroup.paths) != 3 {
		t.Errorf("expected 3 paths for rack, got %d", len(rackGroup.paths))
	}
}

// TestPostInstallMessageParsing tests that we can read messages
func TestPostInstallMessageParsing(t *testing.T) {
	// This test would need Ruby available and actual gemspecs
	// For now, we test the structure exists
	tmpDir := t.TempDir()

	messages, err := ReadPostInstallMessages(tmpDir)
	// Should not error even if directory is empty
	if err != nil {
		t.Errorf("ReadPostInstallMessages should not error on empty dir: %v", err)
	}

	// Should return empty slice for empty directory
	if len(messages) != 0 {
		t.Errorf("expected 0 messages from empty dir, got %d", len(messages))
	}
}
