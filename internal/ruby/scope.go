package ruby

import (
	"path/filepath"
	"strings"
)

// Scope returns the Bundler-compatible ruby scope path segment.
// Format: "{engine}/{api_version}" e.g., "ruby/3.4.0"
// Falls back to DefaultRubyVersion if Ruby is not installed.
func Scope() string {
	engine := DetectEngine()
	version := DetectVersion()
	if version == "" {
		version = DefaultRubyVersion
	}
	engineName := engine.Name
	if engineName == "" {
		engineName = EngineMRI
	}
	return filepath.Join(engineName, ToMajorMinor(version))
}

// ToMajorMinor converts a Ruby version to its API version format.
// Examples: "3.4.7" -> "3.4.0", "3.4" -> "3.4.0", "3" -> "3.0.0"
func ToMajorMinor(version string) string {
	if version == "" {
		return ""
	}

	parts := strings.Split(version, ".")
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0] + ".0.0"
	case 2:
		return parts[0] + "." + parts[1] + ".0"
	default:
		return parts[0] + "." + parts[1] + ".0"
	}
}

// DetectVersion returns the detected Ruby version or empty string if not found.
func DetectVersion() string {
	version := DetectRubyVersion("", "", ToMajorMinor, "")
	return version
}
