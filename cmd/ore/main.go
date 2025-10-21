package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/contriboss/gemfile-go/gemfile"
	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/cmd/ore/commands"
	"github.com/contriboss/ore-light/internal/extensions"
	"github.com/contriboss/ore-light/internal/resolver"
)

var (
	version     = "0.0.0-dev"
	buildCommit = "unknown"
	buildTime   = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "--help", "-h", "help":
		printHelp()
	case "--version", "-v", "version":
		printVersion()
	case "add":
		if err := commands.RunAdd(args); err != nil {
			exitWithError(err)
		}
	case "remove":
		if err := commands.RunRemove(args); err != nil {
			exitWithError(err)
		}
	case "update":
		if err := commands.RunUpdate(args); err != nil {
			exitWithError(err)
		}
	case "outdated":
		if err := commands.RunOutdated(args); err != nil {
			exitWithError(err)
		}
	case "info":
		if err := commands.RunInfo(args); err != nil {
			exitWithError(err)
		}
	case "list":
		if err := commands.RunList(args); err != nil {
			exitWithError(err)
		}
	case "check":
		if err := commands.RunCheck(args); err != nil {
			exitWithError(err)
		}
	case "init":
		if err := commands.RunInit(args); err != nil {
			exitWithError(err)
		}
	case "platform":
		if err := commands.RunPlatform(args); err != nil {
			exitWithError(err)
		}
	case "show":
		if err := commands.RunShow(args); err != nil {
			exitWithError(err)
		}
	case "clean":
		if err := commands.RunClean(args); err != nil {
			exitWithError(err)
		}
	case "lock":
		if err := runLockCommand(args); err != nil {
			exitWithError(err)
		}
	case "download":
		if err := runDownloadCommand(args); err != nil {
			exitWithError(err)
		}
	case "install":
		if err := runInstallCommand(args); err != nil {
			exitWithError(err)
		}
	case "cache":
		if err := runCacheCommand(args); err != nil {
			exitWithError(err)
		}
	case "exec":
		if err := runExecCommand(args); err != nil {
			exitWithError(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func runLockCommand(args []string) error {
	fs := flag.NewFlagSet("lock", flag.ContinueOnError)
	gemfilePath := fs.String("gemfile", defaultGemfilePath(), "Path to Gemfile")
	verbose := fs.Bool("v", false, "Enable verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := os.Stat(*gemfilePath); err != nil {
		return fmt.Errorf("Gemfile not found at %s", *gemfilePath)
	}

	if *verbose {
		fmt.Printf("🔒 Resolving dependencies from %s…\n", *gemfilePath)
	}

	if err := resolver.GenerateLockfile(*gemfilePath); err != nil {
		return fmt.Errorf("failed to generate lockfile: %w", err)
	}

	lockfilePath := *gemfilePath + ".lock"
	if *verbose {
		fmt.Printf("✅ Updated %s\n", lockfilePath)
	} else {
		fmt.Printf("✨ Wrote %s\n", lockfilePath)
	}

	fmt.Println("💡 Run `ore install` to fetch the resolved gems.")
	return nil
}

func printHelp() {
	fmt.Print(`Usage: ore <command> [options]

Commands:
  init         Generate a new Gemfile
  add          Add gems to Gemfile
  remove       Remove gems from Gemfile
  update       Update gems to their latest versions within constraints
  outdated     List gems with newer versions available
  info         Show detailed information about a gem
  list         List all gems in the current bundle
  check        Verify all gems are installed
  show         Show the source location of a gem in the bundle
  platform     Display platform compatibility information
  clean        Remove unused gems from vendor directory
  lock         Regenerate Gemfile.lock from Gemfile
  download     Prefetch gems defined in Gemfile.lock (no Ruby required)
  install      Download (if needed) and unpack gems into a vendor directory
  cache        Inspect or prune the ore gem cache
  exec         Run commands via bundle exec with ore-managed environment
  version      Show version information

Use "ore <command> --help" for command-specific options.
`)
}

func printVersion() {
	fmt.Println(versionInfo())
}

func versionInfo() string {
	return fmt.Sprintf("ore version %s (commit %s, built %s)", version, buildCommit, buildTime)
}
func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func runDownloadCommand(args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	lockfilePath := fs.String("lockfile", defaultLockfilePath(), "Path to Gemfile.lock")
	force := fs.Bool("force", false, "Re-download gems even if present in cache")
	workers := fs.Int("workers", runtime.NumCPU(), "Number of concurrent downloads")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dm, err := newDefaultDownloadManager(*workers)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gems, err := loadGemSpecs(*lockfilePath)
	if err != nil {
		return err
	}

	if len(gems) == 0 {
		fmt.Println("No gems found in lockfile.")
		return nil
	}

	report, err := dm.DownloadAll(ctx, gems, *force)
	if err != nil {
		return err
	}

	fmt.Printf("Download complete. %d fetched, %d skipped (cached).\n", report.Downloaded, report.Skipped)
	return nil
}

func runInstallCommand(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	lockfilePath := fs.String("lockfile", defaultLockfilePath(), "Path to Gemfile.lock")
	workers := fs.Int("workers", runtime.NumCPU(), "Number of concurrent downloads")
	force := fs.Bool("force", false, "Re-download or reinstall even if artifacts exist")
	vendorDir := fs.String("vendor", defaultVendorDir(), "Destination directory for installed gems")
	skipExtensions := fs.Bool("skip-extensions", false, "Skip building native extensions")
	verbose := fs.Bool("verbose", false, "Enable verbose output including extension build logs")
	without := fs.String("without", "", "Comma-separated list of groups to exclude (e.g., development,test)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dm, err := newDefaultDownloadManager(*workers)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load both regular gems and git gems from lockfile
	parsed, err := loadLockfile(*lockfilePath)
	if err != nil {
		return err
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
		gemfilePath := detectGemfileFromLock(*lockfilePath)
		if gemfilePath == "" {
			gemfilePath = "Gemfile"
		}

		if err := enrichGemsWithGroups(gemfilePath, parsed); err != nil {
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

	// Download regular gems from rubygems.org
	if len(gems) > 0 {
		downloadReport, err := dm.DownloadAll(ctx, gems, *force)
		if err != nil {
			return err
		}
		fmt.Printf("Cache ready. %d fetched, %d reused.\n", downloadReport.Downloaded, downloadReport.Skipped)
	}

	// Import the extensions package for config
	extConfig := buildExtensionConfig(*skipExtensions, *verbose)

	// Install regular gems
	var totalInstalled, totalSkipped, totalExtBuilt, totalExtFailed int
	if len(gems) > 0 {
		installReport, err := installFromCache(ctx, dm.CacheDir(), *vendorDir, gems, *force, extConfig)
		if err != nil {
			return err
		}
		totalInstalled += installReport.Installed
		totalSkipped += installReport.Skipped
		totalExtBuilt += installReport.ExtensionsBuilt
		totalExtFailed += installReport.ExtensionsFailed
	}

	// Filter and install git gems
	gitSpecs := parsed.GitSpecs
	if len(excludeGroups) > 0 {
		gitSpecs = filterGitGemsByGroups(gitSpecs, excludeGroups)
	}
	if len(gitSpecs) > 0 {
		fmt.Printf("Installing %d git gem(s)...\n", len(gitSpecs))
		gitReport, err := installGitGems(ctx, *vendorDir, gitSpecs, *force, extConfig)
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
		fmt.Printf("Installing %d path gem(s)...\n", len(pathSpecs))
		pathReport, err := installPathGems(ctx, *vendorDir, pathSpecs, *force, extConfig)
		if err != nil {
			return err
		}
		totalInstalled += pathReport.Installed
		totalSkipped += pathReport.Skipped
		totalExtBuilt += pathReport.ExtensionsBuilt
		totalExtFailed += pathReport.ExtensionsFailed
	}

	fmt.Printf("Installed %d gems (%d skipped) into %s.\n", totalInstalled, totalSkipped, *vendorDir)

	if totalExtBuilt > 0 {
		fmt.Printf("Built %d native extension(s).\n", totalExtBuilt)
	}
	if totalExtFailed > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %d extension(s) failed to build.\n", totalExtFailed)
	}

	fmt.Printf("Use `ore exec --lockfile=%s --vendor=%s <command>` to run Ruby code with this environment.\n", *lockfilePath, *vendorDir)
	return nil
}

func runCacheCommand(args []string) error {
	if len(args) == 0 {
		printCacheHelp()
		return nil
	}

	switch args[0] {
	case "info":
		return runCacheInfo(args[1:])
	case "prune":
		return runCachePrune(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown cache subcommand %q\n\n", args[0])
		printCacheHelp()
		return nil
	}
}

func printCacheHelp() {
	fmt.Print(`Usage: ore cache <subcommand>

Subcommands:
  info         Show cache location, size, and gem count
  prune        Remove all cached gems
`)
}

func runCacheInfo(args []string) error {
	fs := flag.NewFlagSet("cache info", flag.ContinueOnError)
	workers := fs.Int("workers", runtime.NumCPU(), "Number of concurrent operations (unused but reserved)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = workers // Reserved for future use

	cacheDir, err := defaultCacheDir()
	if err != nil {
		return err
	}

	stats, err := collectCacheStats(cacheDir)
	if err != nil {
		return err
	}

	fmt.Printf("Cache directory: %s\n", cacheDir)
	fmt.Printf("Cached gems:    %d\n", stats.Files)
	fmt.Printf("Total size:     %s\n", humanBytes(stats.TotalSize))
	return nil
}

func runCachePrune(args []string) error {
	fs := flag.NewFlagSet("cache prune", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "Show what would be removed without deleting files")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cacheDir, err := defaultCacheDir()
	if err != nil {
		return err
	}

	if *dryRun {
		stats, err := collectCacheStats(cacheDir)
		if err != nil {
			return err
		}
		fmt.Printf("[dry-run] Would remove %d files (%s) from %s\n", stats.Files, humanBytes(stats.TotalSize), cacheDir)
		return nil
	}

	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to prune cache: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("failed to recreate cache dir: %w", err)
	}

	fmt.Printf("Cache cleared: %s\n", cacheDir)
	return nil
}

func runExecCommand(args []string) error {
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	lockfilePath := fs.String("lockfile", defaultLockfilePath(), "Path to Gemfile.lock")
	vendorDir := fs.String("vendor", defaultVendorDir(), "Path to installed gems (created by ore install)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command provided; usage: ore exec [options] -- <command> [args...]")
	}

	gems, err := loadGemSpecs(*lockfilePath)
	if err != nil {
		return err
	}

	env, err := buildExecutionEnv(*vendorDir, gems)
	if err != nil {
		return err
	}

	if err := ensureBundlerAvailable(); err != nil {
		return err
	}

	env = setEnv(env, "BUNDLE_PATH", *vendorDir)
	if gemfile := detectGemfileFromLock(*lockfilePath); gemfile != "" {
		env = setEnv(env, "BUNDLE_GEMFILE", gemfile)
	}

	cmd := exec.Command("bundle", append([]string{"exec"}, cmdArgs...)...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func defaultLockfilePath() string {
	// Try to auto-detect Gemfile.lock or gems.locked
	// This respects BUNDLE_GEMFILE if set
	lockPath, err := lockfile.FindLockfileOnly()
	if err == nil {
		return lockPath
	}

	// Fallback to Gemfile.lock for backward compatibility
	return "Gemfile.lock"
}

func defaultVendorDir() string {
	if env := os.Getenv("ORE_VENDOR_DIR"); env != "" {
		return env
	}
	if env := os.Getenv("ORE_LIGHT_VENDOR_DIR"); env != "" {
		return env
	}
	if appConfig != nil && appConfig.VendorDir != "" {
		return appConfig.VendorDir
	}
	return filepath.Join("vendor", "ore")
}

func defaultGemfilePath() string {
	if env := os.Getenv("ORE_GEMFILE"); env != "" {
		return env
	}
	if appConfig != nil && appConfig.Gemfile != "" {
		return appConfig.Gemfile
	}
	return "Gemfile"
}

func loadLockfile(lockfilePath string) (*lockfile.Lockfile, error) {
	file, err := os.Open(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open lockfile: %w", err)
	}
	defer file.Close()

	parsed, err := lockfile.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lockfile: %w", err)
	}

	return parsed, nil
}

func loadGemSpecs(lockfilePath string) ([]lockfile.GemSpec, error) {
	parsed, err := loadLockfile(lockfilePath)
	if err != nil {
		return nil, err
	}

	return deduplicateGemSpecs(parsed.GemSpecs), nil
}

func deduplicateGemSpecs(specs []lockfile.GemSpec) []lockfile.GemSpec {
	if len(specs) == 0 {
		return nil
	}

	unique := make(map[string]lockfile.GemSpec, len(specs))
	for _, spec := range specs {
		unique[spec.FullName()] = spec
	}

	result := make([]lockfile.GemSpec, 0, len(unique))
	for _, spec := range unique {
		result = append(result, spec)
	}

	return result
}

func humanBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

func defaultCacheDir() (string, error) {
	if cache := os.Getenv("ORE_CACHE_DIR"); cache != "" {
		return cache, nil
	}
	if cache := os.Getenv("ORE_LIGHT_CACHE_DIR"); cache != "" {
		return cache, nil
	}
	if appConfig != nil && appConfig.CacheDir != "" {
		return appConfig.CacheDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}

	return filepath.Join(home, ".cache", "ore", "gems"), nil
}

type cacheStats struct {
	Files     int
	TotalSize int64
}

func collectCacheStats(cacheDir string) (cacheStats, error) {
	var stats cacheStats

	err := filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		stats.Files++
		stats.TotalSize += info.Size()
		return nil
	})

	if os.IsNotExist(err) {
		return stats, nil
	}

	return stats, err
}

func newDefaultDownloadManager(workers int) (*downloadManager, error) {
	cacheDir, err := defaultCacheDir()
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(defaultDownloadBaseURL(), "/")
	client := defaultHTTPClient()

	return newDownloadManager(cacheDir, baseURL, client, workers)
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

func defaultDownloadBaseURL() string {
	if mirror := os.Getenv("ORE_GEM_MIRROR"); mirror != "" {
		return mirror
	}
	if mirror := os.Getenv("ORE_LIGHT_GEM_MIRROR"); mirror != "" {
		return mirror
	}
	if appConfig != nil && appConfig.GemMirror != "" {
		return appConfig.GemMirror
	}
	return "https://rubygems.org/downloads"
}

func ensureBundlerAvailable() error {
	if _, err := exec.LookPath("bundle"); err != nil {
		return fmt.Errorf("bundler executable not found in PATH (install Ruby + Bundler to continue)")
	}
	return nil
}

func detectGemfileFromLock(lockfilePath string) string {
	if lockfilePath == "" {
		lockfilePath = "Gemfile.lock"
	}
	if !strings.HasSuffix(lockfilePath, ".lock") {
		return ""
	}
	candidate := strings.TrimSuffix(lockfilePath, ".lock")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

func buildExtensionConfig(skipExtensions, verbose bool) *extensions.BuildConfig {
	// Check environment variable override
	if extensions.ShouldSkipExtensions() {
		skipExtensions = true
	}

	config := &extensions.BuildConfig{
		SkipExtensions: skipExtensions,
		Verbose:        verbose,
		Parallel:       runtime.NumCPU(),
	}

	// Check if Ruby is available
	if !skipExtensions && !extensions.IsRubyAvailable() {
		fmt.Fprintf(os.Stderr, "Warning: Ruby not found in PATH. Native extensions will be skipped.\n")
		fmt.Fprintf(os.Stderr, "Install Ruby or use --skip-extensions to suppress this warning.\n")
		config.SkipExtensions = true
	}

	return config
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

// enrichGemsWithGroups reads the Gemfile and enriches lockfile gems with group information
func enrichGemsWithGroups(gemfilePath string, parsed *lockfile.Lockfile) error {
	parser := gemfile.NewGemfileParser(gemfilePath)
	parsedGemfile, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse Gemfile: %w", err)
	}

	// Create a map of gem name -> groups from the Gemfile
	gemGroups := make(map[string][]string)
	for _, dep := range parsedGemfile.Dependencies {
		if len(dep.Groups) > 0 {
			gemGroups[dep.Name] = dep.Groups
		} else {
			gemGroups[dep.Name] = []string{"default"}
		}
	}

	// Enrich GemSpecs with group information
	for i := range parsed.GemSpecs {
		if groups, found := gemGroups[parsed.GemSpecs[i].Name]; found {
			parsed.GemSpecs[i].Groups = groups
		}
	}

	// Enrich GitGemSpecs with group information
	for i := range parsed.GitSpecs {
		if groups, found := gemGroups[parsed.GitSpecs[i].Name]; found {
			parsed.GitSpecs[i].Groups = groups
		}
	}

	// Enrich PathGemSpecs with group information
	for i := range parsed.PathSpecs {
		if groups, found := gemGroups[parsed.PathSpecs[i].Name]; found {
			parsed.PathSpecs[i].Groups = groups
		}
	}

	return nil
}
