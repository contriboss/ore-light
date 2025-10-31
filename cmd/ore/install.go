package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
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
	"github.com/contriboss/ore-light/internal/sources"
)

// Ruby developers: This is like a result object from bundle install
// Tracks what was installed, skipped, and extension build results
type installReport struct {
	Total             int
	Installed         int
	Skipped           int
	ExtensionsBuilt   int
	ExtensionsSkipped int
	ExtensionsFailed  int
}

// extensionTarget tracks a gem that needs extensions built
type extensionTarget struct {
	gemName string
	destDir string
}

// installBuildDependency fetches and installs a build-time dependency gem (like rake)
// Returns error if fetch or install fails
func installBuildDependency(ctx context.Context, gemName, cacheDir, vendorDir string, verbose bool) error {
	// Create registry client
	client, err := registry.NewClient("https://rubygems.org", registry.ProtocolRubygems)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	// Get latest version
	versions, err := client.GetGemVersions(ctx, gemName)
	if err != nil {
		return fmt.Errorf("failed to get versions for %s: %w", gemName, err)
	}
	if len(versions) == 0 {
		return fmt.Errorf("no versions found for %s", gemName)
	}
	targetVersion := versions[0]

	// Construct gem filename (platform-independent for build tools)
	gemFileName := fmt.Sprintf("%s-%s.gem", gemName, targetVersion)
	cachedPath := filepath.Join(cacheDir, gemFileName)

	// Check if already cached
	if _, err := os.Stat(cachedPath); err != nil {
		// Need to download
		if verbose {
			fmt.Printf("📦 Fetching build dependency %s-%s...\n", gemName, targetVersion)
		}

		// Create source manager for download
		sourceManager := sources.NewManager([]sources.SourceConfig{
			{URL: "https://rubygems.org", Fallback: ""},
		}, nil)

		outFile, err := os.Create(cachedPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer func() {
			_ = outFile.Close()
		}()

		if err := sourceManager.DownloadGem(ctx, gemFileName, outFile); err != nil {
			return fmt.Errorf("failed to download %s: %w", gemName, err)
		}
	}

	// Extract to vendor/gems
	gemSpec := lockfile.GemSpec{
		Name:    gemName,
		Version: targetVersion,
	}
	destDir := filepath.Join(vendorDir, "gems", gemSpec.FullName())

	// Extract gem contents AND metadata
	metadata, err := geminstall.ExtractGemContents(cachedPath, destDir)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", gemName, err)
	}

	// Write gemspec so Ruby can find the gem
	if len(metadata) > 0 {
		if err := geminstall.WriteGemSpecification(vendorDir, gemSpec, metadata); err != nil {
			return fmt.Errorf("failed to write gemspec for %s: %w", gemName, err)
		}
	}

	// Link binaries
	if err := geminstall.LinkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
		return fmt.Errorf("failed to link binaries for %s: %w", gemName, err)
	}

	if verbose {
		fmt.Printf("✓ Installed build dependency %s-%s\n", gemName, targetVersion)
	}

	return nil
}

