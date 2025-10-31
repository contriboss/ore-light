package ruby

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// toMajorMinor helper for tests
func toMajorMinor(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}

func TestDetectRubyVersionFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		rbenvVer    string
		asdfVer     string
		expected    string
		description string
	}{
		{
			name:        "rbenv_priority",
			rbenvVer:    "3.2.0",
			asdfVer:     "3.3.0",
			expected:    "3.2.0",
			description: "RBENV_VERSION should take priority over ASDF_RUBY_VERSION",
		},
		{
			name:        "asdf_only",
			rbenvVer:    "",
			asdfVer:     "3.3.0",
			expected:    "3.3.0",
			description: "ASDF_RUBY_VERSION should be used if RBENV_VERSION is not set",
		},
		{
			name:        "neither_set",
			rbenvVer:    "",
			asdfVer:     "",
			expected:    "",
			description: "Should return empty string if neither variable is set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.rbenvVer != "" {
				if err := os.Setenv("RBENV_VERSION", tt.rbenvVer); err != nil {
					t.Fatal(err)
				}
				defer func() {
					_ = os.Unsetenv("RBENV_VERSION")
				}()
			}
			if tt.asdfVer != "" {
				if err := os.Setenv("ASDF_RUBY_VERSION", tt.asdfVer); err != nil {
					t.Fatal(err)
				}
				defer func() {
					_ = os.Unsetenv("ASDF_RUBY_VERSION")
				}()
			}

			result := DetectRubyVersionFromEnv()
			if result != tt.expected {
				t.Errorf("%s: expected %q, got %q", tt.description, tt.expected, result)
			}
		})
	}
}

func TestParseMiseToml(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "standard_format",
			content: `[tools]
ruby = "3.4.7"
go = "latest"`,
			expected: "3.4.7",
		},
		{
			name: "ruby_only",
			content: `[tools]
ruby = "3.2.0"`,
			expected: "3.2.0",
		},
		{
			name: "no_ruby",
			content: `[tools]
go = "latest"`,
			expected: "",
		},
		{
			name:     "empty_tools",
			content:  `[tools]`,
			expected: "",
		},
		{
			name: "no_tools_section",
			content: `[env]
VAR = "value"`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "mise.toml")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result := parseMiseToml(tmpFile)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseToolVersions(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "standard_format",
			content: `ruby 3.3.0
nodejs 20.0.0
python 3.11.0`,
			expected: "3.3.0",
		},
		{
			name:     "ruby_only",
			content:  `ruby 3.2.0`,
			expected: "3.2.0",
		},
		{
			name: "ruby_last",
			content: `nodejs 20.0.0
ruby 3.4.0`,
			expected: "3.4.0",
		},
		{
			name: "no_ruby",
			content: `nodejs 20.0.0
python 3.11.0`,
			expected: "",
		},
		{
			name:     "empty_file",
			content:  ``,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), ".tool-versions")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result := parseToolVersions(tmpFile)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseRubyVersion(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "standard_format",
			content:  "3.2.0\n",
			expected: "3.2.0",
		},
		{
			name:     "no_newline",
			content:  "3.3.0",
			expected: "3.3.0",
		},
		{
			name:     "with_whitespace",
			content:  "  3.4.0  \n",
			expected: "3.4.0",
		},
		{
			name:     "empty_file",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), ".ruby-version")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result := parseRubyVersion(tmpFile)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWalkUpForFile(t *testing.T) {
	// Create temporary directory structure
	tmpRoot := t.TempDir()
	subdir := filepath.Join(tmpRoot, "project", "src")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .ruby-version in root
	rootVersionFile := filepath.Join(tmpRoot, ".ruby-version")
	if err := os.WriteFile(rootVersionFile, []byte("3.2.0"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		startDir   string
		filename   string
		shouldFind bool
	}{
		{
			name:       "find_in_parent",
			startDir:   subdir,
			filename:   ".ruby-version",
			shouldFind: true,
		},
		{
			name:       "file_not_exists",
			startDir:   subdir,
			filename:   ".nonexistent",
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := walkUpForFile(tt.startDir, tt.filename)
			if tt.shouldFind {
				if result == "" {
					t.Errorf("expected to find %s, but got empty string", tt.filename)
				}
				if !strings.Contains(result, tt.filename) {
					t.Errorf("expected path containing %s, got %s", tt.filename, result)
				}
			} else {
				if result != "" {
					t.Errorf("expected empty string, got %s", result)
				}
			}
		})
	}
}

func TestNormalizeRubyVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain_version",
			input:    "3.4.0",
			expected: "3.4",
		},
		{
			name:     "with_patchlevel",
			input:    "3.2.2p53",
			expected: "3.2",
		},
		{
			name:     "with_ruby_prefix",
			input:    "ruby-3.3.0",
			expected: "3.3",
		},
		{
			name:     "with_greater_equal",
			input:    ">= 3.0.0",
			expected: "3.0",
		},
		{
			name:     "with_tilde_greater",
			input:    "~> 3.2",
			expected: "3.2",
		},
		{
			name:     "with_whitespace",
			input:    "  3.4.0  ",
			expected: "3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeRubyVersion(tt.input, toMajorMinor)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDetectRubyVersionPriority(t *testing.T) {
	// Create a temporary project directory
	tmpDir := t.TempDir()
	gemfilePath := filepath.Join(tmpDir, "Gemfile")
	lockfilePath := filepath.Join(tmpDir, "Gemfile.lock")

	// Create Gemfile (lowest priority)
	gemfileContent := `source 'https://rubygems.org'
ruby '3.1.0'
gem 'rails'`
	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test 1: Only Gemfile exists
	t.Run("gemfile_only", func(t *testing.T) {
		result := DetectRubyVersion(lockfilePath, gemfilePath, toMajorMinor, "3.0")
		if result != "3.1" {
			t.Errorf("expected 3.1 from Gemfile, got %s", result)
		}
	})

	// Test 2: .ruby-version overrides Gemfile
	rubyVersionFile := filepath.Join(tmpDir, ".ruby-version")
	if err := os.WriteFile(rubyVersionFile, []byte("3.2.0"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("ruby_version_overrides_gemfile", func(t *testing.T) {
		result := DetectRubyVersion(lockfilePath, gemfilePath, toMajorMinor, "3.0")
		if result != "3.2" {
			t.Errorf("expected 3.2 from .ruby-version, got %s", result)
		}
	})

	// Test 3: .tool-versions overrides .ruby-version
	toolVersionsFile := filepath.Join(tmpDir, ".tool-versions")
	if err := os.WriteFile(toolVersionsFile, []byte("ruby 3.3.0\nnodejs 20.0.0"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("tool_versions_overrides_ruby_version", func(t *testing.T) {
		result := DetectRubyVersion(lockfilePath, gemfilePath, toMajorMinor, "3.0")
		if result != "3.3" {
			t.Errorf("expected 3.3 from .tool-versions, got %s", result)
		}
	})

	// Test 4: mise.toml overrides .tool-versions
	miseTomlFile := filepath.Join(tmpDir, "mise.toml")
	miseContent := `[tools]
ruby = "3.4.7"
go = "latest"`
	if err := os.WriteFile(miseTomlFile, []byte(miseContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("mise_toml_overrides_tool_versions", func(t *testing.T) {
		result := DetectRubyVersion(lockfilePath, gemfilePath, toMajorMinor, "3.0")
		if result != "3.4" {
			t.Errorf("expected 3.4 from mise.toml, got %s", result)
		}
	})

	// Test 5: Gemfile.lock overrides mise.toml
	lockfileContent := `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.0.0)

PLATFORMS
  ruby

DEPENDENCIES
  rails

RUBY VERSION
   ruby 3.5.0p0

BUNDLED WITH
   2.4.10`
	if err := os.WriteFile(lockfilePath, []byte(lockfileContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("lockfile_overrides_mise_toml", func(t *testing.T) {
		result := DetectRubyVersion(lockfilePath, gemfilePath, toMajorMinor, "3.0")
		if result != "3.5" {
			t.Errorf("expected 3.5 from Gemfile.lock, got %s", result)
		}
	})

	// Test 6: ENV variables override everything
	if err := os.Setenv("RBENV_VERSION", "3.6.0"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Unsetenv("RBENV_VERSION")
	}()

	t.Run("env_overrides_all", func(t *testing.T) {
		result := DetectRubyVersion(lockfilePath, gemfilePath, toMajorMinor, "3.0")
		if result != "3.6" {
			t.Errorf("expected 3.6 from RBENV_VERSION, got %s", result)
		}
	})
}
