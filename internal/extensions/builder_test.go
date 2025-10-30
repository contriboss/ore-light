package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/contriboss/ore-light/internal/ruby"
)

func TestHasExtensions(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(t *testing.T) string
		wantHas      bool
		wantExtCount int
		wantErr      bool
	}{
		{
			name: "no ext directory",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				// No ext/ directory
				return dir
			},
			wantHas:      false,
			wantExtCount: 0,
			wantErr:      false,
		},
		{
			name: "ext directory with extconf.rb",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "myext")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantHas:      true,
			wantExtCount: 1,
			wantErr:      false,
		},
		{
			name: "ext directory with CMakeLists.txt",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "cmake_ext")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "CMakeLists.txt"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantHas:      true,
			wantExtCount: 1,
			wantErr:      false,
		},
		{
			name: "ext directory with Cargo.toml",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "rust_ext")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "Cargo.toml"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantHas:      true,
			wantExtCount: 1,
			wantErr:      false,
		},
		{
			name: "ext directory with configure script",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "autotools")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "configure"), []byte("#!/bin/sh"), 0755); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantHas:      true,
			wantExtCount: 1,
			wantErr:      false,
		},
		{
			name: "ext directory with Rakefile",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(extDir, "Rakefile"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantHas:      true,
			wantExtCount: 1,
			wantErr:      false,
		},
		{
			name: "ext directory with only source files (no build file)",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				extDir := filepath.Join(dir, "ext", "myext")
				if err := os.MkdirAll(extDir, 0755); err != nil {
					t.Fatal(err)
				}
				// Only C source files, no build configuration
				if err := os.WriteFile(filepath.Join(extDir, "myext.c"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantHas:      false,
			wantExtCount: 0,
			wantErr:      false,
		},
		{
			name: "multiple extensions",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()

				// First extension with extconf.rb
				ext1 := filepath.Join(dir, "ext", "ext1")
				if err := os.MkdirAll(ext1, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(ext1, "extconf.rb"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}

				// Second extension with CMakeLists.txt
				ext2 := filepath.Join(dir, "ext", "ext2")
				if err := os.MkdirAll(ext2, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(ext2, "CMakeLists.txt"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}

				return dir
			},
			wantHas:      true,
			wantExtCount: 2,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setupFunc(t)
			// Use MRI engine for tests
			engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}
			has, extensions, err := HasExtensions(dir, engine)

			if (err != nil) != tt.wantErr {
				t.Errorf("HasExtensions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if has != tt.wantHas {
				t.Errorf("HasExtensions() has = %v, want %v", has, tt.wantHas)
			}

			if len(extensions) != tt.wantExtCount {
				t.Errorf("HasExtensions() extension count = %d, want %d", len(extensions), tt.wantExtCount)
			}
		})
	}
}

func TestBuildExtensions_SkipExtensions(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, "ext", "myext")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extconf.rb"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	config := &BuildConfig{
		SkipExtensions: true,
		Verbose:        false,
	}

	builder := NewBuilder(config)
	ctx := context.Background()
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	result, err := builder.BuildExtensions(ctx, dir, "test-gem", engine)
	if err != nil {
		t.Errorf("BuildExtensions() error = %v, want nil", err)
	}

	if !result.Skipped {
		t.Error("BuildExtensions() should skip when SkipExtensions is true")
	}

	if !result.Success {
		t.Error("BuildExtensions() should succeed when skipping")
	}
}

func TestBuildExtensions_NoExtensions(t *testing.T) {
	dir := t.TempDir()

	config := &BuildConfig{
		SkipExtensions: false,
		Verbose:        false,
	}

	builder := NewBuilder(config)
	ctx := context.Background()
	engine := ruby.Engine{Name: ruby.EngineMRI, Version: "3.4.0"}

	result, err := builder.BuildExtensions(ctx, dir, "test-gem", engine)
	if err != nil {
		t.Errorf("BuildExtensions() error = %v, want nil", err)
	}

	if !result.Skipped {
		t.Error("BuildExtensions() should skip when no extensions found")
	}

	if !result.Success {
		t.Error("BuildExtensions() should succeed when no extensions")
	}
}

func TestShouldSkipExtensions(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    bool
	}{
		{
			name:    "no env vars set",
			envVars: map[string]string{},
			want:    false,
		},
		{
			name: "ORE_SKIP_EXTENSIONS=1",
			envVars: map[string]string{
				"ORE_SKIP_EXTENSIONS": "1",
			},
			want: true,
		},
		{
			name: "ORE_SKIP_EXTENSIONS=true",
			envVars: map[string]string{
				"ORE_SKIP_EXTENSIONS": "true",
			},
			want: true,
		},
		{
			name: "ORE_SKIP_EXTENSIONS=yes",
			envVars: map[string]string{
				"ORE_SKIP_EXTENSIONS": "yes",
			},
			want: true,
		},
		{
			name: "ORE_SKIP_EXTENSIONS=0",
			envVars: map[string]string{
				"ORE_SKIP_EXTENSIONS": "0",
			},
			want: false,
		},
		{
			name: "ORE_LIGHT_SKIP_EXTENSIONS=1",
			envVars: map[string]string{
				"ORE_LIGHT_SKIP_EXTENSIONS": "1",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars first
			_ = os.Unsetenv("ORE_SKIP_EXTENSIONS")
			_ = os.Unsetenv("ORE_LIGHT_SKIP_EXTENSIONS")

			// Set test env vars
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
			}

			got := ShouldSkipExtensions()
			if got != tt.want {
				t.Errorf("ShouldSkipExtensions() = %v, want %v", got, tt.want)
			}

			// Clean up
			for k := range tt.envVars {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestIsRubyAvailable(t *testing.T) {
	// This test is environment-dependent
	// We just check that the function doesn't panic
	available := IsRubyAvailable()
	t.Logf("Ruby available: %v", available)
}

func TestNewBuilder(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &BuildConfig{
			SkipExtensions: true,
			Verbose:        true,
			Parallel:       8,
		}
		builder := NewBuilder(config)
		if builder == nil {
			t.Fatal("NewBuilder() returned nil")
		}
		if builder.config.Parallel != 8 {
			t.Errorf("NewBuilder() config.Parallel = %d, want 8", builder.config.Parallel)
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		builder := NewBuilder(nil)
		if builder == nil {
			t.Fatal("NewBuilder() returned nil")
		}
		if builder.config.Parallel != 4 {
			t.Errorf("NewBuilder() config.Parallel = %d, want 4 (default)", builder.config.Parallel)
		}
	})
}