func installFromCache(ctx context.Context, cacheDir, vendorDir string, gems []lockfile.GemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(gems)}

	// Detect Ruby engine for compatibility filtering
	engine := ruby.DetectEngine()
	engineChecker := resolver.NewEngineCompatibility(engine)

	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return report, err
	}
	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "cache")); err != nil {
		return report, err
	}
	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "bin")); err != nil {
		return report, err
	}
	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "specifications", "cache")); err != nil {
		return report, err
	}

	// Create extension builder
	extBuilder := extensions.NewBuilder(extConfig)

	// Collect gems that need extensions built (defer until all gems installed)
	var extensionTargets []extensionTarget

	for _, gem := range gems {
		gemPath := findGemInCaches(cacheDir, gem)
		if gemPath == "" {
			return report, fmt.Errorf("gem %s is not cached; run `ore download` first", gem.FullName())
		}

		destDir := filepath.Join(vendorDir, "gems", gem.FullName())

		// Smart skip logic
		if _, err := os.Stat(destDir); err == nil && !force {
			// If buildExtensions mode is enabled, check if this gem needs extension building
			if buildExtensions {
				needsBuild, err := extensions.NeedsBuild(destDir, engine)
				if err != nil {
					return report, fmt.Errorf("failed to check if %s needs extension build: %w", gem.FullName(), err)
				}
				if needsBuild {
					// Don't skip - this gem has extensions that need building
					extensionTargets = append(extensionTargets, extensionTarget{
						gemName: gem.FullName(),
						destDir: destDir,
					})
				}
			}
			report.Skipped++
			continue
		}

		// Performance optimization: Extract only metadata first to check compatibility
		// This avoids unpacking the entire data.tar.gz for incompatible gems
		metadata, err := geminstall.ExtractMetadataOnly(gemPath)
		if err != nil {
			return report, fmt.Errorf("failed to extract metadata from %s: %w", gem.FullName(), err)
		}

		// Check engine compatibility BEFORE full extraction
		// Parse metadata to populate gem.Extensions for compatibility check
		if len(metadata) > 0 {
			// Parse extensions from metadata YAML
			gemWithExtensions := gem
			extensions, err := geminstall.ParseExtensionsFromMetadata(metadata)
			if err != nil {
				// Failed to parse metadata - be conservative and assume native extensions
				if extConfig != nil && extConfig.Verbose {
					fmt.Fprintf(os.Stderr, "⚠️  Warning: %s metadata parse error: %v (assuming native extensions)\n", gem.FullName(), err)
				}
				// Create a sentinel extension to trigger native extension check
				gemWithExtensions.Extensions = []string{"ext/extconf.rb"}
			} else if len(extensions) > 0 {
				gemWithExtensions.Extensions = extensions
			}

			// Check if gem is compatible with current Ruby engine
			if !engineChecker.IsCompatible(gemWithExtensions) {
				reason := engineChecker.GetIncompatibilityReason(gemWithExtensions)
				if extConfig != nil && extConfig.Verbose {
					fmt.Printf("⚠️  Skipping %s: %s\n", gem.FullName(), reason)
				}
				report.Skipped++
				continue
			}
		}

		// Gem is compatible - proceed with full extraction
		if err := os.RemoveAll(destDir); err != nil {
			return report, fmt.Errorf("failed to clean install dir for %s: %w", gem.FullName(), err)
		}

		_, err = geminstall.ExtractGemContents(gemPath, destDir)
		if err != nil {
			return report, fmt.Errorf("failed to extract %s: %w", gem.FullName(), err)
		}

		if err := geminstall.CopyGemToVendorCache(gemPath, filepath.Join(vendorDir, "cache", gemFileName(gem))); err != nil {
			return report, err
		}

		if len(metadata) > 0 {
			if err := geminstall.WriteGemSpecification(vendorDir, gem, metadata); err != nil {
				return report, err
			}
		}

		if err := geminstall.LinkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return report, err
		}

		// Collect this gem for extension building (defer until all gems installed)
		extensionTargets = append(extensionTargets, extensionTarget{
			gemName: gem.FullName(),
			destDir: destDir,
		})

		report.Installed++
	}

	// Build extensions for all installed gems (two-phase: install all, then build all)
	// This ensures all gem specifications are written before any extensions build,
	// allowing gems like nokogiri to find build dependencies like mini_portile2
	if extConfig != nil && extConfig.Verbose {
		fmt.Printf("Building extensions for %d gems after all installations complete...\n", len(extensionTargets))
	}
	buildPendingExtensions(ctx, extBuilder, engine, extensionTargets, &report, extConfig, cacheDir, vendorDir)

	return report, nil
}

