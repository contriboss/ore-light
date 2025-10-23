package resolver

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/tinyrange/tinyrange/experimental/pubgrub"
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
	gemSources := make(map[string]string) // gem name -> source URL

	// Track git and path dependencies separately
	var gitSpecs []lockfile.GitGemSpec
	var pathSpecs []lockfile.PathGemSpec
	gitDeps := make(map[string]*gemfile.GemDependency)
	pathDeps := make(map[string]*gemfile.GemDependency)

	fmt.Printf("Resolving dependencies...\n")

	for _, dep := range parsed.Dependencies {
		// Skip development/test gems for MVP (can be added later)
		// TODO: Add group support

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
					Name: string(gitDep.Name),
				})
			}
			gitSpec.Dependencies = lockfileDeps
			gitSpecs = append(gitSpecs, gitSpec)

			// Resolve transitive dependencies from git gem
			gitSolver := pubgrub.NewSolver(defaultSource)
			for _, gitDep := range gitDeps {
				solution, err := gitSolver.Solve(gitDep)
				if err != nil {
					return fmt.Errorf("failed to resolve dependency %s of git gem %s: %w", gitDep.Name, dep.Name, err)
				}

				// Merge solutions
				for _, pkg := range solution {
					pkgName := string(pkg.Name)
					if existingVer, seen := seenPackages[pkgName]; seen {
						if pkg.Version.Sort(existingVer) > 0 {
							seenPackages[pkgName] = pkg.Version
						}
					} else {
						seenPackages[pkgName] = pkg.Version
						allSolutions = append(allSolutions, pkg)
					}
				}
			}

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
					Name: string(pathDep.Name),
				})
			}
			pathSpec.Dependencies = lockfileDeps
			pathSpecs = append(pathSpecs, pathSpec)

			// Resolve transitive dependencies from path gem
			pathSolver := pubgrub.NewSolver(defaultSource)
			for _, pathDep := range pathGemDeps {
				solution, err := pathSolver.Solve(pathDep)
				if err != nil {
					return fmt.Errorf("failed to resolve dependency %s of path gem %s: %w", pathDep.Name, dep.Name, err)
				}

				// Merge solutions
				for _, pkg := range solution {
					pkgName := string(pkg.Name)
					if existingVer, seen := seenPackages[pkgName]; seen {
						if pkg.Version.Sort(existingVer) > 0 {
							seenPackages[pkgName] = pkg.Version
						}
					} else {
						seenPackages[pkgName] = pkg.Version
						allSolutions = append(allSolutions, pkg)
					}
				}
			}

			continue
		}

		// Determine which source to use for this gem
		gemSource := defaultSource
		gemSourceURL := "https://rubygems.org/"
		if dep.Source != nil && dep.Source.Type == "rubygems" {
			gemSourceURL = dep.Source.URL
			if gemSourceURL != "" {
				gemSource = getSource(gemSourceURL)
				// Ensure URL ends with /
				if gemSourceURL[len(gemSourceURL)-1] != '/' {
					gemSourceURL += "/"
				}
			}
		}

		fmt.Printf("Resolving %s from %s...\n", dep.Name, gemSourceURL)

		// Convert constraints
		var condition pubgrub.Condition
		if len(dep.Constraints) > 0 {
			// For now, use the first constraint
			// TODO: Handle multiple constraints
			semverCondition, err := NewSemverCondition(dep.Constraints[0])
			if err != nil {
				// If we can't parse, use any version
				condition = &AnyVersionCondition{}
			} else {
				condition = semverCondition
			}
		} else {
			// No constraints - accept any version
			condition = &AnyVersionCondition{}
		}

		term := pubgrub.NewTerm(pubgrub.Name(dep.Name), condition)

		// Solve for this dependency using the gem's source
		gemSolver := pubgrub.NewSolver(gemSource)
		solution, err := gemSolver.Solve(term)
		if err != nil {
			// Provide helpful error context
			constraintStr := "any version"
			if len(dep.Constraints) > 0 {
				constraintStr = dep.Constraints[0]
			}

			return fmt.Errorf(`Could not resolve dependency: %s
  Constraint: %s

  This could mean:
  - No versions of %s satisfy the constraint %s
  - Conflicting version requirements from other dependencies

  Try: ore add %s (without version constraint) to see available versions

  Original error: %w`, dep.Name, constraintStr, dep.Name, constraintStr, dep.Name, err)
		}

		// Merge solutions, keeping track of seen packages and their sources
		for _, pkg := range solution {
			pkgName := string(pkg.Name)
			if existingVer, seen := seenPackages[pkgName]; seen {
				// If we've seen this package, keep the higher version
				// TODO: This is a simplification - proper resolver should handle conflicts
				if pkg.Version.Sort(existingVer) > 0 {
					seenPackages[pkgName] = pkg.Version
					gemSources[pkgName] = gemSourceURL
				}
			} else {
				seenPackages[pkgName] = pkg.Version
				gemSources[pkgName] = gemSourceURL
				allSolutions = append(allSolutions, pkg)
			}
		}
	}

	// Sort solutions by name for consistent output
	sort.Slice(allSolutions, func(i, j int) bool {
		return string(allSolutions[i].Name) < string(allSolutions[j].Name)
	})

	// Determine lockfile path
	lockfilePath := gemfilePath + ".lock"

	// Convert to lockfile specs and fetch dependencies
	depSource := NewRubyGemsSource()
	specs := make([]lockfile.GemSpec, len(allSolutions))
	for i, pkg := range allSolutions {
		gemName := string(pkg.Name)
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
			lockfileDeps = append(lockfileDeps, lockfile.Dependency{
				Name: string(dep.Name),
				// TODO: Extract constraints from Condition
			})
		}

		specs[i] = lockfile.GemSpec{
			Name:         gemName,
			Version:      version,
			Dependencies: lockfileDeps,
			SourceURL:    gemSources[gemName],
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

	// Fallback to reasonable default
	return "2.5.0"
}
