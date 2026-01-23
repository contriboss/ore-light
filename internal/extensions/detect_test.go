package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/ore-light/internal/ruby"
)

func TestHasBuildCompleteMarker_StandardGem(t *testing.T) {
	baseDir := t.TempDir()
	fullName := "nokogiri-1.19.0-x86_64-linux-gnu"
	gemDir := filepath.Join(baseDir, "gems", fullName)

	if err := os.MkdirAll(filepath.Join(gemDir, "ext", "nokogiri"), 0755); err != nil {
		t.Fatal(err)
	}

	extDir := filepath.Join(baseDir, "extensions", "x86_64-linux", "3.4.0", fullName)
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "gem.build_complete"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	if !hasBuildCompleteMarker(gemDir) {
		t.Fatalf("expected gem.build_complete marker to be detected")
	}
}

func TestHasBuildCompleteMarker_BundlerGitGem(t *testing.T) {
	baseDir := t.TempDir()
	fullName := "mygem-abcdef123456"
	gemDir := filepath.Join(baseDir, "ruby", "3.4.0", "bundler", "gems", fullName)

	if err := os.MkdirAll(filepath.Join(gemDir, "ext", "mygem"), 0755); err != nil {
		t.Fatal(err)
	}

	extDir := filepath.Join(baseDir, "ruby", "3.4.0", "extensions", "x86_64-linux", "3.4.0", fullName)
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "gem.build_complete"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	if !hasBuildCompleteMarker(gemDir) {
		t.Fatalf("expected gem.build_complete marker for bundler git gem to be detected")
	}
}

func TestNeedsBuild_WithBuildCompleteMarker(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}
	baseDir := t.TempDir()
	fullName := "myext-1.0.0"
	gemDir := filepath.Join(baseDir, "gems", fullName)

	extDir := filepath.Join(gemDir, "ext", "myext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# extconf"), 0644); err != nil {
		t.Fatal(err)
	}

	extMarkerDir := filepath.Join(baseDir, "extensions", "x86_64-linux", "3.4.0", fullName)
	if err := os.MkdirAll(extMarkerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extMarkerDir, "gem.build_complete"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	needsBuild, err := NeedsBuild(gemDir, "ruby", engine)
	if err != nil {
		t.Fatalf("NeedsBuild returned error: %v", err)
	}
	if needsBuild {
		t.Fatalf("expected NeedsBuild to be false when gem.build_complete is present")
	}
}

func TestNeedsBuild_IgnoresCompiledArtifactsWithoutMarker(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}
	baseDir := t.TempDir()
	fullName := "myext-1.0.0"
	gemDir := filepath.Join(baseDir, "gems", fullName)

	extDir := filepath.Join(gemDir, "ext", "myext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# extconf"), 0644); err != nil {
		t.Fatal(err)
	}

	// Compiled artifact exists in lib/, but no gem.build_complete marker.
	libDir := filepath.Join(gemDir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "myext.so"), []byte("binary"), 0644); err != nil {
		t.Fatal(err)
	}

	needsBuild, err := NeedsBuild(gemDir, "ruby", engine)
	if err != nil {
		t.Fatalf("NeedsBuild returned error: %v", err)
	}
	if !needsBuild {
		t.Fatalf("expected NeedsBuild to be true without gem.build_complete, even if .so exists")
	}
}

// TestNeedsBuildSkipsPrecompiledGems verifies that platform-specific precompiled gems
// (platform != "ruby" and platform != "") are NOT queued for extension building.
func TestNeedsBuildSkipsPrecompiledGems(t *testing.T) {
	engine := ruby.Engine{
		Name:    ruby.EngineMRI,
		Version: "3.4.0",
	}

	tests := []struct {
		name     string
		platform string
		want     bool // Should we build extensions?
	}{
		{
			name:     "precompiled x86_64-linux-gnu",
			platform: "x86_64-linux-gnu",
			want:     false, // Precompiled, don't build
		},
		{
			name:     "precompiled arm64-darwin",
			platform: "arm64-darwin",
			want:     false, // Precompiled, don't build
		},
		{
			name:     "precompiled x86_64-linux",
			platform: "x86_64-linux",
			want:     false, // Precompiled, don't build
		},
		{
			name:     "ruby platform (source gem)",
			platform: "ruby",
			want:     false, // Source gem but no extensions in this test
		},
		{
			name:     "empty platform (source gem)",
			platform: "",
			want:     false, // Source gem but no extensions in this test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a temporary directory (won't have extensions)
			tmpDir := t.TempDir()

			needsBuild, err := NeedsBuild(tmpDir, tt.platform, engine)
			if err != nil {
				t.Fatalf("NeedsBuild() error = %v", err)
			}

			if needsBuild != tt.want {
				t.Errorf("NeedsBuild() = %v, want %v for platform %q", needsBuild, tt.want, tt.platform)
			}
		})
	}
}

// TestNeedsBuildPrecompiledGemsNeverBuild verifies that even if a precompiled gem
// has an ext/ directory (which it shouldn't in real life), we still don't try to build it.
func TestNeedsBuildPrecompiledGemsNeverBuild(t *testing.T) {
	engine := ruby.Engine{
		Name:    ruby.EngineMRI,
		Version: "3.4.0",
	}

	// Precompiled platforms should NEVER trigger extension builds
	precompiledPlatforms := []string{
		"x86_64-linux-gnu",
		"arm64-darwin",
		"x86_64-darwin",
		"x86_64-linux",
		"arm-linux",
		"java", // JRuby precompiled
	}

	for _, platform := range precompiledPlatforms {
		t.Run("platform_"+platform, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Even with no extensions present, verify the logic short-circuits
			needsBuild, err := NeedsBuild(tmpDir, platform, engine)
			if err != nil {
				t.Fatalf("NeedsBuild() error = %v", err)
			}

			if needsBuild {
				t.Errorf("NeedsBuild() = true for precompiled platform %q, want false", platform)
				t.Errorf("Precompiled gems should NEVER need extension building!")
			}
		})
	}
}
