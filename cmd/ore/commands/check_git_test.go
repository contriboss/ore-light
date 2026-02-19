package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCheckFindsGitGemInBundlerDir verifies that `ore check` looks for git gems
// under <vendor>/bundler/gems, matching Bundler and InstallGitGems behavior.
func TestCheckFindsGitGemInBundlerDir(t *testing.T) {
	tmpDir := t.TempDir()

	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    rake (13.3.1)

GIT
  remote: https://github.com/rgeo/rgeo.git
  revision: b27518c5745b58f4f8f0f91578f7f9981f0fd5f4
  specs:
    rgeo (3.0.0)

PLATFORMS
  ruby

DEPENDENCIES
  rgeo!

BUNDLED WITH
  4.0.3
`

	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatalf("failed to write lockfile: %v", err)
	}

	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	if err := os.WriteFile(gemfilePath, []byte("source 'https://rubygems.org'\n"), 0644); err != nil {
		t.Fatalf("failed to write Gemfile: %v", err)
	}

	vendorDir := filepath.Join(tmpDir, "vendor", "bundle")

	// Regular gem location
	regularGemDir := filepath.Join(vendorDir, "gems", "rake-13.3.1")
	if err := os.MkdirAll(regularGemDir, 0755); err != nil {
		t.Fatalf("failed to create regular gem dir: %v", err)
	}

	// Git gem location (Bundler convention):
	// <vendor>/bundler/gems/<name>-<revision[:12]>
	gitGemDir := filepath.Join(vendorDir, "bundler", "gems", "rgeo-b27518c5745b")
	if err := os.MkdirAll(gitGemDir, 0755); err != nil {
		t.Fatalf("failed to create git gem dir: %v", err)
	}

	checkArgs := []string{
		"-gemfile", gemfilePath,
		"-vendor", vendorDir,
	}

	if err := RunCheck(checkArgs); err != nil {
		t.Fatalf("expected check to pass with git gem in bundler/gems, got error: %v", err)
	}
}
