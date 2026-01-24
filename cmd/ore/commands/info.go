package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	displayBundlerCompatibility(gemInfo, debugCtx) // NEW: Compare ore vs bundler
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

// displayBundlerCompatibility shows detailed comparison of ore install vs bundler expectations
func displayBundlerCompatibility(gem *gemDebugInfo, ctx debugContext) {
	fmt.Printf("🔍 BUNDLER COMPATIBILITY CHECK:\n")
	fmt.Println("   (This section helps diagnose why bundle exec might fail after ore install)")
	fmt.Println()

	// Check which install directory to inspect
	var installDir string
	var gemFullName string
	if gem.platformInstalled {
		installDir = gem.platformInstallDir
		gemFullName = filepath.Base(gem.platformInstallDir)
	} else if gem.rubyInstalled {
		installDir = gem.rubyInstallDir
		gemFullName = filepath.Base(gem.rubyInstallDir)
	} else {
		fmt.Printf("   ⚠️  Gem not installed - nothing to check\n\n")
		return
	}

	specDir := filepath.Join(ctx.vendorDir, "specifications")
	specPath := filepath.Join(specDir, gemFullName+".gemspec")

	fmt.Printf("   Gem Full Name:      %s\n", gemFullName)
	fmt.Printf("   Install Directory:  %s\n", installDir)
	fmt.Printf("   Gemspec Path:       %s\n", specPath)
	fmt.Println()

	// 1. Check gemspec exists
	fmt.Printf("   [1] GEMSPEC FILE:\n")
	if fileExists(specPath) {
		fmt.Printf("       ✅ Exists: %s\n", specPath)

		// Read and display first 20 lines of gemspec
		content, err := os.ReadFile(specPath)
		if err != nil {
			fmt.Printf("       ❌ Error reading: %v\n", err)
		} else {
			lines := splitLines(string(content))
			fmt.Printf("       Content (first 20 lines):\n")
			for i, line := range lines {
				if i >= 20 {
					fmt.Printf("       ... (%d more lines)\n", len(lines)-20)
					break
				}
				fmt.Printf("       %3d: %s\n", i+1, line)
			}

			// Check for critical fields
			fmt.Printf("\n       Critical Field Checks:\n")
			checkGemspecField(content, "installed_by_version", "Required for Bundler to recognize as installed")
			checkGemspecField(content, "s.name", "Gem name declaration")
			checkGemspecField(content, "s.version", "Version declaration")
			checkGemspecField(content, "s.require_paths", "Load path configuration")
			checkGemspecField(content, "# stub:", "RubyGems stub line for fast loading")
		}
	} else {
		fmt.Printf("       ❌ MISSING: %s\n", specPath)
		fmt.Printf("       This is the PRIMARY reason bundle exec would fail!\n")
	}
	fmt.Println()

	// 2. Check gem directory structure
	fmt.Printf("   [2] GEM DIRECTORY STRUCTURE:\n")
	if dirExists(installDir) {
		fmt.Printf("       ✅ Gem directory exists: %s\n", installDir)

		// List contents
		entries, err := os.ReadDir(installDir)
		if err != nil {
			fmt.Printf("       ❌ Error reading directory: %v\n", err)
		} else {
			fmt.Printf("       Contents (%d items):\n", len(entries))
			for _, entry := range entries {
				entryType := "📄"
				if entry.IsDir() {
					entryType = "📁"
				}
				fmt.Printf("         %s %s\n", entryType, entry.Name())
			}

			// Check for lib directory (critical for require_paths)
			libDir := filepath.Join(installDir, "lib")
			if dirExists(libDir) {
				fmt.Printf("       ✅ lib/ directory exists (required for require_paths)\n")

				// List lib contents
				libEntries, err := os.ReadDir(libDir)
				if err == nil && len(libEntries) > 0 {
					fmt.Printf("       lib/ contents:\n")
					for i, entry := range libEntries {
						if i >= 10 {
							fmt.Printf("         ... (%d more)\n", len(libEntries)-10)
							break
						}
						fmt.Printf("         - %s\n", entry.Name())
					}
				}
			} else {
				fmt.Printf("       ⚠️  lib/ directory missing - check require_paths in gemspec\n")
			}
		}
	} else {
		fmt.Printf("       ❌ MISSING: %s\n", installDir)
	}
	fmt.Println()

	// 3. Check Gem.path includes vendor directory
	fmt.Printf("   [3] GEM PATH ANALYSIS:\n")
	gemPath := os.Getenv("GEM_PATH")
	gemHome := os.Getenv("GEM_HOME")
	fmt.Printf("       GEM_HOME: %s\n", gemHome)
	fmt.Printf("       GEM_PATH: %s\n", gemPath)
	fmt.Printf("       Vendor dir (should be in GEM_HOME or GEM_PATH): %s\n", ctx.vendorDir)

	if gemHome == ctx.vendorDir {
		fmt.Printf("       ✅ GEM_HOME matches vendor directory\n")
	} else if gemHome != "" && filepath.Clean(gemHome) == filepath.Clean(ctx.vendorDir) {
		fmt.Printf("       ✅ GEM_HOME matches vendor directory (after normalization)\n")
	} else {
		fmt.Printf("       ⚠️  GEM_HOME does NOT match vendor directory\n")
		fmt.Printf("          Bundler may not find gems installed by ore!\n")
	}
	fmt.Println()

	// 4. Check bundler-specific files
	fmt.Printf("   [4] BUNDLER-SPECIFIC FILES:\n")

	// Check for gem.build_complete (for gems with extensions)
	if gem.hasExtensions {
		buildComplete := filepath.Join(ctx.vendorDir, "extensions", ctx.currentPlatform, gem.name+"-"+gem.targetVersion, "gem.build_complete")
		// Also check common extension paths
		extDirs := []string{
			filepath.Join(ctx.vendorDir, "extensions"),
		}
		fmt.Printf("       Has extensions: YES\n")
		for _, extDir := range extDirs {
			if dirExists(extDir) {
				fmt.Printf("       Extensions dir exists: %s\n", extDir)
			}
		}
		if fileExists(buildComplete) {
			fmt.Printf("       ✅ gem.build_complete exists\n")
		} else {
			fmt.Printf("       ⚠️  gem.build_complete NOT found at expected location\n")
			fmt.Printf("          Expected: %s\n", buildComplete)
		}
	} else {
		fmt.Printf("       Has extensions: NO (gem.build_complete not required)\n")
	}

	// Check for cache/*.gem
	cacheDir := filepath.Join(ctx.vendorDir, "cache")
	cachedGem := filepath.Join(cacheDir, gemFullName+".gem")
	if fileExists(cachedGem) {
		fmt.Printf("       ✅ Cached .gem file: %s\n", cachedGem)
	} else {
		fmt.Printf("       ⚠️  No cached .gem in vendor/cache (may affect bundle install --local)\n")
	}
	fmt.Println()

	// 5. Compare with what a bundle install would create
	fmt.Printf("   [5] BUNDLE INSTALL COMPARISON:\n")
	fmt.Printf("       What bundle install creates that ore MIGHT be missing:\n")
	fmt.Printf("       • specifications/*.gemspec - CHECKED ABOVE\n")
	fmt.Printf("       • gems/<name>-<version>/ - CHECKED ABOVE\n")
	fmt.Printf("       • cache/<name>-<version>.gem - CHECKED ABOVE\n")
	fmt.Printf("       • bin/ executables (if gem has them)\n")
	fmt.Printf("       • doc/ documentation (if not --no-document)\n")
	fmt.Printf("       • extensions/ (if gem has native extensions)\n")
	fmt.Println()

	// Check bin directory for executables
	binDir := filepath.Join(ctx.vendorDir, "bin")
	if dirExists(binDir) {
		fmt.Printf("       Bin directory exists: %s\n", binDir)
	}
	fmt.Println()

	// 6. Run Ruby diagnostic to check what RubyGems/Bundler actually sees
	fmt.Printf("   [6] RUBY/BUNDLER DIAGNOSTIC:\n")
	fmt.Printf("       Running Ruby to check what RubyGems actually sees...\n\n")
	runRubyDiagnostic(gem.name, gem.targetVersion, ctx.vendorDir)
	fmt.Println()
}

