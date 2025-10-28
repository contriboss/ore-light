package ruby

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/contriboss/gemfile-go/gemfile"
)

// DetectRubyVersion detects the Ruby version to use for gem installation
// Priority: 1) Gemfile.lock, 2) Gemfile, 3) defaultVersion
func DetectRubyVersion(lockfilePath, gemfilePath string, toMajorMinor func(string) string, defaultVersion string) string {
	// 1. Try Gemfile.lock RUBY VERSION
	if ver := DetectRubyVersionFromLockfile(lockfilePath, toMajorMinor); ver != "" {
		return ver
	}

	// 2. Try Gemfile ruby directive (tree-sitter)
	if ver := DetectRubyVersionFromGemfile(gemfilePath, toMajorMinor); ver != "" {
		return ver
	}

	// 3. Fallback to default
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
func NormalizeRubyVersion(constraint string, toMajorMinor func(string) string) string {
	// Remove constraint operators
	constraint = strings.TrimSpace(constraint)
	constraint = strings.TrimPrefix(constraint, ">=")
	constraint = strings.TrimPrefix(constraint, "~>")
	constraint = strings.TrimPrefix(constraint, ">")
	constraint = strings.TrimSpace(constraint)

	return toMajorMinor(constraint)
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
			fmt.Sprintf("C:\\Ruby%s\\lib\\ruby\\gems\\%s", strings.Replace(rubyVersion, ".", "", -1), rubyVersion),
			fmt.Sprintf("%s\\Ruby\\lib\\ruby\\gems\\%s", programFiles, rubyVersion),
		}
	}

	return paths
}
