package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/ore-light/internal/ruby"
)

// TestNeedsBuildUsesMetadataNotLockfile verifies that NeedsBuild correctly uses
// the extensions parameter passed to it (from gem metadata), not from any other source.
// This is a regression test for the bug where lockfile extensions were used instead.
func TestNeedsBuildUsesMetadataNotLockfile(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	tests := []struct {
		name               string
		metadataExtensions []string // Extensions from gem metadata (authoritative)
		wantBuild          bool
		description        string
	}{
		{
			name:               "precompiled gem - empty extensions in metadata",
			metadataExtensions: []string{}, // Empty = precompiled or pure Ruby
			wantBuild:          false,
			description:        "Precompiled platform-specific gems have empty extensions",
		},
		{
			name:               "source gem - extensions declared in metadata",
			metadataExtensions: []string{"ext/nokogiri/extconf.rb"},
			wantBuild:          true,
			description:        "Source gems with extensions need building",
		},
		{
			name:               "pure Ruby gem - empty extensions",
			metadataExtensions: []string{},
			wantBuild:          false,
			description:        "Pure Ruby gems have no extensions",
		},
		{
			name:               "gem with multiple extensions",
			metadataExtensions: []string{"ext/foo/extconf.rb", "ext/bar/extconf.rb"},
			wantBuild:          true,
			description:        "Multiple extensions should trigger build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gemDir := t.TempDir()

			// Call NeedsBuild with the metadata extensions
			// The platform parameter is unused (kept for API compatibility)
			needsBuild, err := NeedsBuild(gemDir, tt.metadataExtensions, engine)
			if err != nil {
				t.Fatalf("NeedsBuild() error = %v", err)
			}

			if needsBuild != tt.wantBuild {
				t.Errorf("NeedsBuild() = %v, want %v\n  Description: %s\n  Extensions: %v",
					needsBuild, tt.wantBuild, tt.description, tt.metadataExtensions)
			}
		})
	}
}

// TestNeedsBuildWithBuildCompleteMarkerFromMetadata verifies that gems with a build_complete
// marker are not rebuilt even if extensions are declared in metadata.
func TestNeedsBuildWithBuildCompleteMarkerFromMetadata(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}
	baseDir := t.TempDir()
	fullName := "nokogiri-1.19.0"
	gemDir := filepath.Join(baseDir, "gems", fullName)

	// Create the gem directory
	if err := os.MkdirAll(gemDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a build_complete marker
	extMarkerDir := filepath.Join(baseDir, "extensions", "x86_64-linux", "3.4.0", fullName)
	if err := os.MkdirAll(extMarkerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extMarkerDir, "gem.build_complete"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Gem metadata declares extensions (would normally need building)
	metadataExtensions := []string{"ext/nokogiri/extconf.rb"}

	needsBuild, err := NeedsBuild(gemDir, metadataExtensions, engine)
	if err != nil {
		t.Fatalf("NeedsBuild() error = %v", err)
	}

	if needsBuild {
		t.Errorf("NeedsBuild() = true, want false when gem.build_complete marker exists")
	}
}

// TestPlatformSpecificPrecompiledGemDoesNotBuild tests the critical case that
// was broken: platform-specific precompiled gems (e.g., nokogiri-1.19.0-x86_64-linux-gnu)
// should NOT be built because their metadata extensions field is empty.
func TestPlatformSpecificPrecompiledGemDoesNotBuild(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	tests := []struct {
		name               string
		platform           string
		metadataExtensions []string
		wantBuild          bool
	}{
		{
			name:               "nokogiri precompiled x86_64-linux-gnu",
			platform:           "x86_64-linux-gnu",
			metadataExtensions: []string{}, // Precompiled = empty extensions
			wantBuild:          false,
		},
		{
			name:               "nokogiri precompiled arm64-darwin",
			platform:           "arm64-darwin",
			metadataExtensions: []string{}, // Precompiled = empty extensions
			wantBuild:          false,
		},
		{
			name:               "sassc precompiled x86_64-linux",
			platform:           "x86_64-linux",
			metadataExtensions: []string{}, // Precompiled = empty extensions
			wantBuild:          false,
		},
		{
			name:               "hypothetical source gem with platform suffix",
			platform:           "x86_64-linux-gnu",
			metadataExtensions: []string{"ext/example/extconf.rb"}, // Source with extensions
			wantBuild:          true,                               // Still needs building even with platform suffix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gemDir := t.TempDir()

			needsBuild, err := NeedsBuild(gemDir, tt.metadataExtensions, engine)
			if err != nil {
				t.Fatalf("NeedsBuild() error = %v", err)
			}

			if needsBuild != tt.wantBuild {
				t.Errorf("NeedsBuild() = %v, want %v\n  Platform: %s\n  Extensions: %v",
					needsBuild, tt.wantBuild, tt.platform, tt.metadataExtensions)
			}
		})
	}
}