// checkGemspecField checks if a field exists in gemspec content
func checkGemspecField(content []byte, field, description string) {
	if containsBytes(content, []byte(field)) {
		fmt.Printf("         ✅ %s - %s\n", field, description)
	} else {
		fmt.Printf("         ❌ %s MISSING - %s\n", field, description)
	}
}

// containsBytes checks if b contains substr
func containsBytes(b, substr []byte) bool {
	return len(substr) > 0 && len(b) >= len(substr) && bytesContains(b, substr)
}

// bytesContains is a simple contains check
func bytesContains(b, substr []byte) bool {
	for i := 0; i <= len(b)-len(substr); i++ {
		if bytesEqual(b[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// bytesEqual checks if two byte slices are equal
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// runRubyDiagnostic runs a Ruby script to check what RubyGems/Bundler actually sees
func runRubyDiagnostic(gemName, version, vendorDir string) {
	// Comprehensive Ruby diagnostic script
	rubyScript := fmt.Sprintf(`
require 'rubygems'
require 'pp'

gem_name = %q
gem_version = %q
vendor_dir = %q

puts "=" * 60
puts "RUBYGEMS DIAGNOSTIC FOR: #{gem_name}-#{gem_version}"
puts "=" * 60
puts

# 1. Environment
puts "[A] ENVIRONMENT:"
puts "   GEM_HOME:      #{ENV['GEM_HOME']}"
puts "   GEM_PATH:      #{ENV['GEM_PATH']}"
puts "   Gem.dir:       #{Gem.dir}"
puts "   Gem.path:      #{Gem.path.join(':')}"
puts "   Gem.user_dir:  #{Gem.user_dir}"
puts

# 2. Specification directories
puts "[B] SPECIFICATION DIRECTORIES:"
Gem::Specification.dirs.each_with_index do |dir, i|
  exists = File.directory?(dir) ? "EXISTS" : "MISSING"
  puts "   #{i+1}. #{dir} [#{exists}]"
end
puts

# 3. Search for this gem in stubs
puts "[C] SEARCHING FOR GEM IN STUBS:"
begin
  stubs = Gem::Specification.stubs_for(gem_name)
  if stubs.empty?
    puts "   ❌ NO STUBS FOUND for #{gem_name}"
    puts "   This means RubyGems cannot find the gem at all!"
  else
    puts "   Found #{stubs.count} stub(s):"
    stubs.each do |stub|
      puts "   - #{stub.full_name}"
      puts "     loaded_from: #{stub.loaded_from}"
      puts "     full_gem_path: #{stub.full_gem_path}"
      path_exists = File.directory?(stub.full_gem_path) ? "EXISTS" : "MISSING"
      puts "     path exists: #{path_exists}"
      puts "     valid?: #{stub.valid?}"
      puts "     stubbed?: #{stub.stubbed?}" if stub.respond_to?(:stubbed?)
      puts
    end
  end
rescue => e
  puts "   Error getting stubs: #{e.message}"
end
puts

# 4. Try to find the specific version
puts "[D] FINDING SPECIFIC VERSION #{gem_version}:"
begin
  specs = Gem::Specification.find_all_by_name(gem_name, "= #{gem_version}")
  if specs.empty?
    puts "   ❌ NOT FOUND"

    # Try without version constraint
    all_specs = Gem::Specification.find_all_by_name(gem_name)
    if all_specs.any?
      puts "   But found these versions:"
      all_specs.each { |s| puts "     - #{s.version}" }
    end
  else
    specs.each do |spec|
      puts "   ✅ FOUND: #{spec.full_name}"
      puts "      loaded_from:            #{spec.loaded_from}"
      puts "      full_gem_path:          #{spec.full_gem_path}"
      puts "      gem_dir:                #{spec.gem_dir}"
      puts "      require_paths:          #{spec.require_paths.inspect}"
      puts "      full_require_paths:     #{spec.full_require_paths.inspect}"
      puts "      installation_missing?:  #{spec.respond_to?(:installation_missing?) ? spec.installation_missing? : 'N/A'}"
      puts "      default_gem?:           #{spec.default_gem?}"
      puts "      installed_by_version:   #{spec.installed_by_version rescue 'N/A'}"
      puts

      # Check if full_gem_path exists
      if File.directory?(spec.full_gem_path)
        puts "      Gem directory contents:"
        Dir.entries(spec.full_gem_path).reject { |e| e.start_with?('.') }.each do |entry|
          type = File.directory?(File.join(spec.full_gem_path, entry)) ? "📁" : "📄"
          puts "        #{type} #{entry}"
        end
      else
        puts "      ❌ full_gem_path DOES NOT EXIST!"
      end
    end
  end
rescue => e
  puts "   Error: #{e.message}"
  puts e.backtrace.first(5).join("\n")
end
puts

# 5. Check the actual gemspec file content
puts "[E] GEMSPEC FILE ANALYSIS:"
spec_path = File.join(vendor_dir, "specifications", "#{gem_name}-#{gem_version}.gemspec")
if File.exist?(spec_path)
  puts "   Found: #{spec_path}"
  puts "   First 15 lines:"
  File.readlines(spec_path).first(15).each_with_index do |line, i|
    puts "   #{(i+1).to_s.rjust(3)}: #{line.chomp}"
  end

  # Try to load the gemspec
  puts
  puts "   Attempting to load gemspec..."
  begin
    loaded_spec = Gem::Specification.load(spec_path)
    if loaded_spec
      puts "   ✅ Gemspec loads successfully"
      puts "      name: #{loaded_spec.name}"
      puts "      version: #{loaded_spec.version}"
      puts "      platform: #{loaded_spec.platform}"
    else
      puts "   ❌ Gemspec.load returned nil!"
    end
  rescue => e
    puts "   ❌ Error loading gemspec: #{e.message}"
  end
else
  puts "   ❌ Gemspec NOT FOUND at #{spec_path}"
end
puts

# 6. Bundler check (if available)
puts "[F] BUNDLER CHECK:"
begin
  require 'bundler'
  puts "   Bundler version: #{Bundler::VERSION}"
  puts "   Bundle path: #{Bundler.bundle_path rescue 'N/A'}"
  puts "   Bundler settings: #{Bundler.settings.path rescue 'N/A'}"

  if defined?(Bundler::Definition)
    puts "   Attempting to check Bundler's view..."
    begin
      # Just check installed_specs
      installed = Bundler.rubygems.installed_specs.select { |s| s.name == gem_name }
      if installed.any?
        puts "   ✅ Bundler sees #{installed.count} installed version(s) of #{gem_name}:"
        installed.each { |s| puts "      - #{s.full_name} at #{s.loaded_from}" }
      else
        puts "   ❌ Bundler does NOT see #{gem_name} as installed!"
        puts "      This explains why bundle exec fails!"
      end
    rescue => e
      puts "   Error checking Bundler: #{e.message}"
    end
  end
rescue LoadError
  puts "   Bundler not available"
rescue => e
  puts "   Error: #{e.message}"
end
puts

puts "=" * 60
puts "END DIAGNOSTIC"
puts "=" * 60
`, gemName, version, vendorDir)

	cmd := exec.Command("ruby", "-e", rubyScript)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("       ⚠️  Ruby diagnostic failed: %v\n", err)
		if len(output) > 0 {
			fmt.Printf("       Output:\n")
			for _, line := range strings.Split(string(output), "\n") {
				fmt.Printf("       %s\n", line)
			}
		}
		return
	}

	// Print output with indentation
	for _, line := range strings.Split(string(output), "\n") {
		fmt.Printf("       %s\n", line)
	}
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
			// Determine whether the platform gem appears to be precompiled by checking metadata
			if !gem.hasExtensions {
				fmt.Printf("   Precompiled:        YES (platform gem declares no extensions)\n")
				fmt.Printf("   Build from source:  NO (will use precompiled binaries)\n")
			} else {
				fmt.Printf("   Precompiled:        NO (platform gem declares extensions)\n")
				fmt.Printf("   Build from source:  YES (platform gem contains native extensions)\n")
			}
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
				fmt.Printf("   Precompiled:        N/A (ruby/platform gem declares extensions)\n")
				fmt.Printf("   Build from source:  YES (has extensions)\n")
			} else {
				fmt.Printf("   Precompiled:        N/A (pure Ruby or precompiled)\n")
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
			} else if !gem.hasExtensions {
				fmt.Printf("   Reason:             No extensions declared (pure Ruby or precompiled)\n")
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
	if ctx.requestedPlatform != "" && ctx.requestedPlatform != "ruby" && gem.platformGemExists {
		// Platform gem exists - decide based on metadata
		if !gem.hasExtensions {
			fmt.Printf("   Would build:        NO\n")
			fmt.Printf("   Reason:             Platform gem declares no extensions (precompiled)\n")
		} else if !ctx.engine.SupportsNativeExtensions() {
			fmt.Printf("   Would build:        NO\n")
			fmt.Printf("   Reason:             Engine doesn't support native extensions\n")
		} else {
			fmt.Printf("   Would build:        YES\n")
			fmt.Printf("   Reason:             Platform gem declares extensions and would need building\n")
		}
	} else if ctx.requestedPlatform != "" && ctx.requestedPlatform != "ruby" && !gem.platformGemExists {
		// User requested a platform, but it doesn't exist - would fall back to ruby gem
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
