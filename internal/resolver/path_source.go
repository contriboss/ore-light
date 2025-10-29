package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/pubgrub-go"
)

// PathSource handles resolution of gems from local paths
type PathSource struct {
	// Path to the local gem directory
	Path string
	// Absolute path (resolved)
	AbsPath string
	// Dependencies parsed from gemspec
	dependencies []pubgrub.Term
	// Version from gemspec
	version string
}

// NewPathSource creates a new Path source for a gem
func NewPathSource(path string) (*PathSource, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check that path exists
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("path does not exist: %s", absPath)
	}

	return &PathSource{
		Path:    path,
		AbsPath: absPath,
	}, nil
}

// GetDependencies returns the dependencies for this path gem
func (p *PathSource) GetDependencies(name pubgrub.Name, version pubgrub.Version) ([]pubgrub.Term, error) {
	// If we haven't resolved yet, do it now
	if p.dependencies == nil {
		if err := p.Resolve(); err != nil {
			return nil, err
		}
	}
	return p.dependencies, nil
}

// GetVersions returns a single version for a path gem (from gemspec)
func (p *PathSource) GetVersions(name pubgrub.Name) ([]pubgrub.Version, error) {
	if err := p.Resolve(); err != nil {
		return nil, err
	}

	// Use the version from gemspec
	version, err := NewSemverVersion(p.version)
	if err != nil {
		// If version parsing fails, use a placeholder
		version, _ = NewSemverVersion("0.0.1")
	}

	return []pubgrub.Version{version}, nil
}

// Resolve reads the gemspec from the local path
func (p *PathSource) Resolve() error {
	// Find the gemspec file
	gemspecPath, err := p.findGemspec()
	if err != nil {
		return fmt.Errorf("failed to find gemspec: %w", err)
	}

	// Parse the gemspec to get version and dependencies
	version, deps, err := p.parseGemspec(gemspecPath)
	if err != nil {
		return fmt.Errorf("failed to parse gemspec: %w", err)
	}

	p.version = version
	p.dependencies = deps

	return nil
}

// GetVersion returns the resolved version
func (p *PathSource) GetVersion() string {
	return p.version
}

// findGemspec finds the .gemspec file in the local path
func (p *PathSource) findGemspec() (string, error) {
	// Look for .gemspec files
	matches, err := filepath.Glob(filepath.Join(p.AbsPath, "*.gemspec"))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no gemspec file found in %s", p.AbsPath)
	}

	// Return the first gemspec found
	return matches[0], nil
}

// parseGemspec parses the gemspec file to extract version and dependencies using tree-sitter
func (p *PathSource) parseGemspec(gemspecPath string) (string, []pubgrub.Term, error) {
	// Read gemspec file
	content, err := os.ReadFile(gemspecPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read gemspec: %w", err)
	}

	// Parse with tree-sitter
	parser := gemfile.NewTreeSitterGemspecParser(content)
	gemspec, err := parser.ParseWithTreeSitter()
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse gemspec: %w", err)
	}

	// Convert RuntimeDependencies to PubGrub terms
	var terms []pubgrub.Term
	for _, dep := range gemspec.RuntimeDependencies {
		var condition pubgrub.Condition

		// Convert constraint strings
		if len(dep.Constraints) > 0 {
			constraintStr := strings.Join(dep.Constraints, ", ")
			if constraintStr != "" && constraintStr != ">= 0" {
				semverCond, err := NewSemverCondition(constraintStr)
				if err != nil {
					condition = NewAnyVersionCondition()
				} else {
					condition = semverCond
				}
			} else {
				condition = NewAnyVersionCondition()
			}
		} else {
			condition = NewAnyVersionCondition()
		}

		term := pubgrub.NewTerm(pubgrub.MakeName(dep.Name), condition)
		terms = append(terms, term)
	}

	return gemspec.Version, terms, nil
}

// CopyToVendor copies the path gem to the vendor directory
// This is used during installation
func (p *PathSource) CopyToVendor(destDir string) error {
	// Create destination directory
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	// Copy all files except directories that could cause infinite recursion
	return filepath.Walk(p.AbsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(p.AbsPath, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Skip directories that could cause infinite recursion
		// This prevents copying vendor/bundle/gems/foo into vendor/bundle/gems/foo/vendor/bundle/gems/foo/...
		if info.IsDir() {
			skipDirs := []string{".git", "vendor", ".bundle", "tmp"}
			for _, skip := range skipDirs {
				if info.Name() == skip {
					return filepath.SkipDir
				}
			}
		}

		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy file
		return copyFile(path, destPath, info.Mode())
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string, mode os.FileMode) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, input, mode)
}
