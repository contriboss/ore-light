package resolver

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/ruby"
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
	return GenerateLockfileWithPlatforms(gemfilePath, nil, nil)
}

// GenerateLockfileWithPins resolves gem dependencies with optional version pins.
// versionPins is a map of gem name -> exact version to pin (used for selective updates).
func GenerateLockfileWithPins(gemfilePath string, versionPins map[string]string) error {
	return GenerateLockfileWithPlatforms(gemfilePath, versionPins, nil)
}

// GenerateLockfileWithPlatforms resolves gem dependencies with optional version pins and platforms.
// versionPins is a map of gem name -> exact version to pin (used for selective updates).
// platforms is a list of additional platforms to add to the lockfile (e.g., "x86_64-linux", "java").
func GenerateLockfileWithPlatforms(gemfilePath string, versionPins map[string]string, platforms []string) error {
	// Parse Gemfile
	parser := gemfile.NewGemfileParser(gemfilePath)
	parsed, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", err)
	}

	// Handle gemspec directives
	// Ruby developers: This is like when your Gemfile contains `gemspec`
	// It loads dependencies from the .gemspec file
	if len(parsed.Gemspecs) > 0 {
		if err := loadGemspecDependencies(gemfilePath, parsed); err != nil {
			return fmt.Errorf("failed to load gemspec dependencies: %w", err)
		}
	}

	// Determine lockfile path and platforms early so resolution can honor them.
	lockfilePath := determineLockfilePath(gemfilePath)
	lockPlatforms := detectPlatforms(lockfilePath, platforms)
	if versionPins == nil {
		if pins := loadVersionPinsFromLockfile(lockfilePath); len(pins) > 0 {
			versionPins = pins
		}
	}

	// Determine default source URL from Gemfile sources
	// Respects configured sources, fallback to rubygems.org
	defaultSourceURL := "https://rubygems.org"
	for _, src := range parsed.Sources {
		if src.Type == "rubygems" && src.URL != "" {
			defaultSourceURL = src.URL
			break // Use first rubygems source as default
		}
	}

	// Create RubyGems sources for different gem servers
	// This is like Bundler's source management (rubygems.org, custom mirrors, etc.)
	sources := make(map[string]*RubyGemsSource)
	getSource := func(url string) *RubyGemsSource {
		if url == "" {
			url = defaultSourceURL
		}
		if src, ok := sources[url]; ok {
			return src
		}
		src := NewRubyGemsSourceWithURL(url)
		src.SetRequiredPlatforms(lockPlatforms)
		sources[url] = src
		return src
	}

	// Default source for gems without explicit source
	defaultSource := getSource(defaultSourceURL)

	// Apply version pins to all sources for selective updates
	if versionPins != nil {
		for _, src := range sources {
			src.SetVersionPins(versionPins)
		}
	}

	// Convert Gemfile dependencies to PubGrub terms
	var allSolutions []pubgrub.NameVersion
	seenPackages := make(map[string]pubgrub.Version)
	gemSources := make(map[string]string)  // gem name -> source URL
	gemGroups := make(map[string][]string) // gem name -> groups
	constraintsByGem := make(map[string][]pubgrub.Condition)
	addConstraint := func(name string, cond pubgrub.Condition) {
		if cond == nil {
			return
		}
		constraintsByGem[name] = append(constraintsByGem[name], cond)
	}

	// Track git and path dependencies separately
	var gitSpecs []lockfile.GitGemSpec
	var pathSpecs []lockfile.PathGemSpec
	gitDeps := make(map[string]*gemfile.GemDependency)
	pathDeps := make(map[string]*gemfile.GemDependency)

	quiet := isQuietOutput()
	if !quiet {
		fmt.Printf("Resolving dependencies...\n")
	}
	progress := newResolveProgress(len(parsed.Dependencies))

	// Create a root source for all dependencies
	// The new pubgrub-go uses a root package to collect all requirements
	rootSource := pubgrub.NewRootSource()
	overrides := make(map[string]overrideSpec)

	for _, dep := range parsed.Dependencies {
		if progress != nil && progress.enabled {
			progress.step(dep.Name)
		}

		// Track groups for this dependency
		// Groups determine when gems are installed (e.g., --without development test)
		if len(dep.Groups) > 0 {
			gemGroups[dep.Name] = dep.Groups
		}

		// Check if this is a git dependency
		if dep.Source != nil && dep.Source.Type == "git" {
			if (progress == nil || !progress.enabled) && !quiet {
				fmt.Printf("Resolving %s from git...\n", dep.Name)
			}
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
			gitVersion := gitSource.GetVersion()
			if gitVersion == "" {
				gitVersion = "0.0.1"
			}
			gitSpec := lockfile.GitGemSpec{
				Name:     dep.Name,
				Version:  gitVersion,
				Remote:   dep.Source.URL,
				Revision: gitSource.GetRevision(),
				Branch:   dep.Source.Branch,
				Tag:      dep.Source.Tag,
				Groups:   dep.Groups,
			}

			// Convert dependencies to lockfile format
			var lockfileDeps []lockfile.Dependency
			for _, gitDep := range gitDeps {
				lockDep := lockfileDependencyFromTerm(gitDep)
				lockfileDeps = append(lockfileDeps, lockDep)
				if cond, ok := conditionFromConstraints(lockDep.Constraints); ok {
					addConstraint(lockDep.Name, cond)
				}
			}
			gitSpec.Dependencies = lockfileDeps
			gitSpecs = append(gitSpecs, gitSpec)

			overrides[dep.Name] = overrideSpec{
				version: gitVersion,
				deps:    gitDeps,
			}
			rootSource.AddPackage(pubgrub.MakeName(dep.Name), nil)

			continue
		}

		// Check if this is a path dependency
		if dep.Source != nil && dep.Source.Type == "path" {
			if (progress == nil || !progress.enabled) && !quiet {
				fmt.Printf("Resolving %s from path...\n", dep.Name)
			}
			pathDeps[dep.Name] = &dep

			// Resolve relative paths against the Gemfile directory
			pathURL := dep.Source.URL
			if !filepath.IsAbs(pathURL) {
				pathURL = filepath.Join(filepath.Dir(gemfilePath), pathURL)
			}

			// Create path source and resolve
			pathSource, err := NewPathSource(pathURL)
			if err != nil {
				return fmt.Errorf("failed to create path source for %s: %w", dep.Name, err)
			}

			if err := pathSource.Resolve(); err != nil {
				return fmt.Errorf("failed to resolve path gem %s: %w", dep.Name, err)
			}

			// Get dependencies from the path gem
			pathGemDeps := pathSource.dependencies

			// Create PathGemSpec entry
			pathVersion := pathSource.GetVersion()
			if pathVersion == "" {
				pathVersion = "0.0.1"
			}
			pathSpec := lockfile.PathGemSpec{
				Name:    dep.Name,
				Version: pathVersion,
				Remote:  dep.Source.URL,
				Groups:  dep.Groups,
			}

			// Convert dependencies to lockfile format
			var lockfileDeps []lockfile.Dependency
			for _, pathDep := range pathGemDeps {
				lockDep := lockfileDependencyFromTerm(pathDep)
				lockfileDeps = append(lockfileDeps, lockDep)
				if cond, ok := conditionFromConstraints(lockDep.Constraints); ok {
					addConstraint(lockDep.Name, cond)
				}
			}
			pathSpec.Dependencies = lockfileDeps
			pathSpecs = append(pathSpecs, pathSpec)

			overrides[dep.Name] = overrideSpec{
				version: pathVersion,
				deps:    pathGemDeps,
			}
			rootSource.AddPackage(pubgrub.MakeName(dep.Name), nil)

			continue
		}

		// Determine which source URL to record for this gem
		// Respect configured source, fallback to default
		gemSourceURL := defaultSourceURL + "/"
		if dep.Source != nil && dep.Source.Type == "rubygems" {
			gemSourceURL = dep.Source.URL
			if gemSourceURL != "" {
				// Ensure URL ends with /
				if gemSourceURL[len(gemSourceURL)-1] != '/' {
					gemSourceURL += "/"
				}
			}
		}

		if (progress == nil || !progress.enabled) && !quiet {
			fmt.Printf("Resolving %s from %s...\n", dep.Name, gemSourceURL)
		}

		// Store gem source for later
		gemSources[dep.Name] = gemSourceURL

		// Convert constraints
		var condition pubgrub.Condition

		// Note: version pins are handled by RubyGemsSource.GetVersions()
		// We don't apply them as constraints here to avoid conflicts
		if semverCondition, ok := conditionFromConstraints(dep.Constraints); ok {
			condition = semverCondition
			addConstraint(dep.Name, semverCondition)
		} else {
			// No constraints - accept any version
			condition = NewAnyVersionCondition()
		}

		// Add dependency to root source
		rootSource.AddPackage(pubgrub.MakeName(dep.Name), condition)
	}

	if len(overrides) > 0 {
		defaultSource.SetOverrides(overrides)
	}

	// Create unified solver with root source and gem source
	// This resolves all dependencies together with proper conflict resolution
	// Enable incompatibility tracking for detailed error messages
	solverOptions := []pubgrub.SolverOption{
		pubgrub.WithIncompatibilityTracking(true),
		pubgrub.WithPreferHighestVersions(true),
	}
	if os.Getenv("ORE_PUBGRUB_DEBUG") != "" {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		solverOptions = append(solverOptions, pubgrub.WithLogger(logger))
	}
	unifiedSolver := pubgrub.NewSolverWithOptions(
		[]pubgrub.Source{rootSource, defaultSource},
		solverOptions...,
	)

	// Solve all dependencies at once
	solution, err := unifiedSolver.Solve(rootSource.Term())
	if err != nil {
		return fmt.Errorf(`could not resolve dependencies

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

		// Inherit source from dependencies (use default if not set)
		if gemSources[pkgName] == "" {
			gemSources[pkgName] = defaultSourceURL + "/"
		}
	}

	// Sort solutions by name for consistent output
	sort.Slice(allSolutions, func(i, j int) bool {
		return allSolutions[i].Name.Value() < allSolutions[j].Name.Value()
	})

	baseVersions := make(map[string]string, len(allSolutions))
	for _, pkg := range allSolutions {
		baseVersions[pkg.Name.Value()] = pkg.Version.String()
	}

	existingPlatformVersions := loadExistingPlatformVersions(lockfilePath)

	// Convert to lockfile specs and fetch dependencies
	depSource := NewRubyGemsSource()
	specs := make([]lockfile.GemSpec, 0, len(allSolutions))
	skipSpecs := make(map[string]bool, len(gitDeps)+len(pathDeps)+1)
	for name := range gitDeps {
		skipSpecs[name] = true
	}
	for name := range pathDeps {
		skipSpecs[name] = true
	}
	skipSpecs["bundler"] = true

	for _, pkg := range allSolutions {
		gemName := pkg.Name.Value()
		version := pkg.Version.String()
		if skipSpecs[gemName] {
			continue
		}

		var lockfileDeps []lockfile.Dependency
		if depSource.compactSource != nil {
			if depsMap, err := depSource.compactSource.GetDependenciesMap(gemName, version, ""); err == nil {
				lockfileDeps = dependenciesFromCompactMap(depsMap)
			}
		}
		if lockfileDeps == nil {
			deps, depsErr := depSource.GetDependencies(pkg.Name, pkg.Version)
			if depsErr != nil {
				deps = []pubgrub.Term{}
			}

			for _, dep := range deps {
				lockDep := lockfileDependencyFromTerm(dep)
				lockfileDeps = append(lockfileDeps, lockDep)
			}
		}

		for _, lockDep := range lockfileDeps {
			if cond, ok := conditionFromConstraints(lockDep.Constraints); ok {
				addConstraint(lockDep.Name, cond)
			}
		}

		specs = append(specs, lockfile.GemSpec{
			Name:         gemName,
			Version:      version,
			Dependencies: lockfileDeps,
			SourceURL:    gemSources[gemName],
			Groups:       gemGroups[gemName], // Track which groups this gem belongs to
		})
	}

	platformSpecs, platformSpecNames := buildPlatformSpecs(depSource.compactSource, specs, lockPlatforms, constraintsByGem, gemSources, gemGroups, versionPins, baseVersions, existingPlatformVersions)
	if len(platformSpecs) > 0 {
		specs = append(specs, platformSpecs...)
	}

	if len(platformSpecNames) > 0 || len(skipSpecs) > 0 {
		filtered := make([]lockfile.GemSpec, 0, len(specs))
		for _, spec := range specs {
			if skipSpecs[spec.Name] {
				continue
			}
			if spec.Platform == "" && platformSpecNames[spec.Name] {
				continue
			}
			filtered = append(filtered, spec)
		}
		specs = filtered
	}

	checksums := buildLockfileChecksums(specs, gitSpecs, pathSpecs, gemSources, sources, defaultSourceURL)

	// Build Lockfile structure
	lock := &lockfile.Lockfile{
		GemSpecs:  specs,
		GitSpecs:  gitSpecs,
		PathSpecs: pathSpecs,
		Platforms: lockPlatforms,
		Checksums: checksums,
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

// determineLockfilePath determines the lockfile path based on the Gemfile path.
// Supports both Gemfile/Gemfile.lock and gems.rb/gems.locked naming conventions.
func determineLockfilePath(gemfilePath string) string {
	base := filepath.Base(gemfilePath)
	dir := filepath.Dir(gemfilePath)

	if base == "gems.rb" {
		return filepath.Join(dir, "gems.locked")
	}
	return gemfilePath + ".lock"
}

// detectPlatforms detects the current platform(s) for the lockfile.
// Bundler lockfiles typically include:
// 1. "ruby" - for platform-independent gems
// 2. Current platform (e.g., "arm64-darwin-24", "x86_64-linux")
// 3. Any existing platforms from previous lockfile
// 4. Additional platforms specified via --add-platform flag
func detectPlatforms(lockfilePath string, additionalPlatforms []string) []string {
	platformSet := make(map[string]bool)
	addPlatform := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		platformSet[p] = true
	}

	// Read existing platforms from lockfile if it exists.
	// This mirrors Bundler behavior, which preserves lockfile platforms by default.
	if _, err := os.Stat(lockfilePath); err == nil {
		if file, err := os.Open(lockfilePath); err == nil {
			defer func() {
				_ = file.Close()
			}()
			if parsed, err := lockfile.Parse(file); err == nil {
				for _, p := range parsed.Platforms {
					addPlatform(p)
				}
			}
		}
	}

	// Always include the current Ruby platform (Bundler keeps it alongside existing platforms).
	cmd := exec.Command("ruby", "-e", "puts RUBY_PLATFORM")
	output, err := cmd.Output()
	if err == nil {
		platform := regexp.MustCompile(`\s+`).ReplaceAllString(string(output), "")
		addPlatform(platform)
	}

	// Add additional platforms from --add-platform flags
	for _, p := range additionalPlatforms {
		addPlatform(p)
	}

	// Convert set to sorted slice for consistent output
	platforms := make([]string, 0, len(platformSet))
	for p := range platformSet {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

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

	// Fallback to default Bundler version (same as RubyGems since 4.0)
	return ruby.DefaultRubyGemsVersion
}

func loadVersionPinsFromLockfile(lockfilePath string) map[string]string {
	lf, err := lockfile.ParseFile(lockfilePath)
	if err != nil || lf == nil {
		return nil
	}

	pins := make(map[string]string)
	for _, spec := range lf.GemSpecs {
		if spec.Name == "" || spec.Version == "" {
			continue
		}
		pins[spec.Name] = spec.Version
	}

	return pins
}

// loadGemspecDependencies loads dependencies from .gemspec files referenced by gemspec directives.
// Ruby developers: This is equivalent to evaluating the `gemspec` directive in your Gemfile.
func loadGemspecDependencies(gemfilePath string, parsed *gemfile.ParsedGemfile) error {
	gemfileDir := filepath.Dir(gemfilePath)

	// Track gemspec names to filter out the gem itself from dependencies
	gemspecNames := make(map[string]bool)

	for _, gemspecRef := range parsed.Gemspecs {
		// Resolve the search path relative to the Gemfile
		searchPath := gemspecRef.Path
		if !filepath.IsAbs(searchPath) {
			searchPath = filepath.Join(gemfileDir, searchPath)
		}

		// Find gemspec files using the glob pattern
		gemspecFiles, err := findGemspecFiles(searchPath, gemspecRef.Glob, gemspecRef.Name)
		if err != nil {
			return fmt.Errorf("failed to find gemspec files in %s: %w", searchPath, err)
		}

		if len(gemspecFiles) == 0 {
			return fmt.Errorf("no gemspec files found in %s", searchPath)
		}

		// Parse each gemspec file and merge dependencies
		for _, gemspecPath := range gemspecFiles {
			fmt.Printf("Loading dependencies from %s...\n", filepath.Base(gemspecPath))

			gemspecParser := gemfile.NewGemspecParser(gemspecPath)
			gemspecFile, err := gemspecParser.Parse()
			if err != nil {
				return fmt.Errorf("failed to parse gemspec %s: %w", gemspecPath, err)
			}

			// Track the gemspec name itself
			gemspecNames[gemspecFile.Name] = true

			// Add runtime dependencies to the main dependency list
			for _, dep := range gemspecFile.RuntimeDependencies {
				// Avoid duplicates - if a gem is already explicitly declared, keep that version
				if !isDependencyDeclared(parsed.Dependencies, dep.Name) {
					parsed.Dependencies = append(parsed.Dependencies, dep)
				}
			}

			// Add development dependencies with the specified group
			devGroup := gemspecRef.DevelopmentGroup
			if devGroup == "" {
				devGroup = "development"
			}

			for _, dep := range gemspecFile.DevelopmentDependencies {
				// Set the development group
				dep.Groups = []string{devGroup}

				// Avoid duplicates
				if !isDependencyDeclared(parsed.Dependencies, dep.Name) {
					parsed.Dependencies = append(parsed.Dependencies, dep)
				}
			}
		}
	}

	// Filter out the gemspec gem itself from the dependencies list
	// gemfile-go adds it as a path dependency, but we don't want to resolve it
	filtered := make([]gemfile.GemDependency, 0, len(parsed.Dependencies))
	for _, dep := range parsed.Dependencies {
		if !gemspecNames[dep.Name] {
			filtered = append(filtered, dep)
		}
	}
	parsed.Dependencies = filtered

	return nil
}

// findGemspecFiles finds .gemspec files in the given directory matching the glob pattern.
func findGemspecFiles(searchPath, globPattern, specificName string) ([]string, error) {
	// If a specific name is provided, look for that exact gemspec
	if specificName != "" {
		gemspecPath := filepath.Join(searchPath, specificName+".gemspec")
		if _, err := os.Stat(gemspecPath); err == nil {
			return []string{gemspecPath}, nil
		}
		return nil, fmt.Errorf("gemspec %s.gemspec not found", specificName)
	}

	// Otherwise, use the glob pattern to find gemspec files
	pattern := filepath.Join(searchPath, "*.gemspec")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	return matches, nil
}

// isDependencyDeclared checks if a gem is already explicitly declared in the dependencies list.
func isDependencyDeclared(dependencies []gemfile.GemDependency, gemName string) bool {
	for _, dep := range dependencies {
		if dep.Name == gemName {
			return true
		}
	}
	return false
}
