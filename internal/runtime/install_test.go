package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/gemfile-go/lockfile"
)

func TestShortRevision(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "long revision truncated to 12 chars",
			input: "b27518c5745b123456789abcdef0123456789",
			want:  "b27518c5745b",
		},
		{
			name:  "short revision unchanged",
			input: "abc123",
			want:  "abc123",
		},
		{
			name:  "empty revision",
			input: "",
			want:  "",
		},
		{
			name:  "exactly 12 chars",
			input: "123456789012",
			want:  "123456789012",
		},
		{
			name:  "13 chars truncated",
			input: "1234567890123",
			want:  "123456789012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortRevision(tt.input)
			if got != tt.want {
				t.Errorf("shortRevision(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGitGemInstallPath(t *testing.T) {
	// Test that git gem paths follow Bundler convention.
	// vendorDir is the complete gem home (already includes ruby scope if needed).
	// Git gems go in: <gem-home>/bundler/gems/<name>-<revision[:12]>
	vendorDir := "/usr/local/bundle/ruby/4.0.0" // complete gem home path
	revision := "b27518c5745b123456789abcdef"
	gemName := "rgeo"

	expectedDir := "/usr/local/bundle/ruby/4.0.0/bundler/gems/rgeo-b27518c5745b"
	actualDir := filepath.Join(vendorDir, "bundler", "gems", gemName+"-"+shortRevision(revision))

	if actualDir != expectedDir {
		t.Errorf("git gem path = %q, want %q", actualDir, expectedDir)
	}
}

func TestGitGemInstallPathSystemGemDir(t *testing.T) {
	// Test that git gems in system gem directory work correctly.
	// System gem dir is the complete gem home: /path/to/ruby/gems/4.0.0
	// Git gems go in: /path/to/ruby/gems/4.0.0/bundler/gems/<name>-<revision>
	systemGemDir := "/usr/local/lib/ruby/gems/4.0.0" // complete gem home path
	revision := "b27518c5745b"
	gemName := "rgeo"

	expectedDir := "/usr/local/lib/ruby/gems/4.0.0/bundler/gems/rgeo-b27518c5745b"
	actualDir := filepath.Join(systemGemDir, "bundler", "gems", gemName+"-"+revision)

	if actualDir != expectedDir {
		t.Errorf("git gem path in system dir = %q, want %q", actualDir, expectedDir)
	}
}

func TestPathGemInstallPath(t *testing.T) {
	// Test that path gem paths follow Bundler convention.
	// vendorDir is the complete gem home (already includes ruby scope if needed).
	// Path gems go in: <gem-home>/gems/<name>-<version>
	vendorDir := "/usr/local/bundle/ruby/4.0.0" // complete gem home path
	gemName := "my_gem"
	version := "1.0.0"

	expectedDir := "/usr/local/bundle/ruby/4.0.0/gems/my_gem-1.0.0"
	actualDir := filepath.Join(vendorDir, "gems", gemName+"-"+version)

	if actualDir != expectedDir {
		t.Errorf("path gem path = %q, want %q", actualDir, expectedDir)
	}
}

func TestPathGemInstallPathSystemGemDir(t *testing.T) {
	// Test that path gems in system gem directory work correctly.
	// System gem dir is the complete gem home: /path/to/ruby/gems/4.0.0
	// Path gems go in: /path/to/ruby/gems/4.0.0/gems/<name>-<version>
	systemGemDir := "/usr/local/lib/ruby/gems/4.0.0" // complete gem home path
	gemName := "my_gem"
	version := "1.0.0"

	expectedDir := "/usr/local/lib/ruby/gems/4.0.0/gems/my_gem-1.0.0"
	actualDir := filepath.Join(systemGemDir, "gems", gemName+"-"+version)

	if actualDir != expectedDir {
		t.Errorf("path gem path in system dir = %q, want %q", actualDir, expectedDir)
	}
}

func TestInstallFromCacheCreatesVendorDirs(t *testing.T) {
	cacheDir := t.TempDir()
	vendorDir := filepath.Join(t.TempDir(), "vendor")

	_, err := InstallFromCache(context.Background(), cacheDir, vendorDir, nil, false, false, nil)
	if err != nil {
		t.Fatalf("InstallFromCache() returned error: %v", err)
	}

	expectedDirs := []string{
		filepath.Join(vendorDir, "gems"),
		filepath.Join(vendorDir, "cache"),
		filepath.Join(vendorDir, "bin"),
		filepath.Join(vendorDir, "specifications", "cache"),
	}
	for _, dir := range expectedDirs {
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			t.Fatalf("expected directory %q to exist", dir)
		}
	}
}

func TestInstallPathGems_RelativePath(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a dummy gem directory
	gemDir := filepath.Join(tmpDir, "libs", "my_gem")
	if err := os.MkdirAll(gemDir, 0755); err != nil {
		t.Fatalf("failed to create gem dir: %v", err)
	}
	gemspecPath := filepath.Join(gemDir, "my_gem.gemspec")
	gemspecContent := `Gem::Specification.new do |s|
  s.name    = "my_gem"
  s.version = "0.1.0"
end`
	if err := os.WriteFile(gemspecPath, []byte(gemspecContent), 0644); err != nil {
		t.Fatalf("failed to write gemspec: %v", err)
	}

	// Setup vendor dir as complete gem home (already includes ruby scope)
	vendorDir := filepath.Join(tmpDir, "vendor", "ruby", "3.4.0")
	rubyScope := "ruby/3.4.0" // Kept for API compatibility but not used in path construction
	lockfileDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(lockfileDir, 0755); err != nil {
		t.Fatalf("failed to create lockfile dir: %v", err)
	}

	pathSpecs := []lockfile.PathGemSpec{
		{
			Name:    "my_gem",
			Version: "0.1.0",
			Remote:  "../libs/my_gem", // Relative to lockfileDir
		},
	}

	// Call InstallPathGems
	_, err := InstallPathGems(ctx, vendorDir, rubyScope, pathSpecs, false, false, nil, lockfileDir)
	if err != nil {
		t.Fatalf("InstallPathGems failed: %v", err)
	}

	// Verify gem was "installed" (copied) to vendor directory
	// vendorDir is already the complete gem home, so gems go directly under <vendorDir>/gems/
	expectedGemDir := filepath.Join(vendorDir, "gems", "my_gem-0.1.0")
	if _, err := os.Stat(expectedGemDir); os.IsNotExist(err) {
		t.Errorf("expected gem directory %s to exist", expectedGemDir)
	}
}