// buildPendingExtensions builds extensions for all collected targets after installation
// This ensures all gem specifications are written before any extensions build,
// allowing gems like nokogiri to find build dependencies like mini_portile2
func buildPendingExtensions(ctx context.Context, extBuilder *extensions.Builder, engine ruby.Engine, targets []extensionTarget, report *installReport, extConfig *extensions.BuildConfig, cacheDir, vendorDir string) {
	// Skip if no extension config or extensions disabled
	if extConfig == nil || extConfig.SkipExtensions {
		return
	}

	for _, target := range targets {
		extResult, err := extBuilder.BuildExtensions(ctx, target.destDir, target.gemName, engine)

		// Check if build failed due to missing dependencies
		if (err != nil || !extResult.Success) && extResult != nil && len(extResult.MissingDependencies) > 0 {
			// Try to install missing build dependencies
			if extConfig.Verbose {
				fmt.Printf("Extension build for %s requires: %v\n", target.gemName, extResult.MissingDependencies)
			}

			// Determine cacheDir if not provided
			actualCacheDir := cacheDir
			if actualCacheDir == "" {
				// Import config package to get default cache dir
				var configErr error
				actualCacheDir, configErr = config.DefaultCacheDir(nil)
				if configErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to determine cache directory: %v\n", configErr)
					report.ExtensionsFailed++
					continue
				}
			}

			// Install each missing dependency
			allInstalled := true
			for _, dep := range extResult.MissingDependencies {
				if extConfig.Verbose {
					fmt.Printf("Installing build dependency: %s\n", dep)
				}
				if err := installBuildDependency(ctx, dep, actualCacheDir, vendorDir, extConfig.Verbose); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to install build dependency %s: %v\n", dep, err)
					allInstalled = false
					break
				}
			}

			if !allInstalled {
				report.ExtensionsFailed++
				continue
			}

			// Add vendorDir/bin to PATH so installed binstubs (like rake) can be found by exec.LookPath
			binDir := filepath.Join(vendorDir, "bin")
			currentPath := os.Getenv("PATH")
			var pathErr error
			if currentPath != "" {
				pathErr = os.Setenv("PATH", binDir+":"+currentPath)
			} else {
				pathErr = os.Setenv("PATH", binDir)
			}
			if pathErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to update PATH for build dependencies: %v\n", pathErr)
				report.ExtensionsFailed++
				continue
			}

			// Retry building extensions after installing dependencies
			if extConfig.Verbose {
				fmt.Printf("Retrying extension build for %s...\n", target.gemName)
			}
			extResult, err = extBuilder.BuildExtensions(ctx, target.destDir, target.gemName, engine)
		}

		// Check final result
		if err != nil || (extResult != nil && !extResult.Success) {
			// Extension build failure - warn but continue
			fmt.Fprintf(os.Stderr, "Warning: Failed to build extensions for %s: %v\n", target.gemName, err)
			report.ExtensionsFailed++
		} else if extResult.Skipped {
			report.ExtensionsSkipped++
		} else if extResult.Success && len(extResult.Extensions) > 0 {
			if extConfig.Verbose {
				fmt.Printf("Built %d extension(s) for %s: %v\n", len(extResult.Extensions), target.gemName, extResult.Extensions)
			}
			report.ExtensionsBuilt++
		}
	}
}

// findGemInCaches searches for a gem in cache directories (ore cache + system cache)
func findGemInCaches(primaryCache string, gem lockfile.GemSpec) string {
	fileName := gemFileName(gem)

	// Check primary ore cache first
	path := filepath.Join(primaryCache, fileName)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Try system RubyGems caches (only if Ruby available)
	if gemPaths := tryGetGemPathsForInstall(); len(gemPaths) > 0 {
		for _, gemPath := range gemPaths {
			systemCache := filepath.Join(gemPath, "cache", fileName)
			if _, err := os.Stat(systemCache); err == nil {
				return systemCache
			}
		}
	}

	return ""
}

// tryGetGemPathsForInstall uses same logic as download.go
func tryGetGemPathsForInstall() []string {
	cmd := exec.Command("gem", "environment", "gempath")
	output, err := cmd.Output()
	if err == nil {
		pathsStr := strings.TrimSpace(string(output))
		if pathsStr != "" {
			return strings.Split(pathsStr, string(filepath.ListSeparator))
		}
	}

	// Fallback to common locations
	var defaultPaths []string
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultPaths
	}

	rubyVer := detectRubyVersion()
	if rubyVer == "" {
		return defaultPaths
	}

	commonLocations := []string{
		filepath.Join(home, ".gem", "ruby", rubyVer),
		filepath.Join(home, ".local", "share", "gem", "ruby", rubyVer),
	}

	globPatterns := []string{
		filepath.Join(home, ".rbenv", "versions", "*", "lib", "ruby", "gems", rubyVer),
		filepath.Join(home, ".asdf", "installs", "ruby", "*", "lib", "ruby", "gems", rubyVer),
		filepath.Join(home, ".local", "share", "mise", "installs", "ruby", "*", "lib", "ruby", "gems", rubyVer),
	}

	for _, pattern := range globPatterns {
		if matches, err := filepath.Glob(pattern); err == nil {
			defaultPaths = append(defaultPaths, matches...)
		}
	}

	for _, location := range commonLocations {
		if _, err := os.Stat(location); err == nil {
			defaultPaths = append(defaultPaths, location)
		}
	}

	return defaultPaths
}

// Functions moved to internal/geminstall package

