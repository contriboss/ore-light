package resolver

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/pubgrub-go"
)

// GenerateLockfile resolves gem dependencies and writes a lockfile.
//
// Ruby developers: This is like running `bundle lock` or `bundle install`
// Resolves all gem dependencies using PubGrub algorithm and writes Gemfile.lock
//
// Why this exists: Bundler is Ruby-specific. We need a Go implementation
// that can resolve dependencies without Ruby installed. PubGrub is the
// state-of-the-art dependency resolution algorithm (also used by Dart's pub).
func GenerateLockfile(gemfilePath string) error {
	return GenerateLockfileWithPins(gemfilePath, nil)
}

// GenerateLockfileWithPins resolves gem dependencies with optional version pins.
// versionPins is a map of gem name -> exact version to pin (used for selective updates).
func GenerateLockfileWithPins(gemfilePath string, versionPins map[string]string) error {
	// Parse Gemfile
	parser := gemfile.NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", err)
	}

	// Create RubyGems sources for different gem servers
	// This is like Bundler's source management (rubygems.org, custom mirrors, etc.)
	sources := make(map[string]*RubyGemsSource)
	getSource := func(url string) *RubyGemsSource {
		if url == "" {
			url = "https://rubygems.org"
		}
		if src, ok := sources[url]; ok {
			return src
		}
		src := NewRubyGemsSourceWithURL(url)
		sources[url] = src
		return src
	}

	// Default source for gems without explicit source
	defaultSource := getSource("https://rubygems.org")

	// Convert Gemfile dependencies to PubGrub terms
	var allSolutions []pubgrub.NameVersion
	seenPackages := make(map[string]pubgrub.Version)
	gemSources := make(map[string]string)  // gem name -> source URL
	gemGroups := make(map[string][]string) // gem name -> groups

	// Track git and path dependencies separately
	var gitSpecs []lockfile.GitGemSpec
	var pathSpecs []lockfile.PathGemSpec
	gitDeps := make(map[string]*gemfile.GemDependency)
	pathDeps := make(map[string]*gemfile.GemDependency)

	fmt.Printf("Resolving dependencies...\n")

	// Create a root source for all dependencies
	// The new pubgrub-go uses a root package to collect all requirements
	rootSource := pubgrub.NewRootSource()
	var regularDepTerms []pubgrub.Term

	for _, dep := range parsed.Dependencies {
		// Track groups for this dependency
		// Groups determine when gems are installed (e.g., --without development test)
		if len(dep.Groups) > 0 {
			gemGroups[dep.Name] = dep.Groups
		}

		// Check if this is a git dependency
		if dep.Source != nil && dep.Source.Type == "git" {
			fmt.Printf("Resolving %s from git...\n", dep.Name)
			gitDeps[dep.Name] = &dep

			// Create git source and resolve
			gitSource, err := NewGitSource(dep.Source.URL, dep.Source.Branch, dep.Source.Tag, dep.Source.Ref)
			if err != nil {
				return fmt.Errorf("failed to create git source for %s: %w", dep.Name, err)
			}

			if err := gitSource.Resolve(); err != nil {
				return fmt.Errorf("failed to resolve git gem %s: %w", dep.Name, err)
			}

			// Get dependencies from the git gem
			gitDeps := gitSource.dependencies

			// Create GitGemSpec entry
			gitSpec := lockfile.GitGemSpec{
				Name:     dep.Name,
				Version:  "0.0.1", // Placeholder version
				Remote:   dep.Source.URL,
				Revision: gitSource.GetRevision(),
				Branch:   dep.Source.Branch,
				Tag:      dep.Source.Tag,
				Groups:   dep.Groups,
			}

			// Convert dependencies to lockfile format
			var lockfileDeps []lockfile.Dependency
			for _, gitDep := range gitDeps {
				lockfileDeps = append(lockfileDeps, lockfile.Dependency{
					Name: gitDep.Name.Value(),
				})
			}
			gitSpec.Dependencies = lockfileDeps
			gitSpecs = append(gitSpecs, gitSpec)

			// Add transitive dependencies from git gem to regular solver
			regularDepTerms = append(regularDepTerms, gitDeps...)

			continue
		}

		// Check if this is a path dependency
		if dep.Source != nil && dep.Source.Type == "path" {
			fmt.Printf("Resolving %s from path...\n", dep.Name)
			pathDeps[dep.Name] = &dep

			// Create path source and resolve
			pathSource, err := NewPathSource(dep.Source.URL)
			if err != nil {
				return fmt.Errorf("failed to create path source for %s: %w", dep.Name, err)
			}

			if err := pathSource.Resolve(); err != nil {
				return fmt.Errorf("failed to resolve path gem %s: %w", dep.Name, err)
			}

			// Get dependencies from the path gem
			pathGemDeps := pathSource.dependencies

			// Create PathGemSpec entry
			pathSpec := lockfile.PathGemSpec{
				Name:    dep.Name,
				Version: pathSource.GetVersion(),
				Remote:  dep.Source.URL,
				Groups:  dep.Groups,
			}

			// Convert dependencies to lockfile format
			var lockfileDeps []lockfile.Dependency
			for _, pathDep := range pathGemDeps {
				lockfileDeps = append(lockfileDeps, lockfile.Dependency{
					Name: pathDep.Name.Value(),
				})
			}
			pathSpec.Dependencies = lockfileDeps
			pathSpecs = append(pathSpecs, pathSpec)

			// Add transitive dependencies from path gem to regular solver
			regularDepTerms = append(regularDepTerms, pathGemDeps...)

			continue
		}

		// Determine which source URL to record for this gem
		gemSourceURL := "https://rubygems.org/"
		if dep.Source != nil && dep.Source.Type == "rubygems" {
			gemSourceURL = dep.Source.URL
			if gemSourceURL != "" {
				// Ensure URL ends with /
				if gemSourceURL[len(gemSourceURL)-1] != '/' {
					gemSourceURL += "/"
				}
			}
		}

		fmt.Printf("Resolving %s from %s...\n", dep.Name, gemSourceURL)

		// Store gem source for later
		gemSources[dep.Name] = gemSourceURL

		// Convert constraints
		var condition pubgrub.Condition

		// Check if this gem has a pinned version (for selective updates)
		if pinnedVersion, pinned := versionPins[dep.Name]; pinned {
			// Pin to exact version
			semverCondition, err := NewSemverCondition("= " + pinnedVersion)
			if err != nil {
				// If we can't parse, use any version
				condition = NewAnyVersionCondition()
			} else {
				condition = semverCondition
			}
		} else if len(dep.Constraints) > 0 {
			// Combine multiple constraints with ", " (semver library supports compound constraints)
			// Example: [">= 1.0", "< 2.0"] becomes ">= 1.0, < 2.0"
			constraintStr := strings.Join(dep.Constraints, ", ")
			semverCondition, err := NewSemverCondition(constraintStr)
			if err != nil {
				// If we can't parse, use any version
				condition = NewAnyVersionCondition()
			} else {
				condition = semverCondition
			}
		} else {
			// No constraints - accept any version
			condition = NewAnyVersionCondition()
		}

		// Add dependency to root source
		rootSource.AddPackage(pubgrub.MakeName(dep.Name), condition)
	}

	// Add transitive dependencies from git/path gems to root source
	for _, term := range regularDepTerms {
		rootSource.AddPackage(term.Name, term.Condition)
	}

	// Create unified solver with root source and gem source
	// This resolves all dependencies together with proper conflict resolution
	unifiedSolver := pubgrub.NewSolver(rootSource, defaultSource)

	// Solve all dependencies at once
	solution, err := unifiedSolver.Solve(rootSource.Term())
	if err != nil {
		return fmt.Errorf(`Could not resolve dependencies

  This could mean:
  - No versions satisfy the constraints
  - Conflicting version requirements from dependencies

  Original error: %w`, err)
	}

	// Collect all solved packages (excluding the root package)
	rootName := pubgrub.MakeName("$$root")
	for _, pkg := range solution {
		// Skip the synthetic root package
		if pkg.Name == rootName {
			continue
		}

		pkgName := pkg.Name.Value()
		seenPackages[pkgName] = pkg.Version
		allSolutions = append(allSolutions, pkg)

		// Inherit source from dependencies
		if gemSources[pkgName] == "" {
			gemSources[pkgName] = "https://rubygems.org/"
		}
	}

	// Sort solutions by name for consistent output
	sort.Slice(allSolutions, func(i, j int) bool {
		return allSolutions[i].Name.Value() < allSolutions[j].Name.Value()
	})

	// Determine lockfile path
	lockfilePath := gemfilePath + ".lock"

	// Convert to lockfile specs and fetch dependencies
	depSource := NewRubyGemsSource()
	specs := make([]lockfile.GemSpec, len(allSolutions))
	for i, pkg := range allSolutions {
		gemName := pkg.Name.Value()
		version := pkg.Version.String()

		// Get dependencies for this gem
		deps, depsErr := depSource.GetDependencies(pkg.Name, pkg.Version)
		if depsErr != nil {
			// If we can't fetch dependencies, continue without them
			deps = []pubgrub.Term{}
		}

		// Convert dependencies to lockfile format
		var lockfileDeps []lockfile.Dependency
		for _, dep := range deps {
			// Extract constraint string from Condition using String() method
			var constraints []string
			if dep.Condition != nil && dep.Condition.String() != ">= 0" {
				constraints = []string{dep.Condition.String()}
			}
			lockfileDeps = append(lockfileDeps, lockfile.Dependency{
				Name:        dep.Name.Value(),
				Constraints: constraints,
			})
		}

		specs[i] = lockfile.GemSpec{
			Name:         gemName,
			Version:      version,
			Dependencies: lockfileDeps,
			SourceURL:    gemSources[gemName],
			Groups:       gemGroups[gemName], // Track which groups this gem belongs to
		}
	}

	// Build Lockfile structure
	lock := &lockfile.Lockfile{
		GemSpecs:  specs,
		GitSpecs:  gitSpecs,
		PathSpecs: pathSpecs,
		Platforms: detectPlatforms(),
		Dependencies: func() []lockfile.Dependency {
			var deps []lockfile.Dependency
			for _, dep := range parsed.Dependencies {
				deps = append(deps, lockfile.Dependency{
					Name:        dep.Name,
					Constraints: dep.Constraints,
				})
			}
			return deps
		}(),
		BundledWith: detectBundlerVersion(lockfilePath),
	}

	// Write lockfile
	writer := lockfile.NewLockfileWriter()
	if err := writer.WriteFile(lock, lockfilePath); err != nil {
		return fmt.Errorf("failed to write lockfile: %w", err)
	}

	fmt.Printf("\n✨ Resolved %d dependencies and wrote %d gems to %s\n", len(parsed.Dependencies), len(specs), lockfilePath)
	return nil
}

