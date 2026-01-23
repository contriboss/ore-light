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

// showDebugInfo displays comprehensive diagnostic information about how a gem will be handled
func showDebugInfo(ctx context.Context, client *registry.Client, gems []string, requestedVersion, requestedPlatform string) error {
	// Detect system environment
	engine := ruby.DetectEngine()
	engineChecker := resolver.NewEngineCompatibility(engine)

	// Get cache and vendor directories
	cfg := config.Load()
	cacheDir, _ := config.DefaultCacheDir(cfg)
	vendorDir := config.DefaultVendorDir(cfg, detectRubyVersionForInfo, getSystemGemDirForInfo)

	// Detect current platform
	currentPlatform := detectCurrentPlatform()
	if requestedPlatform == "" {
		requestedPlatform = currentPlatform
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  ORE DEBUG MODE - Gem Installation Diagnostics")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	fmt.Printf("📍 System Information:\n")
	fmt.Printf("   Ruby Engine:        %s %s\n", engine.Name, engine.Version)
	fmt.Printf("   Native Extensions:  %v\n", engine.SupportsNativeExtensions())
	fmt.Printf("   Current Platform:   %s\n", currentPlatform)
	fmt.Printf("   Requested Platform: %s\n", requestedPlatform)
	fmt.Printf("   Cache Directory:    %s\n", cacheDir)
	fmt.Printf("   Vendor Directory:   %s\n", vendorDir)
	fmt.Println()

	for _, gemName := range gems {
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		fmt.Printf("  GEM: %s\n", gemName)
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		fmt.Println()

		// Get available versions
		versions, err := client.GetGemVersions(ctx, gemName)
		if err != nil {
			fmt.Printf("❌ Error fetching versions: %v\n\n", err)
			continue
		}

		if len(versions) == 0 {
			fmt.Printf("❌ No versions found\n\n")
			continue
		}

		// Determine version to inspect
		targetVersion := requestedVersion
		if targetVersion == "" {
			targetVersion = versions[0] // Latest
		}

		fmt.Printf("📦 Version Information:\n")
		fmt.Printf("   Available versions: %d total\n", len(versions))
		fmt.Printf("   Latest version:     %s\n", versions[0])
		fmt.Printf("   Inspecting version: %s\n", targetVersion)
		fmt.Println()

		// Get gem info
		info, err := client.GetGemInfo(ctx, gemName, targetVersion)
		if err != nil {
			fmt.Printf("❌ Error fetching gem info: %v\n\n", err)
			continue
		}

		// Check cache status
		fmt.Printf("💾 Cache Status:\n")

		// Check for ruby platform
		rubyGemFile := fmt.Sprintf("%s-%s.gem", gemName, targetVersion)
		rubyGemPath := filepath.Join(cacheDir, rubyGemFile)
		rubyGemExists := fileExists(rubyGemPath)

		// Check for platform-specific
		platformGemFile := fmt.Sprintf("%s-%s-%s.gem", gemName, targetVersion, requestedPlatform)
		platformGemPath := filepath.Join(cacheDir, platformGemFile)
		platformGemExists := fileExists(platformGemPath)

		if rubyGemExists {
			fmt.Printf("   ✅ Ruby gem cached:     %s\n", rubyGemFile)
		} else {
			fmt.Printf("   ❌ Ruby gem NOT cached: %s\n", rubyGemFile)
		}

		if requestedPlatform != "" && requestedPlatform != "ruby" {
			if platformGemExists {
				fmt.Printf("   ✅ Platform gem cached: %s\n", platformGemFile)
			} else {
				fmt.Printf("   ❌ Platform gem NOT cached: %s\n", platformGemFile)
			}
		}
		fmt.Println()

		// Check installation status
		fmt.Printf("📂 Installation Status:\n")

		// Ruby version
		rubyGemFullName := fmt.Sprintf("%s-%s", gemName, targetVersion)
		rubyInstallDir := filepath.Join(vendorDir, "gems", rubyGemFullName)
		rubyInstalled := dirExists(rubyInstallDir)

		// Platform version
		platformGemFullName := rubyGemFullName
		if requestedPlatform != "" && requestedPlatform != "ruby" {
			platformGemFullName = fmt.Sprintf("%s-%s-%s", gemName, targetVersion, requestedPlatform)
		}
		platformInstallDir := filepath.Join(vendorDir, "gems", platformGemFullName)
		platformInstalled := dirExists(platformInstallDir)

		if rubyInstalled {
			fmt.Printf("   ✅ Ruby version installed:     %s\n", rubyInstallDir)
		} else {
			fmt.Printf("   ❌ Ruby version NOT installed: %s\n", rubyInstallDir)
		}

		if requestedPlatform != "" && requestedPlatform != "ruby" {
			if platformInstalled {
				fmt.Printf("   ✅ Platform version installed: %s\n", platformInstallDir)
			} else {
				fmt.Printf("   ❌ Platform version NOT installed: %s\n", platformInstallDir)
			}
		}
		fmt.Println()

		// Extension information
		fmt.Printf("🔧 Extension Information:\n")

		// Try to get extension info from cached gem
		var gemExtensions []string
		var hasExtensions bool

		// Check if gem is cached and extract metadata
		if rubyGemExists {
			metadata, err := geminstall.ExtractMetadataOnly(rubyGemPath)
			if err == nil && len(metadata) > 0 {
				exts, err := geminstall.ParseExtensionsFromMetadata(metadata)
				if err == nil {
					gemExtensions = exts
					hasExtensions = len(exts) > 0
				}
			}
		} else if platformGemExists {
			metadata, err := geminstall.ExtractMetadataOnly(platformGemPath)
			if err == nil && len(metadata) > 0 {
				exts, err := geminstall.ParseExtensionsFromMetadata(metadata)
				if err == nil {
					gemExtensions = exts
					hasExtensions = len(exts) > 0
				}
			}
		}

		if rubyGemExists || platformGemExists {
			fmt.Printf("   Has extensions:     %v\n", hasExtensions)

			if hasExtensions {
				fmt.Printf("   Extensions declared: %d\n", len(gemExtensions))
				for _, ext := range gemExtensions {
					fmt.Printf("     - %s\n", ext)
				}
			} else {
				fmt.Printf("   Extensions:         (none - pure Ruby or precompiled)\n")
			}
		} else {
			fmt.Printf("   ⚠️  Gem not cached - download to see extension details\n")
			fmt.Printf("   Run: ore fetch %s --version %s\n", gemName, targetVersion)
		}
		fmt.Println()

		// Platform-specific behavior analysis
		fmt.Printf("🎯 Platform-Specific Behavior:\n")

		isPlatformSpecific := requestedPlatform != "" && requestedPlatform != "ruby"

		if isPlatformSpecific {
			fmt.Printf("   Platform gem:       YES (%s)\n", requestedPlatform)
			fmt.Printf("   Precompiled:        Assumed YES (platform-specific gems are precompiled)\n")
			fmt.Printf("   Build from source:  NO (will use precompiled binaries)\n")
		} else {
			fmt.Printf("   Platform gem:       NO (ruby platform)\n")
			if rubyGemExists || platformGemExists {
				if hasExtensions {
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

		// Extension build analysis
		fmt.Printf("🏗️  Extension Build Analysis:\n")

		// Create mock GemSpec for analysis
		mockSpec := lockfile.GemSpec{
			Name:       gemName,
			Version:    targetVersion,
			Platform:   requestedPlatform,
			Extensions: gemExtensions, // Use extensions from cached gem if available
		}

		// Check if extensions would be built
		if platformInstalled {
			needsBuild, err := extensions.NeedsBuild(platformInstallDir, mockSpec.Extensions, mockSpec.Platform, engine)
			if err != nil {
				fmt.Printf("   ⚠️  Error checking build status: %v\n", err)
			} else {
				fmt.Printf("   Needs building:     %v\n", needsBuild)
				if needsBuild {
					fmt.Printf("   Reason:             Extensions declared but not built\n")
				} else if isPlatformSpecific {
					fmt.Printf("   Reason:             Platform-specific gem (precompiled)\n")
				} else if !hasExtensions {
					fmt.Printf("   Reason:             No extensions declared\n")
				} else {
					fmt.Printf("   Reason:             Already built (gem.build_complete marker exists)\n")
				}
			}
		} else {
			// Predict what would happen on install
			if isPlatformSpecific {
				fmt.Printf("   Would build:        NO\n")
				fmt.Printf("   Reason:             Platform-specific gems are precompiled\n")
			} else if !hasExtensions {
				fmt.Printf("   Would build:        NO\n")
				fmt.Printf("   Reason:             No extensions declared\n")
			} else if !engine.SupportsNativeExtensions() {
				fmt.Printf("   Would build:        NO\n")
				fmt.Printf("   Reason:             Engine doesn't support native extensions\n")
			} else {
				fmt.Printf("   Would build:        YES\n")
				fmt.Printf("   Reason:             Has extensions and engine supports building\n")
			}
		}
		fmt.Println()

		// Engine compatibility
		fmt.Printf("⚙️  Engine Compatibility:\n")
		compatible := engineChecker.IsCompatible(mockSpec)
		fmt.Printf("   Compatible:         %v\n", compatible)
		if !compatible {
			reason := engineChecker.GetIncompatibilityReason(mockSpec)
			fmt.Printf("   Reason:             %s\n", reason)
		} else {
			fmt.Printf("   Can install:        YES\n")
		}
		fmt.Println()

		// Dependencies
		runtimeDeps := info.Dependencies.Runtime
		devDeps := info.Dependencies.Development

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

		// Action recommendations
		fmt.Printf("💡 Recommendations:\n")
		if !rubyGemExists && !platformGemExists {
			fmt.Printf("   → Run `ore fetch %s` to download gem\n", gemName)
		}
		if !rubyInstalled && !platformInstalled {
			fmt.Printf("   → Run `ore install` to install gem\n")
		}
		if hasExtensions && !isPlatformSpecific && engine.SupportsNativeExtensions() {
			fmt.Printf("   → Ensure build tools are installed (gcc, make, ruby-dev)\n")
		}
		if isPlatformSpecific {
			fmt.Printf("   → Use platform-specific gem for faster installation (no compilation)\n")
		}
		fmt.Println()
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	return nil
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