func buildExecutionEnv(vendorDir string, specs []lockfile.GemSpec) ([]string, error) {
	if err := geminstall.EnsureDir(vendorDir); err != nil {
		return nil, err
	}

	libPaths := collectLibraryPaths(vendorDir, specs)
	if len(libPaths) == 0 {
		return nil, fmt.Errorf("no gem libraries found under %s; run `ore install` first", vendorDir)
	}

	env := os.Environ()

	// Only set GEM_HOME/GEM_PATH when using non-system vendorDir (isolated install)
	// For system gem dir, Ruby's default Gem.dir works correctly
	systemGemDir := getSystemGemDir()
	if vendorDir != systemGemDir {
		env = setEnv(env, "GEM_HOME", vendorDir)
		env = setEnv(env, "GEM_PATH", vendorDir)
		// Disable Bundler's auto-setup to avoid conflicts
		env = setEnv(env, "BUNDLE_GEMFILE", "")
	}

	env = prependPath(env, filepath.Join(vendorDir, "bin"))
	env = prependRubyLib(env, libPaths)

	return env, nil
}

func collectLibraryPaths(vendorDir string, specs []lockfile.GemSpec) []string {
	seen := make(map[string]struct{})
	var libs []string

	for _, spec := range specs {
		libDir := filepath.Join(vendorDir, "gems", spec.FullName(), "lib")
		if _, err := os.Stat(libDir); err != nil {
			continue
		}
		if _, ok := seen[libDir]; ok {
			continue
		}
		seen[libDir] = struct{}{}
		libs = append(libs, libDir)
	}

	return libs
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func prependPath(env []string, path string) []string {
	if path == "" {
		return env
	}
	current, _ := getEnvValue(env, "PATH")
	if current == "" {
		return setEnv(env, "PATH", path)
	}
	return setEnv(env, "PATH", fmt.Sprintf("%s%c%s", path, os.PathListSeparator, current))
}

func prependRubyLib(env []string, libs []string) []string {
	if len(libs) == 0 {
		return env
	}
	libValue := strings.Join(libs, string(os.PathListSeparator))
	if current, _ := getEnvValue(env, "RUBYLIB"); current != "" {
		libValue = libValue + string(os.PathListSeparator) + current
	}
	return setEnv(env, "RUBYLIB", libValue)
}

func getEnvValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix), true
		}
	}
	if value, ok := os.LookupEnv(key); ok {
		return value, true
	}
	return "", false
}

// installGitGems installs gems from Git sources
func installGitGems(ctx context.Context, vendorDir string, gitSpecs []lockfile.GitGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(gitSpecs)}

	// Detect Ruby engine for extension compatibility filtering
	engine := ruby.DetectEngine()

	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return report, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	// Collect gems that need extensions built (defer until all gems installed)
	var extensionTargets []extensionTarget

	for _, spec := range gitSpecs {
		gemName := fmt.Sprintf("%s-%s", spec.Name, spec.Version)
		destDir := filepath.Join(vendorDir, "gems", gemName)

		// Smart skip logic
		if _, err := os.Stat(destDir); err == nil && !force {
			// If buildExtensions mode is enabled, check if this gem needs extension building
			if buildExtensions {
				needsBuild, err := extensions.NeedsBuild(destDir, engine)
				if err != nil {
					return report, fmt.Errorf("failed to check if %s needs extension build: %w", gemName, err)
				}
				if needsBuild {
					// Don't skip - this gem has extensions that need building
					extensionTargets = append(extensionTargets, extensionTarget{
						gemName: gemName,
						destDir: destDir,
					})
				}
			}
			report.Skipped++
			continue
		}

		if err := os.RemoveAll(destDir); err != nil {
			return report, fmt.Errorf("failed to clean install dir for %s: %w", gemName, err)
		}

		// Clone the git repo at the locked revision
		if err := cloneGitGem(spec, destDir); err != nil {
			return report, fmt.Errorf("failed to clone git gem %s: %w", spec.Name, err)
		}

		// Link binaries if any
		if err := geminstall.LinkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return report, err
		}

		// Collect this gem for extension building (defer until all gems installed)
		extensionTargets = append(extensionTargets, extensionTarget{
			gemName: gemName,
			destDir: destDir,
		})

		report.Installed++
	}

	// Build extensions for all installed gems (two-phase: install all, then build all)
	buildPendingExtensions(ctx, extBuilder, engine, extensionTargets, &report, extConfig, "", vendorDir)

	return report, nil
}

// cloneGitGem clones a git gem at the specified revision
func cloneGitGem(spec lockfile.GitGemSpec, destDir string) error {
	// Import the resolver package to use GitSource
	gitSource, err := resolver.NewGitSource(spec.Remote, spec.Branch, spec.Tag, spec.Revision)
	if err != nil {
		return fmt.Errorf("failed to create git source: %w", err)
	}

	// Clone at the locked revision
	if err := gitSource.CloneAtRevision(spec.Revision, destDir); err != nil {
		return fmt.Errorf("failed to clone at revision %s: %w", spec.Revision, err)
	}

	return nil
}