// TestNilExtensionsTriggersFilesystemCheck verifies that passing nil for extensions
// (for git/path gems where metadata isn't available) triggers a filesystem check.
func TestNilExtensionsTriggersFilesystemCheckBehavior(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	t.Run("nil extensions with ext dir triggers build", func(t *testing.T) {
		gemDir := t.TempDir()

		// Create ext/ directory with extconf.rb
		extDir := filepath.Join(gemDir, "ext", "example")
		if err := os.MkdirAll(extDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# extconf"), 0644); err != nil {
			t.Fatal(err)
		}

		// Pass nil to trigger filesystem check
		needsBuild, err := NeedsBuild(gemDir, nil, engine)
		if err != nil {
			t.Fatalf("NeedsBuild() error = %v", err)
		}

		if !needsBuild {
			t.Errorf("NeedsBuild() = false, want true when nil extensions and ext/ exists")
		}
	})

	t.Run("nil extensions without ext dir skips build", func(t *testing.T) {
		gemDir := t.TempDir()

		// No ext/ directory
		needsBuild, err := NeedsBuild(gemDir, nil, engine)
		if err != nil {
			t.Fatalf("NeedsBuild() error = %v", err)
		}

		if needsBuild {
			t.Errorf("NeedsBuild() = true, want false when nil extensions and no ext/ dir")
		}
	})
}

// TestBundlerParityExtensionsCheck verifies that our logic matches Bundler's exactly:
// Build if and only if spec.extensions is not empty (and not already built).
func TestBundlerParityExtensionsCheck(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	tests := []struct {
		name       string
		extensions []string
		wantBuild  bool
		bundlerRef string
	}{
		{
			name:       "empty extensions - Bundler skips",
			extensions: []string{},
			wantBuild:  false,
			bundlerRef: "spec.extensions.empty? -> skip build",
		},
		{
			name:       "one extension - Bundler builds",
			extensions: []string{"ext/extconf.rb"},
			wantBuild:  true,
			bundlerRef: "spec.extensions.any? -> build",
		},
		{
			name:       "multiple extensions - Bundler builds",
			extensions: []string{"ext/foo/extconf.rb", "ext/bar/extconf.rb"},
			wantBuild:  true,
			bundlerRef: "spec.extensions.any? -> build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gemDir := t.TempDir()

			needsBuild, err := NeedsBuild(gemDir, tt.extensions, engine)
			if err != nil {
				t.Fatalf("NeedsBuild() error = %v", err)
			}

			if needsBuild != tt.wantBuild {
				t.Errorf("NeedsBuild() = %v, want %v (Bundler: %s)",
					needsBuild, tt.wantBuild, tt.bundlerRef)
			}
		})
	}
}

// TestEngineWithoutNativeExtensionSupport verifies that engines like JRuby
// that don't support native extensions skip building regardless of extensions.
func TestEngineWithoutNativeExtensionSupportSkipsBuild(t *testing.T) {
	jrubyEngine := ruby.Engine{Name: ruby.EngineJRuby, Version: "9.4.0.0"}

	tests := []struct {
		name       string
		extensions []string
	}{
		{
			name:       "JRuby with C extensions declared",
			extensions: []string{"ext/nokogiri/extconf.rb"},
		},
		{
			name:       "JRuby with multiple extensions",
			extensions: []string{"ext/foo/extconf.rb", "ext/bar/extconf.rb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gemDir := t.TempDir()

			needsBuild, err := NeedsBuild(gemDir, tt.extensions, jrubyEngine)
			if err != nil {
				t.Fatalf("NeedsBuild() error = %v", err)
			}

			if needsBuild {
				t.Errorf("NeedsBuild() = true, want false for JRuby (no native extension support)")
			}
		})
	}
}
