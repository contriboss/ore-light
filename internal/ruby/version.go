package ruby

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/pelletier/go-toml/v2"
)

// DetectRubyVersion detects the Ruby version to use for gem installation
// Priority:
//  1. Environment variables (RBENV_VERSION, ASDF_RUBY_VERSION)
//  2. Gemfile.lock RUBY VERSION
//  3. mise.toml / .mise.toml
//  4. .tool-versions (ASDF/Mise)
//  5. .ruby-version (Rbenv/Mise)
//  6. Gemfile ruby directive
//  7. defaultVersion
func DetectRubyVersion(lockfilePath, gemfilePath string, toMajorMinor func(string) string, defaultVersion string) string {
	// Get directory for version manager file search
	projectDir := filepath.Dir(gemfilePath)
	if projectDir == "" {
		projectDir = "."
	}

	// 1. Check environment variables (explicit override)
	if ver := DetectRubyVersionFromEnv(); ver != "" {
		return toMajorMinor(ver)
	}

	// 2. Try Gemfile.lock RUBY VERSION (project-authoritative)
	if ver := DetectRubyVersionFromLockfile(lockfilePath, toMajorMinor); ver != "" {
		return ver
	}

	// 3. Try mise.toml / .mise.toml (Mise-specific config)
	if ver := DetectRubyVersionFromMiseToml(projectDir, toMajorMinor); ver != "" {
		return ver
	}

	// 4. Try .tool-versions (ASDF/Mise shared format)
	if ver := DetectRubyVersionFromToolVersions(projectDir, toMajorMinor); ver != "" {
		return ver
	}

	// 5. Try .ruby-version (Rbenv/Mise shared format)
	if ver := DetectRubyVersionFromRubyVersion(projectDir, toMajorMinor); ver != "" {
		return ver
	}

	// 6. Try Gemfile ruby directive (may have constraints)
	if ver := DetectRubyVersionFromGemfile(gemfilePath, toMajorMinor); ver != "" {
		return ver
	}

	// 7. Fallback to default
	return defaultVersion
}

// DetectRubyVersionFromLockfile extracts Ruby version from Gemfile.lock
func DetectRubyVersionFromLockfile(lockfilePath string, toMajorMinor func(string) string) string {
	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	inRubySection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for "RUBY VERSION" section
		if trimmed == "RUBY VERSION" {
			inRubySection = true
			continue
		}

		// Parse "   ruby 3.4.0p0" or "   ruby 3.4.0"
		if inRubySection && strings.HasPrefix(trimmed, "ruby ") {
			versionStr := strings.TrimPrefix(trimmed, "ruby ")
			// Remove patchlevel suffix (p0, p194, etc)
			if idx := strings.Index(versionStr, "p"); idx > 0 {
				versionStr = versionStr[:idx]
			}
			return toMajorMinor(versionStr)
		}

		// Exit Ruby section if we hit another section
		if inRubySection && trimmed != "" && !strings.HasPrefix(trimmed, "ruby ") {
			break
		}
	}

	return ""
}

// DetectRubyVersionFromGemfile extracts Ruby version from Gemfile using tree-sitter
func DetectRubyVersionFromGemfile(gemfilePath string, toMajorMinor func(string) string) string {
	parser := gemfile.NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		return ""
	}

	// gemfile-go parses: ruby "3.4.0", ruby ">= 3.0", ruby "~> 3.3", etc
	if parsed.RubyVersion != "" {
		return NormalizeRubyVersion(parsed.RubyVersion, toMajorMinor)
	}

	return ""
}

// NormalizeRubyVersion converts version constraints to usable version
// "3.4.0" -> "3.4.0"
// ">= 3.0.0" -> "3.0.0"
// "~> 3.3" -> "3.3.0"
// "3.2.2p53" -> "3.2.2" (strips patchlevel)
// "ruby-3.2.0" -> "3.2.0" (strips prefix)
func NormalizeRubyVersion(constraint string, toMajorMinor func(string) string) string {
	// Remove constraint operators
	constraint = strings.TrimSpace(constraint)
	constraint = strings.TrimPrefix(constraint, ">=")
	constraint = strings.TrimPrefix(constraint, "~>")
	constraint = strings.TrimPrefix(constraint, ">")
	constraint = strings.TrimSpace(constraint)

	// Remove ruby- prefix (e.g., "ruby-3.2.0" -> "3.2.0")
	constraint = strings.TrimPrefix(constraint, "ruby-")

	// Remove patchlevel suffix (e.g., "3.2.2p53" -> "3.2.2")
	if idx := strings.Index(constraint, "p"); idx > 0 {
		constraint = constraint[:idx]
	}

	return toMajorMinor(constraint)
}

// DetectRubyVersionFromEnv checks environment variables for Ruby version
// Priority: RBENV_VERSION > ASDF_RUBY_VERSION
func DetectRubyVersionFromEnv() string {
	// Rbenv has highest priority
	if ver := os.Getenv("RBENV_VERSION"); ver != "" {
		return strings.TrimSpace(ver)
	}

	// ASDF
	if ver := os.Getenv("ASDF_RUBY_VERSION"); ver != "" {
		return strings.TrimSpace(ver)
	}

	return ""
}

