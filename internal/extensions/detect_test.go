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

	// Only create the build_complete marker - ext/ directory is irrelevant
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

	// Only create the build_complete marker - ext/ directory is irrelevant
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

	// Create a build_complete marker
	extMarkerDir := filepath.Join(baseDir, "extensions", "x86_64-linux", "3.4.0", fullName)
	if err := os.MkdirAll(extMarkerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extMarkerDir, "gem.build_complete"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Gem declares extensions in metadata
	extensions := []string{"ext/myext/extconf.rb"}
	needsBuild, err := NeedsBuild(gemDir, extensions, "ruby", engine)
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

	// NO build_complete marker exists
	// This tests that NeedsBuild returns true when extensions are declared but not built

	// Gem declares extensions in metadata
	extensions := []string{"ext/myext/extconf.rb"}
	needsBuild, err := NeedsBuild(gemDir, extensions, "ruby", engine)
	if err != nil {
		t.Fatalf("NeedsBuild returned error: %v", err)
	}
	if !needsBuild {
		t.Fatalf("expected NeedsBuild to be true without gem.build_complete marker")
	}
}

// TestNeedsBuildWithExtensionsMetadata verifies that gems with extensions in metadata
// are correctly identified as needing builds, while gems without extensions are not.
func TestNeedsBuildWithExtensionsMetadata(t *testing.T) {
	engine := ruby.Engine{
		Name:    ruby.EngineMRI,
		Version: "3.4.0",
	}

	tests := []struct {
		name       string
		extensions []string
		want       bool // Should we build extensions?
	}{
		{
			name:       "gem with C extensions declared",
			extensions: []string{"ext/myext/extconf.rb"},
			want:       true, // Has extensions, needs build
		},
		{
			name:       "gem with multiple extensions",
			extensions: []string{"ext/foo/extconf.rb", "ext/bar/extconf.rb"},
			want:       true, // Has extensions, needs build
		},
		{
			name:       "pure Ruby gem (no extensions)",
			extensions: []string{},
			want:       false, // No extensions, don't build
		},
		{
			name:       "precompiled gem (empty extensions list)",
			extensions: []string{},
			want:       false, // No extensions declared, already precompiled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			needsBuild, err := NeedsBuild(tmpDir, tt.extensions, "ruby", engine)
			if err != nil {
				t.Fatalf("NeedsBuild() error = %v", err)
			}

			if needsBuild != tt.want {
				t.Errorf("NeedsBuild() = %v, want %v for extensions %v", needsBuild, tt.want, tt.extensions)
			}
		})
	}
}

// TestNeedsBuildWithFilesystemCheck verifies that passing nil for extensions
// triggers a filesystem check for git/path gems
func TestNeedsBuildWithFilesystemCheck(t *testing.T) {
	engine := ruby.Engine{
		Name:    ruby.EngineMRI,
		Version: "3.4.0",
	}

	t.Run("git gem with ext dir", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create ext/ directory with extconf.rb
		extDir := filepath.Join(tmpDir, "ext", "myext")
		if err := os.MkdirAll(extDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# extconf"), 0644); err != nil {
			t.Fatal(err)
		}

		// Pass nil to trigger a filesystem check
		// Git gems are pure Ruby, so pass empty platform
		needsBuild, err := NeedsBuild(tmpDir, nil, "", engine)
		if err != nil {
			t.Fatalf("NeedsBuild() error = %v", err)
		}

		if !needsBuild {
			t.Errorf("NeedsBuild() = false, want true for git gem with ext/ directory")
		}
	})

	t.Run("git gem without ext dir", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Pass nil to trigger filesystem check
		needsBuild, err := NeedsBuild(tmpDir, nil, "", engine)
		if err != nil {
			t.Fatalf("NeedsBuild() error = %v", err)
		}

		if needsBuild {
			t.Errorf("NeedsBuild() = true, want false for git gem without ext/ directory")
		}
	})
}
