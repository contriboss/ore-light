package resolver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

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

// parseGemspec parses the gemspec file to extract version and dependencies
func (p *PathSource) parseGemspec(gemspecPath string) (string, []pubgrub.Term, error) {
	// Use Ruby to parse the gemspec and output version + dependencies as JSON
	rubyScript := `
require 'rubygems'
require 'json'

spec = Gem::Specification.load(ARGV[0])
deps = spec.dependencies.select { |d| d.type == :runtime }.map do |d|
  {
    name: d.name,
    requirements: d.requirement.to_s
  }
end

result = {
  version: spec.version.to_s,
  dependencies: deps
}

puts JSON.generate(result)
`

	cmd := exec.Command("ruby", "-e", rubyScript, gemspecPath)
	output, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse gemspec with Ruby: %w", err)
	}

	// Parse the JSON output
	var result struct {
		Version      string `json:"version"`
		Dependencies []struct {
			Name         string `json:"name"`
			Requirements string `json:"requirements"`
		} `json:"dependencies"`
	}

	if err := parsePathJSON(output, &result); err != nil {
		return "", nil, fmt.Errorf("failed to parse Ruby output: %w", err)
	}

	// Convert dependencies to PubGrub terms
	var terms []pubgrub.Term
	for _, dep := range result.Dependencies {
		var condition pubgrub.Condition
		if dep.Requirements != "" && dep.Requirements != ">= 0" {
			semverCond, err := NewSemverCondition(dep.Requirements)
			if err != nil {
				condition = NewAnyVersionCondition()
			} else {
				condition = semverCond
			}
		} else {
			condition = NewAnyVersionCondition()
		}

		term := pubgrub.NewTerm(pubgrub.MakeName(dep.Name), condition)
		terms = append(terms, term)
	}

	return result.Version, terms, nil
}

// parsePathJSON is a simple JSON parser for path gem data
func parsePathJSON(data []byte, result interface{}) error {
	str := string(data)

	switch ptr := result.(type) {
	case *struct {
		Version      string `json:"version"`
		Dependencies []struct {
			Name         string `json:"name"`
			Requirements string `json:"requirements"`
		} `json:"dependencies"`
	}:
		// Extract version
		versionRe := regexp.MustCompile(`"version"\s*:\s*"([^"]+)"`)
		if match := versionRe.FindStringSubmatch(str); len(match) > 1 {
			ptr.Version = match[1]
		}

		// Extract dependencies array
		depsRe := regexp.MustCompile(`"dependencies"\s*:\s*\[([^\]]*)\]`)
		depsMatch := depsRe.FindStringSubmatch(str)
		if len(depsMatch) > 1 {
			depsStr := depsMatch[1]

			// Parse individual dependency objects
			objRe := regexp.MustCompile(`\{[^}]+\}`)
			objects := objRe.FindAllString(depsStr, -1)

			nameRe := regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)
			reqRe := regexp.MustCompile(`"requirements"\s*:\s*"([^"]*)"`)

			for _, obj := range objects {
				nameMatch := nameRe.FindStringSubmatch(obj)
				reqMatch := reqRe.FindStringSubmatch(obj)

				if len(nameMatch) > 1 {
					dep := struct {
						Name         string `json:"name"`
						Requirements string `json:"requirements"`
					}{
						Name: nameMatch[1],
					}
					if len(reqMatch) > 1 {
						dep.Requirements = reqMatch[1]
					}
					ptr.Dependencies = append(ptr.Dependencies, dep)
				}
			}
		}

		return nil
	default:
		return fmt.Errorf("unsupported type for JSON parsing")
	}
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
