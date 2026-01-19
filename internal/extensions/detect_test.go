package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/ore-light/internal/ruby"
)

func TestNeedsBuild(t *testing.T) {
	// Use MRI engine for tests (supports native extensions)
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	tests := []struct {
		name      string
		setupFunc func(t *testing.T) string
		want      bool
		wantErr   bool
	}{
		{
			name: "no extensions directory - does not need build",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				// Just lib/ directory, no ext/
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "has extconf.rb but no compiled artifacts - needs build",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "myext")
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# extconf"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "has extconf.rb and .so file - does not need build",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "myext")
				libDir := filepath.Join(dir, "lib", "myext")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# extconf"), 0644); err != nil {
					t.Fatal(err)
				}
				// Compiled .so file exists
				if err := os.WriteFile(filepath.Join(libDir, "myext.so"), []byte("compiled"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "has extconf.rb and gem.build_complete marker - does not need build (precompiled gem)",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "nokogiri")
				libDir := filepath.Join(dir, "lib", "nokogiri", "3.4")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# extconf"), 0644); err != nil {
					t.Fatal(err)
				}
				// gem.build_complete marker exists (precompiled gem convention)
				if err := os.WriteFile(filepath.Join(libDir, "gem.build_complete"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "has Cargo.toml with cdylib and .so file - does not need build",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "rust_ext")
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				cargo := []byte("[package]\nname = \"rust_ext\"\n[lib]\ncrate-type = [\"cdylib\"]\n")
				if err := os.WriteFile(filepath.Join(extDir, "Cargo.toml"), cargo, 0644); err != nil {
					t.Fatal(err)
				}
				// Compiled .so file exists
				if err := os.WriteFile(filepath.Join(libDir, "rust_ext.so"), []byte("compiled"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "has .bundle file (macOS) - does not need build",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "myext")
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# extconf"), 0644); err != nil {
					t.Fatal(err)
				}
				// Compiled .bundle file exists (macOS)
				if err := os.WriteFile(filepath.Join(libDir, "myext.bundle"), []byte("compiled"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setupFunc(t)

			got, err := NeedsBuild(dir, engine)

			if (err != nil) != tt.wantErr {
				t.Errorf("NeedsBuild() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("NeedsBuild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsBuild_JRubyEngine(t *testing.T) {
	// JRuby engine - native extensions work differently
	engine := ruby.Engine{Name: ruby.EngineJRuby, Version: "9.4.0"}

	t.Run("JRuby with .jar file - does not need build", func(t *testing.T) {
		dir := t.TempDir()
		extDir := filepath.Join(dir, "ext", "java_ext")
		libDir := filepath.Join(dir, "lib")
		if err := os.MkdirAll(extDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(libDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(extDir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
			t.Fatal(err)
		}
		// Compiled .jar file exists
		if err := os.WriteFile(filepath.Join(libDir, "java_ext.jar"), []byte("compiled"), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := NeedsBuild(dir, engine)
		if err != nil {
			t.Errorf("NeedsBuild() error = %v", err)
			return
		}

		if got != false {
			t.Errorf("NeedsBuild() = %v, want false (JRuby with .jar)", got)
		}
	})
}

func TestHasBuildCompleteMarker(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) string
		want      bool
	}{
		{
			name: "no marker file",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: false,
		},
		{
			name: "marker in lib/ subdirectory (precompiled gem style)",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib", "nokogiri", "3.4")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(libDir, "gem.build_complete"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: true,
		},
		{
			name: "marker in gem root directory",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "gem.build_complete"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: true,
		},
		{
			name: "marker deeply nested in lib/",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				deepDir := filepath.Join(dir, "lib", "grpc", "2.1", "x86_64-linux")
				if err := os.MkdirAll(deepDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(deepDir, "gem.build_complete"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setupFunc(t)

			got := hasBuildCompleteMarker(dir)

			if got != tt.want {
				t.Errorf("hasBuildCompleteMarker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasCompiledArtifacts(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) string
		want      bool
	}{
		{
			name: "no lib directory",
			setupFunc: func(t *testing.T) string {
				return t.TempDir()
			},
			want: false,
		},
		{
			name: "empty lib directory",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: false,
		},
		{
			name: "lib with .rb files only",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(libDir, "mylib.rb"), []byte("# ruby"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: false,
		},
		{
			name: "lib with .so file",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(libDir, "myext.so"), []byte("binary"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: true,
		},
		{
			name: "lib with .bundle file (macOS)",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(libDir, "myext.bundle"), []byte("binary"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: true,
		},
		{
			name: "lib with .dylib file (macOS)",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(libDir, "myext.dylib"), []byte("binary"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: true,
		},
		{
			name: "lib with .jar file (JRuby)",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(libDir, "myext.jar"), []byte("binary"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: true,
		},
		{
			name: ".so file in nested directory (lib/gem/version/)",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				libDir := filepath.Join(dir, "lib", "nokogiri", "3.4")
				if err := os.MkdirAll(libDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(libDir, "nokogiri.so"), []byte("binary"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setupFunc(t)

			got := hasCompiledArtifacts(dir)

			if got != tt.want {
				t.Errorf("hasCompiledArtifacts() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNeedsBuild_PrecompiledNokogiriScenario tests the specific scenario that was failing:
// a precompiled nokogiri gem with ext/extconf.rb but also with compiled .so files
func TestNeedsBuild_PrecompiledNokogiriScenario(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	// Simulate nokogiri-1.19.0-x86_64-linux-gnu structure
	dir := t.TempDir()

	// Create ext/ structure (this is what was triggering false positives)
	extDir := filepath.Join(dir, "ext", "nokogiri")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# nokogiri extconf"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create lib/ structure with precompiled .so files
	libDir := filepath.Join(dir, "lib", "nokogiri", "3.4")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "nokogiri.so"), []byte("precompiled binary"), 0644); err != nil {
		t.Fatal(err)
	}

	// Also add the gem.build_complete marker (what precompiled gems include)
	if err := os.WriteFile(filepath.Join(libDir, "gem.build_complete"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// This should return false - the gem is already built
	got, err := NeedsBuild(dir, engine)
	if err != nil {
		t.Fatalf("NeedsBuild() error = %v", err)
	}

	if got != false {
		t.Errorf("NeedsBuild() = %v, want false for precompiled nokogiri-like gem", got)
	}
}

// TestNeedsBuild_SourceGemNeedsCompilation tests that source gems (without precompiled artifacts)
// correctly return true for NeedsBuild
func TestNeedsBuild_SourceGemNeedsCompilation(t *testing.T) {
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	// Simulate a source gem that needs compilation
	dir := t.TempDir()

	// Create ext/ structure
	extDir := filepath.Join(dir, "ext", "myext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte("# needs compilation"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "myext.c"), []byte("/* C source */"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create lib/ structure without compiled files (only Ruby files)
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "myext.rb"), []byte("# Ruby wrapper"), 0644); err != nil {
		t.Fatal(err)
	}

	// This should return true - the gem needs to be built
	got, err := NeedsBuild(dir, engine)
	if err != nil {
		t.Fatalf("NeedsBuild() error = %v", err)
	}

	if got != true {
		t.Errorf("NeedsBuild() = %v, want true for source gem needing compilation", got)
	}
}
