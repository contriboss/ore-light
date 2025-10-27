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
	"github.com/contriboss/ore-light/internal/extensions"
	"github.com/contriboss/ore-light/internal/geminstall"
	"github.com/contriboss/ore-light/internal/resolver"
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

func installFromCache(ctx context.Context, cacheDir, vendorDir string, gems []lockfile.GemSpec, force bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(gems)}

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

	for _, gem := range gems {
		gemPath := findGemInCaches(cacheDir, gem)
		if gemPath == "" {
			return report, fmt.Errorf("gem %s is not cached; run `ore download` first", gem.FullName())
		}

		destDir := filepath.Join(vendorDir, "gems", gem.FullName())

		if _, err := os.Stat(destDir); err == nil && !force {
			report.Skipped++
			continue
		}

		if err := os.RemoveAll(destDir); err != nil {
			return report, fmt.Errorf("failed to clean install dir for %s: %w", gem.FullName(), err)
		}

		metadata, err := geminstall.ExtractGemContents(gemPath, destDir)
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

		// Build extensions if present
		extResult, err := extBuilder.BuildExtensions(ctx, destDir, gem.FullName())
		if err != nil {
			// Extension build failure - warn but continue
			if extConfig != nil && !extConfig.SkipExtensions {
				fmt.Fprintf(os.Stderr, "Warning: Failed to build extensions for %s: %v\n", gem.FullName(), err)
				report.ExtensionsFailed++
			}
		} else if extResult.Skipped {
			report.ExtensionsSkipped++
		} else if extResult.Success && len(extResult.Extensions) > 0 {
			if extConfig != nil && extConfig.Verbose {
				fmt.Printf("Built %d extension(s) for %s: %v\n", len(extResult.Extensions), gem.FullName(), extResult.Extensions)
			}
			report.ExtensionsBuilt++
		}

		report.Installed++
	}

	return report, nil
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
	env = setEnv(env, "GEM_HOME", vendorDir)
	env = setEnv(env, "GEM_PATH", vendorDir)
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
func installGitGems(ctx context.Context, vendorDir string, gitSpecs []lockfile.GitGemSpec, force bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(gitSpecs)}

	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return report, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	for _, spec := range gitSpecs {
		gemName := fmt.Sprintf("%s-%s", spec.Name, spec.Version)
		destDir := filepath.Join(vendorDir, "gems", gemName)

		if _, err := os.Stat(destDir); err == nil && !force {
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

		// Build extensions if present
		extResult, err := extBuilder.BuildExtensions(ctx, destDir, gemName)
		if err != nil {
			if extConfig != nil && !extConfig.SkipExtensions {
				fmt.Fprintf(os.Stderr, "Warning: Failed to build extensions for %s: %v\n", gemName, err)
				report.ExtensionsFailed++
			}
		} else if extResult.Skipped {
			report.ExtensionsSkipped++
		} else if extResult.Success && len(extResult.Extensions) > 0 {
			if extConfig != nil && extConfig.Verbose {
				fmt.Printf("Built %d extension(s) for %s: %v\n", len(extResult.Extensions), gemName, extResult.Extensions)
			}
			report.ExtensionsBuilt++
		}

		report.Installed++
	}

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
func installPathGems(ctx context.Context, vendorDir string, pathSpecs []lockfile.PathGemSpec, force bool, extConfig *extensions.BuildConfig) (installReport, error) {
	report := installReport{Total: len(pathSpecs)}

	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return report, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	for _, spec := range pathSpecs {
		gemName := fmt.Sprintf("%s-%s", spec.Name, spec.Version)
		destDir := filepath.Join(vendorDir, "gems", gemName)

		if _, err := os.Stat(destDir); err == nil && !force {
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

		// Build extensions if present
		extResult, err := extBuilder.BuildExtensions(ctx, destDir, gemName)
		if err != nil {
			if extConfig != nil && !extConfig.SkipExtensions {
				fmt.Fprintf(os.Stderr, "Warning: Failed to build extensions for %s: %v\n", gemName, err)
				report.ExtensionsFailed++
			}
		} else if extResult.Skipped {
			report.ExtensionsSkipped++
		} else if extResult.Success && len(extResult.Extensions) > 0 {
			if extConfig != nil && extConfig.Verbose {
				fmt.Printf("Built %d extension(s) for %s: %v\n", len(extResult.Extensions), gemName, extResult.Extensions)
			}
			report.ExtensionsBuilt++
		}

		report.Installed++
	}

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
	defer file.Close()

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
