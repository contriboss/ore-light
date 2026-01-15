package resolver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExistingRubySpecs(t *testing.T) {
	// Create a temporary lockfile for testing
	tmpDir := t.TempDir()
	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")

	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    nokogiri (1.16.0)
      mini_portile2 (~> 2.8.2)
      racc (~> 1.4)
    nokogiri (1.16.0-arm64-darwin)
      racc (~> 1.4)
    nokogiri (1.16.0-x86_64-linux)
      racc (~> 1.4)
    mini_portile2 (2.8.5)
    racc (1.7.3)
    rack (3.0.8)

PLATFORMS
  arm64-darwin
  ruby
  x86_64-linux

DEPENDENCIES
  nokogiri
  rack

BUNDLED WITH
   2.5.4
`
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatalf("failed to write test lockfile: %v", err)
	}

	result := loadExistingRubySpecs(lockfilePath)

	// Should include gems with no platform (ruby platform)
	expectedRubySpecs := map[string]bool{
		"nokogiri":      true,
		"mini_portile2": true,
		"racc":          true,
		"rack":          true,
	}

	if len(result) != len(expectedRubySpecs) {
		t.Errorf("loadExistingRubySpecs() returned %d specs, want %d", len(result), len(expectedRubySpecs))
	}

	for gem := range expectedRubySpecs {
		if !result[gem] {
			t.Errorf("loadExistingRubySpecs() missing expected gem %q", gem)
		}
	}
}

func TestLoadExistingRubySpecs_NonExistent(t *testing.T) {
	result := loadExistingRubySpecs("/nonexistent/path/Gemfile.lock")
	if len(result) != 0 {
		t.Errorf("loadExistingRubySpecs() with non-existent file returned %d specs, want 0", len(result))
	}
}

func TestLoadExistingPlatformVersions(t *testing.T) {
	tmpDir := t.TempDir()
	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")

	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    nokogiri (1.16.0)
      racc (~> 1.4)
    nokogiri (1.16.0-arm64-darwin)
      racc (~> 1.4)
    nokogiri (1.16.0-x86_64-linux)
      racc (~> 1.4)
    racc (1.7.3)

PLATFORMS
  arm64-darwin
  ruby
  x86_64-linux

DEPENDENCIES
  nokogiri

BUNDLED WITH
   2.5.4
`
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatalf("failed to write test lockfile: %v", err)
	}

	result := loadExistingPlatformVersions(lockfilePath)

	// nokogiri should have platform-specific versions
	if result["nokogiri"] == nil {
		t.Fatal("loadExistingPlatformVersions() missing nokogiri")
	}

	if result["nokogiri"]["arm64-darwin"] != "1.16.0" {
		t.Errorf("nokogiri[arm64-darwin] = %q, want %q", result["nokogiri"]["arm64-darwin"], "1.16.0")
	}

	if result["nokogiri"]["x86_64-linux"] != "1.16.0" {
		t.Errorf("nokogiri[x86_64-linux] = %q, want %q", result["nokogiri"]["x86_64-linux"], "1.16.0")
	}

	// racc has no platform-specific version, should not be in result
	if result["racc"] != nil {
		t.Errorf("loadExistingPlatformVersions() should not include racc (no platform-specific version)")
	}
}

func TestLoadExistingPlatformVersions_NonExistent(t *testing.T) {
	result := loadExistingPlatformVersions("/nonexistent/path/Gemfile.lock")
	if len(result) != 0 {
		t.Errorf("loadExistingPlatformVersions() with non-existent file returned %d entries, want 0", len(result))
	}
}
