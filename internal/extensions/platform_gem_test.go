package extensions

import (
	"testing"

	"github.com/contriboss/ore-light/internal/ruby"
)

// TestPlatformSpecificGemShouldNotNeedBuilding tests that platform-specific precompiled gems
// are correctly identified as NOT needing building.
//
// This is a regression test for the bug where ore was attempting to build extensions
// for precompiled platform-specific gems like nokogiri-1.19.0-x86_64-linux-gnu.
func TestPlatformSpecificGemShouldNotNeedBuilding(t *testing.T) {
	engine := ruby.Engine{
		Name:    ruby.EngineMRI,
		Version: "4.0.1",
	}

	tests := []struct {
		name       string
		extensions []string // Extensions from gem metadata
		platform   string
		wantBuild  bool
	}{
		{
			name:       "precompiled platform gem - empty extensions",
			extensions: []string{}, // Precompiled gems have NO extensions in metadata
			platform:   "x86_64-linux-gnu",
			wantBuild:  false,
		},
		{
			name:       "source gem with extensions",
			extensions: []string{"ext/nokogiri/extconf.rb"},
			platform:   "ruby",
			wantBuild:  true,
		},
		{
			name:       "pure ruby gem",
			extensions: []string{},
			platform:   "ruby",
			wantBuild:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use empty gemDir since we're testing metadata-based detection
			gemDir := t.TempDir()

			needsBuild, err := NeedsBuild(gemDir, tt.extensions, tt.platform, engine)
			if err != nil {
				t.Fatalf("NeedsBuild() error = %v", err)
			}

			if needsBuild != tt.wantBuild {
				t.Errorf("NeedsBuild() = %v, want %v for %s (platform=%s, extensions=%v)",
					needsBuild, tt.wantBuild, tt.name, tt.platform, tt.extensions)
			}
		})
	}
}
