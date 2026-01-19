package extensions

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/contriboss/ore-light/internal/ruby"
)

// NeedsBuild checks if a gem directory needs extension building.
// It returns true if the gem has extension sources but no build-complete marker.
//
// This mirrors RubyGems/Bundler missing_extensions? logic:
// - Returns false if no extensions are declared
// - Returns false if gem.build_complete marker exists in the extensions dir
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
	if hasBuildCompleteMarker(gemDir) {
		return false, nil
	}

	// Without a build-complete marker, consider extensions missing.
	return true, nil
}

// hasBuildCompleteMarker checks if the gem.build_complete file exists.
// RubyGems/Bundler place this file in the extension dir:
//
//	<gem_home>/extensions/<platform>/<api_version>/<full_name>/gem.build_complete
func hasBuildCompleteMarker(gemDir string) bool {
	baseDir, fullName := baseDirAndFullName(gemDir)
	if baseDir == "" || fullName == "" {
		return false
	}

	// We don't try to compute platform/api_version; glob both segments instead.
	pattern := filepath.Join(baseDir, "extensions", "*", "*", fullName, "gem.build_complete")
	// Error only occurs for malformed patterns (syntax errors); our pattern is always valid
	matches, _ := filepath.Glob(pattern)
	return len(matches) > 0
}

func baseDirAndFullName(gemDir string) (string, string) {
	cleaned := filepath.Clean(gemDir)
	fullName := filepath.Base(cleaned)
	sep := string(os.PathSeparator)
	pathWithSep := cleaned + sep

	// Bundler git/path gems are under .../<ruby_scope>/bundler/gems/<name>
	if idx := strings.LastIndex(pathWithSep, sep+"bundler"+sep+"gems"+sep); idx != -1 {
		base := strings.TrimSuffix(pathWithSep[:idx], sep)
		return base, fullName
	}

	// Standard gems are under .../gems/<full_name>
	if idx := strings.LastIndex(pathWithSep, sep+"gems"+sep); idx != -1 {
		base := strings.TrimSuffix(pathWithSep[:idx], sep)
		return base, fullName
	}

	// Fallback: use parent dir
	return filepath.Dir(cleaned), fullName
}