// detectPlatforms detects the current platform(s) for the lockfile.
// Bundler lockfiles typically include:
// 1. "ruby" - for platform-independent gems
// 2. Current platform (e.g., "arm64-darwin-24", "x86_64-linux")
func detectPlatforms() []string {
	platforms := []string{"ruby"}

	// Try to get Ruby platform
	cmd := exec.Command("ruby", "-e", "puts RUBY_PLATFORM")
	output, err := cmd.Output()
	if err == nil {
		platform := regexp.MustCompile(`\s+`).ReplaceAllString(string(output), "")
		if platform != "" && platform != "ruby" {
			platforms = append(platforms, platform)
		}
	}

	return platforms
}

// detectBundlerVersion attempts to detect the Bundler version from:
// 1. Existing Gemfile.lock's BUNDLED WITH section (if exists)
// 2. Running `bundle --version` and parsing output
// 3. Fallback to a reasonable default
func detectBundlerVersion(lockfilePath string) string {
	// Try to read existing lockfile
	if _, err := os.Stat(lockfilePath); err == nil {
		if existingLock, err := lockfile.ParseFile(lockfilePath); err == nil {
			if existingLock.BundledWith != "" {
				return existingLock.BundledWith
			}
		}
	}

	// Try running bundle --version
	cmd := exec.Command("bundle", "--version")
	output, err := cmd.Output()
	if err == nil {
		// Parse output like "Bundler version 2.5.23"
		versionRegex := regexp.MustCompile(`Bundler version (\d+\.\d+\.\d+)`)
		if matches := versionRegex.FindStringSubmatch(string(output)); len(matches) > 1 {
			return matches[1]
		}
	}

	// Fallback to DEFAULT_BUNDLER_VERSION constant
	// Note: This should match the constant in cmd/ore/main.go
	return "2.7.2"
}
