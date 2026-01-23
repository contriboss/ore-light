package extensions

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/contriboss/ore-light/internal/ruby"
)

// NeedsBuild checks if a gem directory needs extension building based on gemspec metadata.
// It returns true if the gem declares extensions but no build-complete marker exists.
//
// This follows RubyGems/Bundler conventions by checking the Extensions field from gemspec metadata
// as the authoritative source of truth. The presence of an ext/ directory is NOT checked for
// regular gems, as it may contain documentation, precompiled binaries, or other non-source files.
//
// Parameters:
//   - gemDir: Path to the extracted gem directory
//   - extensions: Extensions list from gemspec metadata (e.g., ["ext/nokogiri/extconf.rb"]).
//     Pass nil for git/path gems where metadata isn't loaded yet (triggers filesystem check).
//   - engine: Ruby engine (for native extension support check)
//
// Returns false if:
//   - Engine doesn't support native extensions (e.g., standard JRuby)
//   - No extensions are declared in gemspec metadata (len(extensions) == 0)
//   - gem.build_complete marker exists (already built)
//
// Returns true if:
//   - Extensions are declared AND no build-complete marker exists
//
// IMPORTANT: The extensions field is the authoritative indicator:
//   - Empty extensions = precompiled OR pure Ruby (no build needed)
//   - Non-empty extensions = needs building from source
//
// Platform in gem name (e.g., x86_64-linux-gnu) does NOT determine if building is needed.
func NeedsBuild(gemDir string, extensions []string, engine ruby.Engine) (bool, error) {

	// Short-circuit: Skip engines that don't support native extensions
	if !engine.SupportsNativeExtensions() {
		return false, nil
	}

	// For git/path gems where metadata isn't available, fall back to filesystem check
	if extensions == nil {
		hasExt, _, err := HasExtensions(gemDir, engine)
		if err != nil || !hasExt {
			return false, err
		}
		// Has extensions - check if already built
		if hasBuildCompleteMarker(gemDir) {
			return false, nil
		}
		return true, nil
	}

	// Check gemspec metadata - this is the authoritative source of truth.
	// If no extensions are declared, nothing needs building, regardless of
	// whether an ext/ directory exists (it may contain docs, precompiled binaries, etc.)
	if len(extensions) == 0 {
		return false, nil
	}

	// Extensions are declared - check if already built
	// RubyGems/Bundler create this marker file after successful extension build
	if hasBuildCompleteMarker(gemDir) {
		return false, nil
	}

	// Extensions declared but not yet built
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
