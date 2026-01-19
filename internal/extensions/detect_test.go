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

	needsBuild, err := NeedsBuild(gemDir, engine)
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

	needsBuild, err := NeedsBuild(gemDir, engine)
	if err != nil {
		t.Fatalf("NeedsBuild returned error: %v", err)
	}
	if !needsBuild {
		t.Fatalf("expected NeedsBuild to be true without gem.build_complete, even if .so exists")
	}
}