// installPathGems installs gems from local paths
func installPathGems(ctx context.Context, vendorDir string, pathSpecs []lockfile.PathGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(pathSpecs)}

	// Detect Ruby engine for extension compatibility filtering
	engine := ruby.DetectEngine()

	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return report, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	// Collect gems that need extensions built (defer until all gems installed)
	var extensionTargets []extensionTarget

	for _, spec := range pathSpecs {
		gemName := fmt.Sprintf("%s-%s", spec.Name, spec.Version)
		destDir := filepath.Join(vendorDir, "gems", gemName)

		// Smart skip logic
		if _, err := os.Stat(destDir); err == nil && !force {
			// If buildExtensions mode is enabled, check if this gem needs extension building
			if buildExtensions {
				needsBuild, err := extensions.NeedsBuild(destDir, engine)
				if err != nil {
					return report, fmt.Errorf("failed to check if %s needs extension build: %w", gemName, err)
				}
				if needsBuild {
					// Don't skip - this gem has extensions that need building
					extensionTargets = append(extensionTargets, extensionTarget{
						gemName: gemName,
						destDir: destDir,
					})
				}
			}
			report.Skipped++
			continue
		}

		if err := os.RemoveAll(destDir); err != nil {
			return report, fmt.Errorf("failed to clean install dir for %s: %w", gemName, err)
		}

		// Copy the path gem to vendor
		if err := copyPathGem(spec, destDir); err != nil {
			return report, fmt.Errorf("failed to copy path gem %s: %w", spec.Name, err)
		}

		// Link binaries if any
		if err := geminstall.LinkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return report, err
		}

		// Collect this gem for extension building (defer until all gems installed)
		extensionTargets = append(extensionTargets, extensionTarget{
			gemName: gemName,
			destDir: destDir,
		})

		report.Installed++
	}

	// Build extensions for all installed gems (two-phase: install all, then build all)
	buildPendingExtensions(ctx, extBuilder, engine, extensionTargets, &report, extConfig, "", vendorDir)

	return report, nil
}

// copyPathGem copies a path gem to the vendor directory
func copyPathGem(spec lockfile.PathGemSpec, destDir string) error {
	pathSource, err := resolver.NewPathSource(spec.Remote)
	if err != nil {
		return fmt.Errorf("failed to create path source: %w", err)
	}

	if err := pathSource.CopyToVendor(destDir); err != nil {
		return fmt.Errorf("failed to copy to vendor: %w", err)
	}

	return nil
}

// Helper for tests: create a minimal .gem archive.
func createFakeGemArchive(dest string, files map[string][]byte, marshalData []byte) error {
	var metadataBuf bytes.Buffer
	metaGz := gzip.NewWriter(&metadataBuf)
	if marshalData == nil {
		marshalData = []byte("placeholder metadata")
	}
	if _, err := metaGz.Write(marshalData); err != nil {
		return err
	}
	if err := metaGz.Close(); err != nil {
		return err
	}

	var dataBuffer bytes.Buffer
	dataGz := gzip.NewWriter(&dataBuffer)
	dataTw := tar.NewWriter(dataGz)

	for path, content := range files {
		if err := dataTw.WriteHeader(&tar.Header{
			Name: path,
			Mode: 0o755,
			Size: int64(len(content)),
		}); err != nil {
			return err
		}
		if _, err := dataTw.Write(content); err != nil {
			return err
		}
	}

	if err := dataTw.Close(); err != nil {
		return err
	}
	if err := dataGz.Close(); err != nil {
		return err
	}

	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	tw := tar.NewWriter(file)

	if err := tw.WriteHeader(&tar.Header{
		Name: "metadata.gz",
		Mode: 0o644,
		Size: int64(metadataBuf.Len()),
	}); err != nil {
		return err
	}
	if _, err := tw.Write(metadataBuf.Bytes()); err != nil {
		return err
	}

	if err := tw.WriteHeader(&tar.Header{
		Name: "data.tar.gz",
		Mode: 0o644,
		Size: int64(dataBuffer.Len()),
	}); err != nil {
		return err
	}
	if _, err := tw.Write(dataBuffer.Bytes()); err != nil {
		return err
	}

	return tw.Close()
}
