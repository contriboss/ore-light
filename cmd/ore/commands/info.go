package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/extensions"
	"github.com/contriboss/ore-light/internal/geminstall"
	"github.com/contriboss/ore-light/internal/registry"
	"github.com/contriboss/ore-light/internal/resolver"
	"github.com/contriboss/ore-light/internal/ruby"
)

// RunInfo implements the ore info command
func RunInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	verbose := fs.Bool("v", false, "Enable verbose output")
	debug := fs.Bool("debug", false, "Show detailed diagnostic information about gem handling")
	version := fs.String("version", "", "Specific version to inspect (for debug mode)")
	platform := fs.String("platform", "", "Platform to check (e.g., x86_64-linux-gnu, ruby)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	gems := fs.Args()
	if len(gems) == 0 {
		return fmt.Errorf("at least one gem name is required")
	}

	client, err := registry.NewClient("https://rubygems.org", registry.ProtocolRubygems)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	ctx := context.Background()

	// Debug mode: comprehensive diagnostics
	if *debug {
		return showDebugInfo(ctx, client, gems, *version, *platform)
	}

	// Standard info mode
	for _, gemName := range gems {
		if *verbose {
			fmt.Printf("🔍 Fetching info for %s...\n", gemName)
		}

		// Get versions first
		versions, err := client.GetGemVersions(ctx, gemName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not fetch versions for %s: %v\n", gemName, err)
			continue
		}

		if len(versions) == 0 {
			fmt.Printf("No versions found for gem: %s\n", gemName)
			continue
		}

		// Get info for latest version
		latestVersion := versions[0]
		info, err := client.GetGemInfo(ctx, gemName, latestVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not fetch info for %s: %v\n", gemName, err)
			continue
		}

		// Print gem information
		fmt.Printf("\n*** %s ***\n\n", gemName)
		fmt.Printf("  Latest version: %s\n", latestVersion)

		// Show available versions (limit to 20)
		fmt.Printf("  Versions: %s", versions[0])
		limit := 20
		if len(versions) > limit {
			for i := 1; i < limit; i++ {
				fmt.Printf(", %s", versions[i])
			}
			fmt.Printf(" (+ %d more)\n", len(versions)-limit)
		} else {
			for i := 1; i < len(versions); i++ {
				fmt.Printf(", %s", versions[i])
			}
			fmt.Println()
		}

		// Show dependencies
		runtimeDeps := info.Dependencies.Runtime
		devDeps := info.Dependencies.Development

		if len(runtimeDeps) > 0 {
			fmt.Printf("  Runtime dependencies:\n")
			for _, dep := range runtimeDeps {
				fmt.Printf("    - %s %s\n", dep.Name, dep.Requirements)
			}
		} else {
			fmt.Printf("  Runtime dependencies: (none)\n")
		}

		if len(devDeps) > 0 && *verbose {
			fmt.Printf("  Development dependencies:\n")
			for _, dep := range devDeps {
				fmt.Printf("    - %s %s\n", dep.Name, dep.Requirements)
			}
		}

		fmt.Println()
	}

	return nil
}

// debugContext holds all the context needed for debug diagnostics
type debugContext struct {
	engine            ruby.Engine
	engineChecker     *resolver.EngineCompatibility
	cacheDir          string
	vendorDir         string
	currentPlatform   string
	requestedPlatform string
}

// gemDebugInfo holds all information about a specific gem for diagnostics
type gemDebugInfo struct {
	name               string
	targetVersion      string
	versions           []string
	info               *registry.GemInfo
	rubyGemPath        string
	rubyGemExists      bool
	platformGemPath    string
	platformGemExists  bool
	rubyInstallDir     string
	rubyInstalled      bool
	platformInstallDir string
	platformInstalled  bool
	gemExtensions      []string
	hasExtensions      bool
	isPlatformSpecific bool
}

