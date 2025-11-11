package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/logger"
	"github.com/contriboss/ore-light/internal/registry"
	"github.com/contriboss/ore-light/internal/sources"
)

// RunFetch implements the ore fetch command
// Downloads gems to cache without modifying lockfile (like `gem fetch`)
func RunFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	version := fs.String("version", "", "Gem version to fetch (default: latest)")
	platform := fs.String("platform", "", "Platform to fetch (e.g., x86_64-linux, java, ruby)")
	source := fs.String("source", "https://rubygems.org", "Gem source URL")

	if err := fs.Parse(args); err != nil {
		return err
	}

	gems := fs.Args()
	if len(gems) == 0 {
		return fmt.Errorf("at least one gem name is required")
	}

	// Get cache directory
	cacheDir, err := config.DefaultCacheDir(nil)
	if err != nil {
		return fmt.Errorf("failed to determine cache directory: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create registry client
	client, err := registry.NewClient("https://rubygems.org", registry.ProtocolRubygems)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	// Create source manager
	sourceManager := sources.NewManager([]sources.SourceConfig{
		{URL: *source, Fallback: ""},
	}, nil)

	ctx := context.Background()

	for _, gemName := range gems {
		if err := fetchGem(ctx, client, sourceManager, gemName, *version, *platform, cacheDir); err != nil {
			logger.Error("error fetching gem", "gem", gemName, "error", err)
			continue
		}
	}

	return nil
}

func fetchGem(ctx context.Context, client *registry.Client, sourceManager *sources.Manager, gemName, version, platform, cacheDir string) error {
	// Determine version to fetch
	targetVersion := version
	if targetVersion == "" {
		logger.Debug("finding latest version", "gem", gemName)
		versions, err := client.GetGemVersions(ctx, gemName)
		if err != nil {
			return fmt.Errorf("failed to get versions: %w", err)
		}
		if len(versions) == 0 {
			return fmt.Errorf("no versions found for gem %s", gemName)
		}
		targetVersion = versions[0]
	}

	// Determine platform
	targetPlatform := platform
	if targetPlatform == "" {
		targetPlatform = detectDefaultPlatform()
	}

	logger.Info("fetching gem", "gem", gemName, "version", targetVersion, "platform", targetPlatform)

	// Construct gem filename
	gemFileName := constructGemFilename(gemName, targetVersion, targetPlatform)

	// Check if already cached
	cachedPath := filepath.Join(cacheDir, gemFileName)
	if _, err := os.Stat(cachedPath); err == nil {
		fmt.Printf("✓ %s already cached at %s\n", gemFileName, cachedPath)
		return nil
	}

	// Create output file
	outFile, err := os.Create(cachedPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		_ = outFile.Close()
	}()

	// Download gem
	if err := sourceManager.DownloadGem(ctx, gemFileName, outFile); err != nil {
		// If platform-specific download fails, try platform-independent
		if targetPlatform != "ruby" {
			logger.Debug("platform-specific gem not found, trying platform-independent")
			gemFileName = constructGemFilename(gemName, targetVersion, "ruby")
			cachedPath = filepath.Join(cacheDir, gemFileName)

			// Check cache again
			if _, err := os.Stat(cachedPath); err == nil {
				fmt.Printf("✓ %s already cached at %s\n", gemFileName, cachedPath)
				return nil
			}

			// Close previous file and open new one
			_ = outFile.Close()
			outFile, err = os.Create(cachedPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer func() {
				_ = outFile.Close()
			}()

			if err := sourceManager.DownloadGem(ctx, gemFileName, outFile); err != nil {
				return fmt.Errorf("failed to download: %w", err)
			}
		} else {
			return fmt.Errorf("failed to download: %w", err)
		}
	}

	fmt.Printf("✓ Downloaded %s to %s\n", gemFileName, cachedPath)
	return nil
}

// constructGemFilename constructs the gem filename with platform suffix
func constructGemFilename(name, version, platform string) string {
	if platform == "" || platform == "ruby" {
		// Platform-independent gem
		return fmt.Sprintf("%s-%s.gem", name, version)
	}
	// Platform-specific gem
	return fmt.Sprintf("%s-%s-%s.gem", name, version, platform)
}

// detectDefaultPlatform detects the default platform for the current system
func detectDefaultPlatform() string {
	// Try to get from Ruby first
	// This is the same logic as in platform.go
	platform := os.Getenv("RUBY_PLATFORM")
	if platform != "" {
		return platform
	}

	// Fallback to Go runtime detection
	arch := runtime.GOARCH
	os := runtime.GOOS

	// Map Go arch to Ruby arch
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	case "386":
		arch = "x86"
	}

	// Map Go OS to Ruby OS
	switch os {
	case "darwin":
		os = "darwin"
	case "linux":
		os = "linux"
	case "windows":
		os = "mingw32"
	}

	return fmt.Sprintf("%s-%s", arch, os)
}