// walkUpForFile walks up from startDir to filesystem root looking for filename
// Returns the full path to the file if found, or empty string
func walkUpForFile(startDir, filename string) string {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}

	homeDir, _ := os.UserHomeDir()

	for {
		candidate := filepath.Join(dir, filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		// Stop at filesystem root
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		// Also stop at home directory to avoid scanning entire filesystem
		if dir == homeDir {
			break
		}

		dir = parent
	}

	return ""
}

// DetectRubyVersionFromMiseToml detects Ruby version from mise.toml or .mise.toml
// Searches from dir upwards to filesystem root
func DetectRubyVersionFromMiseToml(dir string, toMajorMinor func(string) string) string {
	// Try mise.toml first, then .mise.toml
	for _, filename := range []string{"mise.toml", ".mise.toml"} {
		if path := walkUpForFile(dir, filename); path != "" {
			if ver := parseMiseToml(path); ver != "" {
				return toMajorMinor(ver)
			}
		}
	}
	return ""
}

// parseMiseToml parses mise.toml/.mise.toml and extracts ruby version
func parseMiseToml(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var config struct {
		Tools map[string]interface{} `toml:"tools"`
	}

	if err := toml.Unmarshal(data, &config); err != nil {
		return ""
	}

	if config.Tools == nil {
		return ""
	}

	// Handle both string and other types
	if rubyVersion, ok := config.Tools["ruby"]; ok {
		if ver, ok := rubyVersion.(string); ok {
			return ver
		}
	}

	return ""
}

// DetectRubyVersionFromToolVersions detects Ruby version from .tool-versions (ASDF/Mise format)
// Searches from dir upwards to filesystem root
func DetectRubyVersionFromToolVersions(dir string, toMajorMinor func(string) string) string {
	if path := walkUpForFile(dir, ".tool-versions"); path != "" {
		if ver := parseToolVersions(path); ver != "" {
			return toMajorMinor(ver)
		}
	}
	return ""
}

// parseToolVersions parses .tool-versions file (space-separated format)
func parseToolVersions(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "ruby" {
			return fields[1]
		}
	}

	return ""
}

// DetectRubyVersionFromRubyVersion detects Ruby version from .ruby-version (Rbenv/Mise format)
// Searches from dir upwards to filesystem root
func DetectRubyVersionFromRubyVersion(dir string, toMajorMinor func(string) string) string {
	if path := walkUpForFile(dir, ".ruby-version"); path != "" {
		if ver := parseRubyVersion(path); ver != "" {
			return toMajorMinor(ver)
		}
	}
	return ""
}

// parseRubyVersion parses .ruby-version file (single-line format)
func parseRubyVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// GetSystemGemDir returns the system gem directory without requiring Ruby
// Tries: 1) GEM_HOME env, 2) Standard OS paths, 3) User gem dir, 4) gem command
func GetSystemGemDir(detectRubyVersion func() string) string {
	// 1. Check GEM_HOME environment variable
	if gemHome := os.Getenv("GEM_HOME"); gemHome != "" {
		if info, err := os.Stat(gemHome); err == nil && info.IsDir() {
			return gemHome
		}
	}

	// 2. Detect Ruby version for path construction
	rubyVersion := detectRubyVersion()

	// 3. Try standard OS-specific gem paths
	standardPaths := GetStandardGemPaths(rubyVersion)
	for _, path := range standardPaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}

	// 4. Try user gem directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		userGemDir := filepath.Join(homeDir, ".gem", "ruby", rubyVersion)
		// Return even if doesn't exist - will be created during install
		return userGemDir
	}

	// 5. Last resort: try `gem environment gemdir` if Ruby is available
	cmd := exec.Command("gem", "environment", "gemdir")
	if output, err := cmd.Output(); err == nil {
		gemDir := strings.TrimSpace(string(output))
		if gemDir != "" {
			return gemDir
		}
	}

	// Should never reach here due to step 4 always returning
	return ""
}

// GetStandardGemPaths returns OS-specific standard gem installation paths
func GetStandardGemPaths(rubyVersion string) []string {
	var paths []string

	switch runtime.GOOS {
	case "darwin": // macOS
		paths = []string{
			fmt.Sprintf("/Library/Ruby/Gems/%s", rubyVersion),
			fmt.Sprintf("/opt/homebrew/lib/ruby/gems/%s", rubyVersion),
			fmt.Sprintf("/usr/local/lib/ruby/gems/%s", rubyVersion),
		}

	case "linux":
		paths = []string{
			fmt.Sprintf("/usr/lib/ruby/gems/%s", rubyVersion),
			fmt.Sprintf("/usr/local/lib/ruby/gems/%s", rubyVersion),
			fmt.Sprintf("/usr/lib64/ruby/gems/%s", rubyVersion),
		}

	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = "C:\\Program Files"
		}
		paths = []string{
			fmt.Sprintf("C:\\Ruby%s\\lib\\ruby\\gems\\%s", strings.ReplaceAll(rubyVersion, ".", ""), rubyVersion),
			fmt.Sprintf("%s\\Ruby\\lib\\ruby\\gems\\%s", programFiles, rubyVersion),
		}
	}

	return paths
}
