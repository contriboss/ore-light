package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/extensions"
	"github.com/contriboss/ore-light/internal/ruby"
)

// DownloadManager interface for dependency injection
type DownloadManager interface {
	CheckSourceHealth(ctx context.Context)
	DownloadAll(ctx context.Context, gems []lockfile.GemSpec, force bool) (DownloadReport, error)
	CacheDir() string
}

// DownloadReport contains statistics about the download operation
type DownloadReport struct {
	Downloaded int
	Skipped    int
}

// InstallReport contains statistics about gem installation
type InstallReport struct {
	Installed        int
	Skipped          int
	ExtensionsBuilt  int
	ExtensionsFailed int
}

// InstallCallbacks contains dependencies that RunInstall needs from main
type InstallCallbacks struct {
	GetDownloadManager  func(workers int) (DownloadManager, error)
	GetDefaultVendorDir func() string
	InstallFromCache    func(ctx context.Context, cacheDir, vendorDir string, gems []lockfile.GemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error)
	InstallGitGems      func(ctx context.Context, vendorDir, rubyScope string, gitSpecs []lockfile.GitGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error)
	InstallPathGems     func(ctx context.Context, vendorDir, rubyScope string, pathSpecs []lockfile.PathGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig, lockfileDir string) (InstallReport, error)
}