// showDebugInfo displays comprehensive diagnostic information about how a gem will be handled
func showDebugInfo(ctx context.Context, client *registry.Client, gems []string, requestedVersion, requestedPlatform string) error {
	// Initialize debug context
	debugCtx := initializeDebugContext(requestedPlatform)

	// Display header and system information
	displayDebugHeader(debugCtx)

	// Process each gem
	for _, gemName := range gems {
		if err := debugGem(ctx, client, gemName, requestedVersion, debugCtx); err != nil {
			fmt.Printf("❌ Error processing gem %s: %v\n\n", gemName, err)
			continue
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	return nil
}

// initializeDebugContext sets up the debug environment context
func initializeDebugContext(requestedPlatform string) debugContext {
	engine := ruby.DetectEngine()
	cfg := config.Load()
	cacheDir, _ := config.DefaultCacheDir(cfg)
	vendorDir := config.DefaultVendorDir(cfg, detectRubyVersionForInfo, getSystemGemDirForInfo)
	currentPlatform := detectCurrentPlatform()

	if requestedPlatform == "" {
		requestedPlatform = currentPlatform
	}

	return debugContext{
		engine:            engine,
		engineChecker:     resolver.NewEngineCompatibility(engine),
		cacheDir:          cacheDir,
		vendorDir:         vendorDir,
		currentPlatform:   currentPlatform,
		requestedPlatform: requestedPlatform,
	}
}

// displayDebugHeader shows the initial system information
func displayDebugHeader(ctx debugContext) {
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  ORE DEBUG MODE - Gem Installation Diagnostics")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	fmt.Printf("📍 System Information:\n")
	fmt.Printf("   Ruby Engine:        %s %s\n", ctx.engine.Name, ctx.engine.Version)
	fmt.Printf("   Native Extensions:  %v\n", ctx.engine.SupportsNativeExtensions())
	fmt.Printf("   Current Platform:   %s\n", ctx.currentPlatform)
	fmt.Printf("   Requested Platform: %s\n", ctx.requestedPlatform)
	fmt.Printf("   Cache Directory:    %s\n", ctx.cacheDir)
	fmt.Printf("   Vendor Directory:   %s\n", ctx.vendorDir)
	fmt.Println()
}

// debugGem processes and displays diagnostics for a single gem
func debugGem(ctx context.Context, client *registry.Client, gemName, requestedVersion string, debugCtx debugContext) error {
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("  GEM: %s\n", gemName)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Println()

	// Gather all gem information
	gemInfo, err := gatherGemInfo(ctx, client, gemName, requestedVersion, debugCtx)
	if err != nil {
		return err
	}

	// Display each section
	displayVersionInfo(gemInfo)
	displayCacheStatus(gemInfo, debugCtx)
	displayInstallationStatus(gemInfo, debugCtx)
	displayExtensionInfo(gemInfo)
	displayPlatformBehavior(gemInfo, debugCtx)
	displayExtensionBuildAnalysis(gemInfo, debugCtx)
	displayEngineCompatibility(gemInfo, debugCtx)
	displayDependencies(gemInfo)
	displayRecommendations(gemInfo, debugCtx)

	return nil
}

// gatherGemInfo collects all information about a gem for diagnostics
func gatherGemInfo(ctx context.Context, client *registry.Client, gemName, requestedVersion string, debugCtx debugContext) (*gemDebugInfo, error) {
	// Get available versions
	versions, err := client.GetGemVersions(ctx, gemName)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found")
	}

	// Determine version to inspect
	targetVersion := requestedVersion
	if targetVersion == "" {
		targetVersion = versions[0] // Latest
	}

	// Get gem info
	info, err := client.GetGemInfo(ctx, gemName, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("fetching gem info: %w", err)
	}

	// Build gem file paths and check existence
	rubyGemFile := fmt.Sprintf("%s-%s.gem", gemName, targetVersion)
	rubyGemPath := filepath.Join(debugCtx.cacheDir, rubyGemFile)
	rubyGemExists := fileExists(rubyGemPath)

	platformGemFile := fmt.Sprintf("%s-%s-%s.gem", gemName, targetVersion, debugCtx.requestedPlatform)
	platformGemPath := filepath.Join(debugCtx.cacheDir, platformGemFile)
	platformGemExists := fileExists(platformGemPath)

	// Build installation paths and check existence
	rubyGemFullName := fmt.Sprintf("%s-%s", gemName, targetVersion)
	rubyInstallDir := filepath.Join(debugCtx.vendorDir, "gems", rubyGemFullName)
	rubyInstalled := dirExists(rubyInstallDir)

	platformGemFullName := rubyGemFullName
	if debugCtx.requestedPlatform != "" && debugCtx.requestedPlatform != "ruby" {
		platformGemFullName = fmt.Sprintf("%s-%s-%s", gemName, targetVersion, debugCtx.requestedPlatform)
	}
	platformInstallDir := filepath.Join(debugCtx.vendorDir, "gems", platformGemFullName)
	platformInstalled := dirExists(platformInstallDir)

	// Extract extension information from cached gem
	gemExtensions, hasExtensions := extractExtensionInfo(rubyGemPath, rubyGemExists, platformGemPath, platformGemExists)

	// Determine if this is a platform-specific gem
	isPlatformSpecific := platformGemExists && debugCtx.requestedPlatform != "" && debugCtx.requestedPlatform != "ruby"

	return &gemDebugInfo{
		name:               gemName,
		targetVersion:      targetVersion,
		versions:           versions,
		info:               info,
		rubyGemPath:        rubyGemPath,
		rubyGemExists:      rubyGemExists,
		platformGemPath:    platformGemPath,
		platformGemExists:  platformGemExists,
		rubyInstallDir:     rubyInstallDir,
		rubyInstalled:      rubyInstalled,
		platformInstallDir: platformInstallDir,
		platformInstalled:  platformInstalled,
		gemExtensions:      gemExtensions,
		hasExtensions:      hasExtensions,
		isPlatformSpecific: isPlatformSpecific,
	}, nil
}

// extractExtensionInfo extracts extension information from a cached gem
func extractExtensionInfo(rubyGemPath string, rubyGemExists bool, platformGemPath string, platformGemExists bool) ([]string, bool) {
	var gemExtensions []string
	var hasExtensions bool

	// Try ruby gem first
	if rubyGemExists {
		if metadata, err := geminstall.ExtractMetadataOnly(rubyGemPath); err == nil && len(metadata) > 0 {
			if exts, err := geminstall.ParseExtensionsFromMetadata(metadata); err == nil {
				gemExtensions = exts
				hasExtensions = len(exts) > 0
				return gemExtensions, hasExtensions
			}
		}
	}

	// Try platform gem
	if platformGemExists {
		if metadata, err := geminstall.ExtractMetadataOnly(platformGemPath); err == nil && len(metadata) > 0 {
			if exts, err := geminstall.ParseExtensionsFromMetadata(metadata); err == nil {
				gemExtensions = exts
				hasExtensions = len(exts) > 0
				return gemExtensions, hasExtensions
			}
		}
	}

	return gemExtensions, hasExtensions
}

// displayVersionInfo shows version information
func displayVersionInfo(gem *gemDebugInfo) {
	fmt.Printf("📦 Version Information:\n")
	fmt.Printf("   Available versions: %d total\n", len(gem.versions))
	fmt.Printf("   Latest version:     %s\n", gem.versions[0])
	fmt.Printf("   Inspecting version: %s\n", gem.targetVersion)
	fmt.Println()
}

// displayCacheStatus shows what's cached
func displayCacheStatus(gem *gemDebugInfo, ctx debugContext) {
	fmt.Printf("💾 Cache Status:\n")

	rubyGemFile := filepath.Base(gem.rubyGemPath)
	if gem.rubyGemExists {
		fmt.Printf("   ✅ Ruby gem cached:     %s\n", rubyGemFile)
	} else {
		fmt.Printf("   ❌ Ruby gem NOT cached: %s\n", rubyGemFile)
	}

	if ctx.requestedPlatform != "" && ctx.requestedPlatform != "ruby" {
		platformGemFile := filepath.Base(gem.platformGemPath)
		if gem.platformGemExists {
			fmt.Printf("   ✅ Platform gem cached: %s\n", platformGemFile)
		} else {
			fmt.Printf("   ❌ Platform gem NOT cached: %s\n", platformGemFile)
		}
	}
	fmt.Println()
}

// displayInstallationStatus shows what's installed
func displayInstallationStatus(gem *gemDebugInfo, ctx debugContext) {
	fmt.Printf("📂 Installation Status:\n")

	if gem.rubyInstalled {
		fmt.Printf("   ✅ Ruby version installed:     %s\n", gem.rubyInstallDir)
	} else {
		fmt.Printf("   ❌ Ruby version NOT installed: %s\n", gem.rubyInstallDir)
	}

	if ctx.requestedPlatform != "" && ctx.requestedPlatform != "ruby" {
		if gem.platformInstalled {
			fmt.Printf("   ✅ Platform version installed: %s\n", gem.platformInstallDir)
		} else {
			fmt.Printf("   ❌ Platform version NOT installed: %s\n", gem.platformInstallDir)
		}
	}
	fmt.Println()
}

// displayExtensionInfo shows extension details
func displayExtensionInfo(gem *gemDebugInfo) {
	fmt.Printf("🔧 Extension Information:\n")

	if gem.rubyGemExists || gem.platformGemExists {
		fmt.Printf("   Has extensions:     %v\n", gem.hasExtensions)

		if gem.hasExtensions {
			fmt.Printf("   Extensions declared: %d\n", len(gem.gemExtensions))
			for _, ext := range gem.gemExtensions {
				fmt.Printf("     - %s\n", ext)
			}
		} else {
			fmt.Printf("   Extensions:         (none - pure Ruby or precompiled)\n")
		}
	} else {
		fmt.Printf("   ⚠️  Gem not cached - download to see extension details\n")
		fmt.Printf("   Run: ore fetch %s --version %s\n", gem.name, gem.targetVersion)
	}
	fmt.Println()
}

// displayPlatformBehavior shows platform-specific behavior analysis
func displayPlatformBehavior(gem *gemDebugInfo, ctx debugContext) {
	fmt.Printf("🎯 Platform-Specific Behavior:\n")

	if ctx.requestedPlatform != "" && ctx.requestedPlatform != "ruby" {
		// User requested a platform-specific gem
		if gem.platformGemExists {
			fmt.Printf("   Requested platform: %s\n", ctx.requestedPlatform)
			fmt.Printf("   Platform gem:       ✅ Available\n")
			fmt.Printf("   Precompiled:        Assumed YES (platform-specific gems are precompiled)\n")
			fmt.Printf("   Build from source:  NO (will use precompiled binaries)\n")
		} else {
			// Platform gem not available
			fmt.Printf("   Requested platform: %s\n", ctx.requestedPlatform)
			fmt.Printf("   Platform gem:       ❌ NOT Available\n")
			if gem.rubyGemExists {
				fmt.Printf("   Fallback:           Ruby platform gem available\n")
				if gem.hasExtensions {
					fmt.Printf("   Build from source:  YES (ruby gem has extensions)\n")
				} else {
					fmt.Printf("   Build from source:  NO (pure Ruby gem)\n")
				}
			} else {
				fmt.Printf("   Fallback:           No gems cached\n")
			}
		}
	} else {
		// Ruby platform
		fmt.Printf("   Platform:           ruby (default)\n")
		if gem.rubyGemExists || gem.platformGemExists {
			if gem.hasExtensions {
				fmt.Printf("   Precompiled:        NO\n")
				fmt.Printf("   Build from source:  YES (has extensions)\n")
			} else {
				fmt.Printf("   Precompiled:        N/A (pure Ruby)\n")
				fmt.Printf("   Build from source:  NO (no extensions)\n")
			}
		} else {
			fmt.Printf("   Note:               Download gem to determine build requirements\n")
		}
	}
	fmt.Println()
}

// displayExtensionBuildAnalysis shows detailed build analysis
func displayExtensionBuildAnalysis(gem *gemDebugInfo, ctx debugContext) {
	fmt.Printf("🏗️  Extension Build Analysis:\n")

	// Use the platform of the gem that's actually available
	actualPlatform := ctx.requestedPlatform
	if !gem.isPlatformSpecific && gem.rubyGemExists {
		actualPlatform = "ruby"
	}

	// Create mock GemSpec for analysis
	mockSpec := lockfile.GemSpec{
		Name:       gem.name,
		Version:    gem.targetVersion,
		Platform:   actualPlatform,
		Extensions: gem.gemExtensions,
	}

	// Check if extensions would be built
	checkDir := gem.platformInstallDir
	if !gem.isPlatformSpecific {
		checkDir = gem.rubyInstallDir
	}

	if (gem.isPlatformSpecific && gem.platformInstalled) || (!gem.isPlatformSpecific && gem.rubyInstalled) {
		// Already installed - check actual status
		needsBuild, err := extensions.NeedsBuild(checkDir, mockSpec.Extensions, ctx.engine)
		if err != nil {
			fmt.Printf("   ⚠️  Error checking build status: %v\n", err)
		} else {
			fmt.Printf("   Needs building:     %v\n", needsBuild)
			if needsBuild {
				fmt.Printf("   Reason:             Extensions declared but not built\n")
			} else if gem.isPlatformSpecific {
				fmt.Printf("   Reason:             Platform-specific gem is precompiled\n")
			} else if !gem.hasExtensions {
				fmt.Printf("   Reason:             No extensions declared\n")
			} else {
				fmt.Printf("   Reason:             Already built (gem.build_complete marker exists)\n")
			}
		}
	} else {
		// Predict what would happen on install
		predictBuildBehavior(gem, ctx)
	}
	fmt.Println()
}

// predictBuildBehavior predicts what would happen during installation
func predictBuildBehavior(gem *gemDebugInfo, ctx debugContext) {
	if gem.isPlatformSpecific {
		// Platform gem exists and would be used
		fmt.Printf("   Would build:        NO\n")
		fmt.Printf("   Reason:             Platform-specific gem is precompiled\n")
	} else if ctx.requestedPlatform != "" && ctx.requestedPlatform != "ruby" && !gem.platformGemExists {
		// User requested platform but it doesn't exist - would fall back to ruby gem
		fmt.Printf("   Would build:        ")
		if !gem.hasExtensions {
			fmt.Printf("NO\n")
			fmt.Printf("   Reason:             Ruby gem has no extensions (platform gem unavailable)\n")
		} else if !ctx.engine.SupportsNativeExtensions() {
			fmt.Printf("NO\n")
			fmt.Printf("   Reason:             Engine doesn't support native extensions\n")
		} else {
			fmt.Printf("YES\n")
			fmt.Printf("   Reason:             Ruby gem has extensions (platform gem unavailable)\n")
		}
	} else if !gem.hasExtensions {
		fmt.Printf("   Would build:        NO\n")
		fmt.Printf("   Reason:             No extensions declared\n")
	} else if !ctx.engine.SupportsNativeExtensions() {
		fmt.Printf("   Would build:        NO\n")
		fmt.Printf("   Reason:             Engine doesn't support native extensions\n")
	} else {
		fmt.Printf("   Would build:        YES\n")
		fmt.Printf("   Reason:             Has extensions and engine supports building\n")
	}
}

// displayEngineCompatibility shows engine compatibility analysis
func displayEngineCompatibility(gem *gemDebugInfo, ctx debugContext) {
	fmt.Printf("⚙️  Engine Compatibility:\n")

	mockSpec := lockfile.GemSpec{
		Name:       gem.name,
		Version:    gem.targetVersion,
		Platform:   ctx.requestedPlatform,
		Extensions: gem.gemExtensions,
	}

	compatible := ctx.engineChecker.IsCompatible(mockSpec)
	fmt.Printf("   Compatible:         %v\n", compatible)
	if !compatible {
		reason := ctx.engineChecker.GetIncompatibilityReason(mockSpec)
		fmt.Printf("   Reason:             %s\n", reason)
	} else {
		fmt.Printf("   Can install:        YES\n")
	}
	fmt.Println()
}

// displayDependencies shows gem dependencies
func displayDependencies(gem *gemDebugInfo) {
	runtimeDeps := gem.info.Dependencies.Runtime
	devDeps := gem.info.Dependencies.Development

	fmt.Printf("📚 Dependencies:\n")
	fmt.Printf("   Runtime:            %d\n", len(runtimeDeps))
	if len(runtimeDeps) > 0 {
		for _, dep := range runtimeDeps {
			fmt.Printf("     - %s %s\n", dep.Name, dep.Requirements)
		}
	}
	fmt.Printf("   Development:        %d\n", len(devDeps))
	if len(devDeps) > 0 {
		for _, dep := range devDeps {
			fmt.Printf("     - %s %s\n", dep.Name, dep.Requirements)
		}
	}
	fmt.Println()
}

// displayRecommendations shows actionable recommendations
func displayRecommendations(gem *gemDebugInfo, ctx debugContext) {
	fmt.Printf("💡 Recommendations:\n")

	if !gem.rubyGemExists && !gem.platformGemExists {
		fmt.Printf("   → Run `ore fetch %s` to download gem\n", gem.name)
	}
	if !gem.rubyInstalled && !gem.platformInstalled {
		fmt.Printf("   → Run `ore install` to install gem\n")
	}
	if ctx.requestedPlatform != "" && ctx.requestedPlatform != "ruby" && !gem.platformGemExists {
		fmt.Printf("   → Platform-specific gem for %s not available\n", ctx.requestedPlatform)
		if gem.rubyGemExists && gem.hasExtensions {
			fmt.Printf("   → Will fall back to building ruby gem from source\n")
			fmt.Printf("   → Ensure build tools are installed (gcc, make, ruby-dev)\n")
		}
	}
	if gem.hasExtensions && !gem.isPlatformSpecific && ctx.engine.SupportsNativeExtensions() && (gem.rubyGemExists || gem.rubyInstalled) {
		fmt.Printf("   → Ensure build tools are installed (gcc, make, ruby-dev)\n")
	}
	if gem.isPlatformSpecific {
		fmt.Printf("   → Using platform-specific gem - no compilation needed\n")
	}
	fmt.Println()
}

// Helper functions
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func detectRubyVersionForInfo() string {
	lockfilePath := config.DefaultLockfilePath()
	gemfilePath := defaultGemfilePath()
	return ruby.DetectRubyVersion(lockfilePath, gemfilePath, config.ToMajorMinor, ruby.DefaultRubyVersion)
}

func getSystemGemDirForInfo() string {
	return ruby.GetSystemGemDir(detectRubyVersionForInfo)
}
