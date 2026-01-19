package extensions

import (
	"os"
	"path/filepath"

	"github.com/contriboss/ore-light/internal/ruby"
)

// NeedsBuild checks if a gem directory needs extension building.
// It returns true if the gem has extension sources but no compiled artifacts.
//
// This mirrors RubyGems' Gem::Specification#missing_extensions? logic:
// - Returns false if no extensions are declared
// - Returns false if gem.build_complete marker exists (precompiled gem)
// - Returns false if compiled artifacts (.so, .bundle, etc.) exist in lib/
func NeedsBuild(gemDir string, engine ruby.Engine) (bool, error) {
	// Short-circuit: Skip engines that don't support native extensions
	if !engine.SupportsNativeExtensions() {
		return false, nil
	}

	// Check if gem has extensions at all
	hasExt, _, err := HasExtensions(gemDir, engine)
	if err != nil || !hasExt {
		return false, err
	}

	// Check for gem.build_complete marker file (RubyGems/Bundler convention)
	// Precompiled native gems include this file to indicate extensions are already built
	if hasBuildCompleteMarker(gemDir) {
		return false, nil
	}

	// Check if compiled artifacts already exist in lib/
	return !hasCompiledArtifacts(gemDir), nil
}

// hasBuildCompleteMarker checks if the gem.build_complete file exists.
// This file is created by RubyGems after successfully building extensions,
// and is included in precompiled native gems to skip rebuilding.
//
// The file can be in multiple locations depending on the gem structure:
// - extensions/<platform>/<ruby-version>/<gem-name>/gem.build_complete (standard)
// - lib/<gem-name>/<ruby-version>/gem.build_complete (some precompiled gems)
// - Directly in gem directory for simpler gems
func hasBuildCompleteMarker(gemDir string) bool {
	// Check common locations for gem.build_complete
	possiblePaths := []string{
		// Inside lib/ for precompiled gems (common for nokogiri, grpc, etc.)
		filepath.Join(gemDir, "lib"),
		// Root of gem directory
		gemDir,
	}

	for _, basePath := range possiblePaths {
		found := false
		_ = filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if d.Name() == "gem.build_complete" {
				found = true
				return filepath.SkipAll
			}
			return nil
		})
		if found {
			return true
		}
	}

	return false
}

// hasCompiledArtifacts checks for compiled extension files in the gem directory.
// It looks for .so (Linux), .bundle (macOS), .dylib (macOS), and .jar (JRuby) files.
func hasCompiledArtifacts(gemDir string) bool {
	extensions := []string{".so", ".bundle", ".dylib", ".jar"}

	// Check lib/ directory where compiled extensions typically live
	libDir := filepath.Join(gemDir, "lib")
	if _, err := os.Stat(libDir); err != nil {
		return false
	}

	return hasArtifactsIn(libDir, extensions)
}

// hasArtifactsIn recursively searches a directory for files with given extensions.
func hasArtifactsIn(dir string, extensions []string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		for _, ext := range extensions {
			if filepath.Ext(path) == ext {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}