// RunInstall implements the ore install command
func RunInstall(args []string, callbacks InstallCallbacks) error {
	startTime := time.Now()

	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", "", "Path to Gemfile (used to derive lockfile path)")
	lockfilePath := fs.String("lockfile", "", "Path to Gemfile.lock")
	workers := fs.Int("workers", defaultDownloadWorkers(), "Number of concurrent downloads")
	force := fs.Bool("force", false, "Re-download or reinstall even if artifacts exist")
	vendorDir := fs.String("vendor", callbacks.GetDefaultVendorDir(), "Destination directory for installed gems")
	skipExtensions := fs.Bool("skip-extensions", false, "Skip building native extensions")
	buildExts := fs.Bool("build-extensions", false, "Force building native extensions even for already-installed gems")
	verbose := fs.Bool("verbose", false, "Enable verbose output including extension build logs")
	without := fs.String("without", "", "Comma-separated list of groups to exclude (e.g., development,test)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve effective lockfile path
	effectiveLockfilePath := resolveLockfilePathWithDerivedFallback(*gemfilePath, *lockfilePath)

	quiet := quietOutput()

	// Debug: show which lockfile we're using
	if !quiet || os.Getenv("ORE_DEBUG") != "" {
		fmt.Printf("DEBUG: BUNDLE_GEMFILE=%s\n", os.Getenv("BUNDLE_GEMFILE"))
		fmt.Printf("DEBUG: Using lockfile: %s\n", effectiveLockfilePath)
	}

	dm, err := callbacks.GetDownloadManager(*workers)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Perform pre-flight health checks on gem sources
	dm.CheckSourceHealth(ctx)

	// Load both regular gems and git gems from lockfile
	parsed, err := loadOrGenerateLockfile(effectiveLockfilePath, quiet, *vendorDir)
	if err != nil {
		return err
	}

	// Debug: show parsed gem counts
	if !quiet || os.Getenv("ORE_DEBUG") != "" {
		fmt.Printf("DEBUG: Parsed lockfile - GemSpecs: %d, GitSpecs: %d, PathSpecs: %d\n",
			len(parsed.GemSpecs), len(parsed.GitSpecs), len(parsed.PathSpecs))
		if len(parsed.GemSpecs) > 0 {
			fmt.Printf("DEBUG: First few gems: ")
			for i, g := range parsed.GemSpecs {
				if i >= 5 {
					break
				}
				fmt.Printf("%s ", g.FullName())
			}
			fmt.Println()
		}
	}

	if len(parsed.GemSpecs) == 0 && len(parsed.GitSpecs) == 0 {
		fmt.Println("No gems found in lockfile.")
		return nil
	}

	// Parse excluded groups from --without flag
	var excludeGroups []string
	if *without != "" {
		excludeGroups = parseGroupList(*without)
		if *verbose {
			fmt.Printf("Excluding groups: %v\n", excludeGroups)
		}

		// If filtering by groups, we need to load the Gemfile to get group information
		effectiveGemfilePath := *gemfilePath
		if effectiveGemfilePath == "" {
			effectiveGemfilePath = detectGemfileFromLock(effectiveLockfilePath)
		}
		if effectiveGemfilePath == "" {
			effectiveGemfilePath = "Gemfile"
		}

		if err := enrichGemsWithGroups(effectiveGemfilePath, parsed); err != nil {
			if *verbose {
				fmt.Fprintf(os.Stderr, "Warning: could not load Gemfile for group filtering: %v\n", err)
				fmt.Fprintf(os.Stderr, "Proceeding without group filtering.\n")
			}
			excludeGroups = nil // Disable filtering if we can't read the Gemfile
		}
	}

	// Filter and deduplicate GemSpecs
	gems := deduplicateGemSpecs(parsed.GemSpecs)
	if len(excludeGroups) > 0 {
		// Filter by groups - only keep direct dependencies with allowed groups
		gems = filterGemsByGroupsAndDependencies(gems, parsed.GemSpecs, excludeGroups)
	}

	// Filter by current platform
	gems = filterGemsByPlatform(gems)

	// Download regular gems from rubygems.org
	// Note: Engine compatibility filtering happens during installation
	// after extracting metadata (which contains extension info)
	if len(gems) > 0 {
		downloadReport, err := dm.DownloadAll(ctx, gems, *force)
		if err != nil {
			return err
		}
		if !quiet {
			fmt.Printf("Cache ready. %d fetched, %d reused.\n", downloadReport.Downloaded, downloadReport.Skipped)
		}
	}

	// Import the extensions package for config
	extConfig := buildExtensionConfig(*skipExtensions, *verbose, *vendorDir)

	// Install regular gems
	var totalInstalled, totalSkipped, totalExtBuilt, totalExtFailed int
	if len(gems) > 0 {
		if !quiet {
			fmt.Printf("Installing %d rubygems gem(s)...\n", len(gems))
		}
		installReport, err := callbacks.InstallFromCache(ctx, dm.CacheDir(), *vendorDir, gems, *force, *buildExts, extConfig)
		if err != nil {
			return err
		}
		totalInstalled += installReport.Installed
		totalSkipped += installReport.Skipped
		totalExtBuilt += installReport.ExtensionsBuilt
		totalExtFailed += installReport.ExtensionsFailed
	}

	// Get ruby scope for Bundler-compatible paths
	rubyScope := ruby.Scope()

	// Filter and install git gems
	gitSpecs := parsed.GitSpecs
	if len(excludeGroups) > 0 {
		gitSpecs = filterGitGemsByGroups(gitSpecs, excludeGroups)
	}
	if len(gitSpecs) > 0 {
		if !quiet {
			fmt.Printf("Installing %d git gem(s)...\n", len(gitSpecs))
		}
		gitReport, err := callbacks.InstallGitGems(ctx, *vendorDir, rubyScope, gitSpecs, *force, *buildExts, extConfig)
		if err != nil {
			return err
		}
		totalInstalled += gitReport.Installed
		totalSkipped += gitReport.Skipped
		totalExtBuilt += gitReport.ExtensionsBuilt
		totalExtFailed += gitReport.ExtensionsFailed
	}

	// Filter and install path gems
	pathSpecs := parsed.PathSpecs
	if len(excludeGroups) > 0 {
		pathSpecs = filterPathGemsByGroups(pathSpecs, excludeGroups)
	}
	if len(pathSpecs) > 0 {
		if !quiet {
			fmt.Printf("Installing %d path gem(s)...\n", len(pathSpecs))
		}
		lockfileDir := filepath.Dir(effectiveLockfilePath)
		pathReport, err := callbacks.InstallPathGems(ctx, *vendorDir, rubyScope, pathSpecs, *force, *buildExts, extConfig, lockfileDir)
		if err != nil {
			return err
		}
		totalInstalled += pathReport.Installed
		totalSkipped += pathReport.Skipped
		totalExtBuilt += pathReport.ExtensionsBuilt
		totalExtFailed += pathReport.ExtensionsFailed
	}

	elapsed := time.Since(startTime)

	// Simplify vendor dir display for common paths
	vendorDisplay := *vendorDir
	if home, err := os.UserHomeDir(); err == nil {
		vendorDisplay = strings.Replace(vendorDisplay, home, "~", 1)
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, *vendorDir); err == nil && !strings.HasPrefix(rel, "..") {
			vendorDisplay = rel
		}
	}

	fmt.Printf("Installed %d gems (%d skipped) into %s in %s.\n", totalInstalled, totalSkipped, vendorDisplay, elapsed.Round(time.Millisecond))

	if totalExtBuilt > 0 {
		fmt.Printf("Built %d native extension(s).\n", totalExtBuilt)
	}
	if totalExtFailed > 0 {
		fmt.Fprintf(os.Stderr, "Error: %d native extension(s) failed to build.\n", totalExtFailed)
		return fmt.Errorf("%d native extension(s) failed to build", totalExtFailed)
	}

	// Display post-install messages
	if totalInstalled > 0 && !quiet {
		if messages, err := ReadPostInstallMessages(*vendorDir); err == nil {
			DisplayPostInstallMessages(messages)
		}
	}

	// Build simplified exec command suggestion
	execCmd := "ore exec"

	// Only include --lockfile if non-default
	defaultLock := defaultLockfilePath()
	if effectiveLockfilePath != defaultLock {
		// Simplify lockfile path
		lockDisplay := effectiveLockfilePath
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, effectiveLockfilePath); err == nil && !strings.HasPrefix(rel, "..") {
				lockDisplay = rel
			}
		}
		execCmd += fmt.Sprintf(" --lockfile=%s", lockDisplay)
	}

	// Only include --vendor if non-default
	defaultVendor := callbacks.GetDefaultVendorDir()
	if *vendorDir != defaultVendor {
		execCmd += fmt.Sprintf(" --vendor=%s", vendorDisplay)
	}

	execCmd += " <command>"

	if !quiet {
		fmt.Printf("Use `%s` to run commands with this environment.\n", execCmd)
	}
	return nil
}

