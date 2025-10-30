package extensions

import (
	"os"
	"path/filepath"

	"github.com/contriboss/ore-light/internal/ruby"
)

// NeedsBuild checks if a gem directory needs extension building.
// It returns true if the gem has extension sources but no compiled artifacts.
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

	// Check if compiled artifacts already exist
	return !hasCompiledArtifacts(gemDir), nil
}

// hasCompiledArtifacts checks for compiled extension files in the gem directory.
// It looks for .so (Linux), .bundle (macOS), .dll (Windows), .dylib (macOS), and .jar (JRuby) files.
func hasCompiledArtifacts(gemDir string) bool {
	extensions := []string{".so", ".bundle", ".dll", ".dylib", ".jar"}

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
