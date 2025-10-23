// Package extensions provides native extension compilation support for Ruby gems
package extensions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	rubyext "github.com/contriboss/ruby-extension-go"
)

// Ruby developers: This is like a configuration object for native extensions
// Similar to what bundle install uses when compiling C extensions
type BuildConfig struct {
	SkipExtensions bool
	Verbose        bool
	Parallel       int
	RubyPath       string
}

// This is like RubyGems' ext builder but as a Go service object
// Wraps ruby-extension-go for building native extensions (C, Rust, etc.)
type Builder struct {
	factory *rubyext.BuilderFactory
	config  *BuildConfig
}

// NewBuilder creates a new extension builder
func NewBuilder(config *BuildConfig) *Builder {
	if config == nil {
		config = &BuildConfig{
			Parallel: 4, // Default to 4 parallel jobs
		}
	}

	return &Builder{
		factory: rubyext.NewBuilderFactory(),
		config:  config,
	}
}

// BuildResult represents the result of building extensions for a gem
type BuildResult struct {
	GemName    string
	Extensions []string
	Success    bool
	Skipped    bool
	Error      error
}

// HasExtensions checks if a gem directory contains extensions
func HasExtensions(gemDir string) (bool, []string, error) {
	// Check for common extension directories and files
	extDir := filepath.Join(gemDir, "ext")
	if _, err := os.Stat(extDir); err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, err
	}

	// Find extension files
	var extensions []string
	err := filepath.Walk(extDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check for known extension build files
		name := info.Name()
		if name == "extconf.rb" || name == "Rakefile" || name == "rakefile" ||
			name == "CMakeLists.txt" || name == "Cargo.toml" ||
			name == "configure" || name == "configure.sh" || name == "mkrf_conf.rb" {
			// Make path relative to gemDir
			relPath, err := filepath.Rel(gemDir, path)
			if err != nil {
				return err
			}
			extensions = append(extensions, relPath)
		}
		return nil
	})

	if err != nil {
		return false, nil, err
	}

	return len(extensions) > 0, extensions, nil
}

// BuildExtensions builds all extensions for a gem
func (b *Builder) BuildExtensions(ctx context.Context, gemDir, gemName string) (*BuildResult, error) {
	result := &BuildResult{
		GemName: gemName,
		Success: false,
	}

	// Check if we should skip extensions
	if b.config.SkipExtensions {
		result.Skipped = true
		result.Success = true
		return result, nil
	}

	// Check if gem has extensions
	hasExt, extensions, err := HasExtensions(gemDir)
	if err != nil {
		result.Error = fmt.Errorf("failed to check for extensions: %w", err)
		return result, result.Error
	}

	if !hasExt {
		result.Skipped = true
		result.Success = true
		return result, nil
	}

	// Verify Ruby is available
	rubyPath := b.config.RubyPath
	if rubyPath == "" {
		rubyPath = "ruby"
	}

	if _, err := exec.LookPath(rubyPath); err != nil {
		result.Error = fmt.Errorf("ruby not found in PATH (required for building extensions): %w", err)
		return result, result.Error
	}

	// Get Ruby version
	rubyVersion, err := getRubyVersion(rubyPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to get Ruby version: %w", err)
		return result, result.Error
	}

	// Configure build
	buildConfig := &rubyext.BuildConfig{
		GemDir:      gemDir,
		RubyPath:    rubyPath,
		RubyVersion: rubyVersion,
		Verbose:     b.config.Verbose,
		Parallel:    b.config.Parallel,
		// StopOnFailure: true, // Stop on first failure
	}

	// Build all extensions
	results, err := b.factory.BuildAllExtensions(ctx, buildConfig, extensions)
	if err != nil {
		result.Error = fmt.Errorf("extension build failed for %s: %w", gemName, err)
		return result, result.Error
	}

	// Collect built extensions
	var builtExtensions []string
	var buildFailed bool
	for _, extResult := range results {
		if !extResult.Success {
			buildFailed = true
			if b.config.Verbose {
				fmt.Fprintf(os.Stderr, "Extension build failed:\n%s\n", strings.Join(extResult.Output, "\n"))
			}
		} else {
			builtExtensions = append(builtExtensions, extResult.Extensions...)
		}
	}

	if buildFailed {
		result.Error = fmt.Errorf("one or more extensions failed to build for %s", gemName)
		return result, result.Error
	}

	result.Extensions = builtExtensions
	result.Success = true
	return result, nil
}

// getRubyVersion executes ruby -v and extracts the version
func getRubyVersion(rubyPath string) (string, error) {
	cmd := exec.Command(rubyPath, "-v")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse output like "ruby 3.4.0 (2024-12-25 revision ...) [x86_64-darwin24]"
	versionStr := string(output)
	parts := strings.Fields(versionStr)
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected ruby version output: %s", versionStr)
	}

	return parts[1], nil
}

// ShouldSkipExtensions checks environment and config to determine if extensions should be skipped
func ShouldSkipExtensions() bool {
	// Check environment variable
	if skip := os.Getenv("ORE_SKIP_EXTENSIONS"); skip != "" {
		return skip == "1" || strings.ToLower(skip) == "true" || strings.ToLower(skip) == "yes"
	}
	if skip := os.Getenv("ORE_LIGHT_SKIP_EXTENSIONS"); skip != "" {
		return skip == "1" || strings.ToLower(skip) == "true" || strings.ToLower(skip) == "yes"
	}
	return false
}

// IsRubyAvailable checks if Ruby is available in PATH
func IsRubyAvailable() bool {
	_, err := exec.LookPath("ruby")
	return err == nil
}

// IsCompilerAvailable checks if a C compiler is available
func IsCompilerAvailable() bool {
	compilers := []string{"gcc", "clang", "cc"}
	for _, compiler := range compilers {
		if _, err := exec.LookPath(compiler); err == nil {
			return true
		}
	}
	return false
}