// parseGroupList parses a comma-separated list of groups
func parseGroupList(groupsStr string) []string {
	if groupsStr == "" {
		return nil
	}

	groups := strings.Split(groupsStr, ",")
	result := make([]string, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g != "" {
			result = append(result, g)
		}
	}
	return result
}

// filterGemsByGroupsAndDependencies filters gems by groups and includes transitive dependencies
func filterGemsByGroupsAndDependencies(gems []lockfile.GemSpec, allGems []lockfile.GemSpec, excludeGroups []string) []lockfile.GemSpec {
	// Create a map of all gems for lookup
	gemMap := make(map[string]lockfile.GemSpec)
	for _, gem := range allGems {
		gemMap[gem.Name] = gem
	}

	// Identify gems that should be kept (have groups and are not excluded)
	// Gems with empty groups are transitive deps, we'll handle them later
	rootGems := make(map[string]bool)
	for _, gem := range gems {
		if len(gem.Groups) > 0 {
			// This is a direct dependency from Gemfile
			excluded := false
			for _, gemGroup := range gem.Groups {
				for _, excludeGroup := range excludeGroups {
					if gemGroup == excludeGroup {
						excluded = true
						break
					}
				}
				if excluded {
					break
				}
			}
			if !excluded {
				rootGems[gem.Name] = true
			}
		}
	}

	// Perform depth-first traversal to find all needed dependencies
	neededGems := make(map[string]bool)
	var collectDependencies func(gemName string)
	collectDependencies = func(gemName string) {
		if neededGems[gemName] {
			return // Already processed
		}
		neededGems[gemName] = true

		if gem, found := gemMap[gemName]; found {
			for _, dep := range gem.Dependencies {
				collectDependencies(dep.Name)
			}
		}
	}

	// Collect all dependencies of root gems
	for gemName := range rootGems {
		collectDependencies(gemName)
	}

	// Build final result with only needed gems
	var result []lockfile.GemSpec
	for _, gem := range allGems {
		if neededGems[gem.Name] {
			result = append(result, gem)
		}
	}

	return result
}

// filterGemsByPlatform filters gems to only include compatible platforms
func filterGemsByPlatform(gems []lockfile.GemSpec) []lockfile.GemSpec {
	currentPlatform := detectCurrentPlatform()

	var filtered []lockfile.GemSpec
	for _, gem := range gems {
		// Keep pure Ruby gems (no platform constraint)
		if gem.Platform == "" {
			filtered = append(filtered, gem)
			continue
		}

		// Keep gems matching current platform
		if platformMatches(gem.Platform, currentPlatform) {
			filtered = append(filtered, gem)
		}
	}
	return filtered
}

// filterGitGemsByGroups filters git gems by excluding specified groups
func filterGitGemsByGroups(gitSpecs []lockfile.GitGemSpec, excludeGroups []string) []lockfile.GitGemSpec {
	var result []lockfile.GitGemSpec
	for _, gem := range gitSpecs {
		if len(gem.Groups) == 0 {
			// No group info means it's not in the Gemfile, skip it
			continue
		}

		excluded := false
		for _, gemGroup := range gem.Groups {
			for _, excludeGroup := range excludeGroups {
				if gemGroup == excludeGroup {
					excluded = true
					break
				}
			}
			if excluded {
				break
			}
		}

		if !excluded {
			result = append(result, gem)
		}
	}
	return result
}

// filterPathGemsByGroups filters path gems by excluding specified groups
func filterPathGemsByGroups(pathSpecs []lockfile.PathGemSpec, excludeGroups []string) []lockfile.PathGemSpec {
	var result []lockfile.PathGemSpec
	for _, gem := range pathSpecs {
		if len(gem.Groups) == 0 {
			// No group info means it's not in the Gemfile, skip it
			continue
		}

		excluded := false
		for _, gemGroup := range gem.Groups {
			for _, excludeGroup := range excludeGroups {
				if gemGroup == excludeGroup {
					excluded = true
					break
				}
			}
			if excluded {
				break
			}
		}

		if !excluded {
			result = append(result, gem)
		}
	}
	return result
}

// buildExtensionConfig creates extension build configuration
func buildExtensionConfig(skipExtensions, verbose bool, vendorDir string) *extensions.BuildConfig {
	// Check environment variable override
	if extensions.ShouldSkipExtensions() {
		skipExtensions = true
	}

	config := &extensions.BuildConfig{
		SkipExtensions: skipExtensions,
		Verbose:        verbose,
		Parallel:       runtime.NumCPU(),
		VendorDir:      vendorDir,
	}

	// Check if Ruby is available
	if !skipExtensions && !extensions.IsRubyAvailable() {
		fmt.Fprintf(os.Stderr, "Warning: Ruby not found in PATH. Native extensions will be skipped.\n")
		fmt.Fprintf(os.Stderr, "Install Ruby or use --skip-extensions to suppress this warning.\n")
		config.SkipExtensions = true
	}

	return config
}
