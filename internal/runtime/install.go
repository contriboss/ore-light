package runtime

import (
	"context"
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
	"github.com/contriboss/ore-light/internal/sources"
)

// InstallReport tracks installation progress and results
type InstallReport struct {
	Total             int
	Installed         int
	Skipped           int
	ExtensionsBuilt   int
	ExtensionsSkipped int
	ExtensionsFailed  int
}

type extensionTarget struct {
	gemName string
	destDir string
}

// InstallFromCache installs gems from the cache directory
func InstallFromCache(ctx context.Context, cacheDir, vendorDir string, gems []lockfile.GemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error) {
	report := InstallReport{Total: len(gems)}

	engine := ruby.DetectEngine()
	engineChecker := resolver.NewEngineCompatibility(engine)

	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return InstallReport{}, err
	}
	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "cache")); err != nil {
		return InstallReport{}, err
	}
	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "bin")); err != nil {
		return InstallReport{}, err
	}
	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "specifications", "cache")); err != nil {
		return InstallReport{}, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	var extensionTargets []extensionTarget

	for _, gem := range gems {
		gemPath := findGemInCaches(cacheDir, gem)
		if gemPath == "" {
			return InstallReport{}, fmt.Errorf("gem %s is not cached; run `ore fetch` first", gem.FullName())
		}

		destDir := filepath.Join(vendorDir, "gems", gem.FullName())

		if _, err := os.Stat(destDir); err == nil && !force {
			if buildExtensions {
				needsBuild, err := extensions.NeedsBuild(destDir, engine)
				if err != nil {
					return InstallReport{}, fmt.Errorf("failed to check if %s needs extension build: %w", gem.FullName(), err)
				}
				if needsBuild {
					extensionTargets = append(extensionTargets, extensionTarget{
						gemName: gem.FullName(),
						destDir: destDir,
					})
				}
			}
			report.Skipped++
			continue
		}

		metadata, err := geminstall.ExtractMetadataOnly(gemPath)
		if err != nil {
			return InstallReport{}, fmt.Errorf("failed to extract metadata from %s: %w", gem.FullName(), err)
		}

		if len(metadata) > 0 {
			gemWithExtensions := gem
			extensions, err := geminstall.ParseExtensionsFromMetadata(metadata)
			if err != nil {
				if extConfig != nil && extConfig.Verbose {
					fmt.Fprintf(os.Stderr, "⚠️  Warning: %s metadata parse error: %v (assuming native extensions)\n", gem.FullName(), err)
				}
				gemWithExtensions.Extensions = []string{"ext/extconf.rb"}
			} else if len(extensions) > 0 {
				gemWithExtensions.Extensions = extensions
			}

			if !engineChecker.IsCompatible(gemWithExtensions) {
				reason := engineChecker.GetIncompatibilityReason(gemWithExtensions)
				if extConfig != nil && extConfig.Verbose {
					fmt.Printf("⚠️  Skipping %s: %s\n", gem.FullName(), reason)
				}
				report.Skipped++
				continue
			}
		}

		if err := os.RemoveAll(destDir); err != nil {
			return InstallReport{}, fmt.Errorf("failed to clean install dir for %s: %w", gem.FullName(), err)
		}

		_, err = geminstall.ExtractGemContents(gemPath, destDir)
		if err != nil {
			return InstallReport{}, fmt.Errorf("failed to extract %s: %w", gem.FullName(), err)
		}

		if err := geminstall.CopyGemToVendorCache(gemPath, filepath.Join(vendorDir, "cache", gemFileName(gem))); err != nil {
			return InstallReport{}, err
		}

		if len(metadata) > 0 {
			if err := geminstall.WriteGemSpecification(vendorDir, gem, metadata); err != nil {
				return InstallReport{}, err
			}
		}

		if err := geminstall.LinkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return InstallReport{}, err
		}

		extensionTargets = append(extensionTargets, extensionTarget{
			gemName: gem.FullName(),
			destDir: destDir,
		})

		report.Installed++
	}

	if extConfig != nil && extConfig.Verbose {
		fmt.Printf("Building extensions for %d gems after all installations complete...\n", len(extensionTargets))
	}
	buildPendingExtensions(ctx, extBuilder, engine, extensionTargets, &report, extConfig, cacheDir, vendorDir)

	return InstallReport{
		Installed:        report.Installed,
		Skipped:          report.Skipped,
		ExtensionsBuilt:  report.ExtensionsBuilt,
		ExtensionsFailed: report.ExtensionsFailed,
	}, nil
}

// InstallGitGems installs gems from git sources
func InstallGitGems(ctx context.Context, vendorDir string, gitSpecs []lockfile.GitGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error) {
	report := InstallReport{Total: len(gitSpecs)}

	engine := ruby.DetectEngine()

	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return InstallReport{}, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	var extensionTargets []extensionTarget

	for _, spec := range gitSpecs {
		gemName := fmt.Sprintf("%s-%s", spec.Name, spec.Version)
		destDir := filepath.Join(vendorDir, "gems", gemName)

		if _, err := os.Stat(destDir); err == nil && !force {
			if buildExtensions {
				needsBuild, err := extensions.NeedsBuild(destDir, engine)
				if err != nil {
					return InstallReport{}, fmt.Errorf("failed to check if %s needs extension build: %w", gemName, err)
				}
				if needsBuild {
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
			return InstallReport{}, fmt.Errorf("failed to clean install dir for %s: %w", gemName, err)
		}

		if err := cloneGitGem(spec, destDir); err != nil {
			return InstallReport{}, fmt.Errorf("failed to clone git gem %s: %w", spec.Name, err)
		}

		if err := geminstall.LinkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return InstallReport{}, err
		}

		extensionTargets = append(extensionTargets, extensionTarget{
			gemName: gemName,
			destDir: destDir,
		})

		report.Installed++
	}

	buildPendingExtensions(ctx, extBuilder, engine, extensionTargets, &report, extConfig, "", vendorDir)

	return InstallReport{
		Installed:        report.Installed,
		Skipped:          report.Skipped,
		ExtensionsBuilt:  report.ExtensionsBuilt,
		ExtensionsFailed: report.ExtensionsFailed,
	}, nil
}

// InstallPathGems installs gems from local paths
func InstallPathGems(ctx context.Context, vendorDir string, pathSpecs []lockfile.PathGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error) {
	report := InstallReport{Total: len(pathSpecs)}

	engine := ruby.DetectEngine()

	if err := geminstall.EnsureDir(filepath.Join(vendorDir, "gems")); err != nil {
		return InstallReport{}, err
	}

	extBuilder := extensions.NewBuilder(extConfig)

	var extensionTargets []extensionTarget

	for _, spec := range pathSpecs {
		gemName := fmt.Sprintf("%s-%s", spec.Name, spec.Version)
		destDir := filepath.Join(vendorDir, "gems", gemName)

		if _, err := os.Stat(destDir); err == nil && !force {
			if buildExtensions {
				needsBuild, err := extensions.NeedsBuild(destDir, engine)
				if err != nil {
					return InstallReport{}, fmt.Errorf("failed to check if %s needs extension build: %w", gemName, err)
				}
				if needsBuild {
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
			return InstallReport{}, fmt.Errorf("failed to clean install dir for %s: %w", gemName, err)
		}

		if err := copyPathGem(spec, destDir); err != nil {
			return InstallReport{}, fmt.Errorf("failed to copy path gem %s: %w", spec.Name, err)
		}

		if err := geminstall.LinkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
			return InstallReport{}, err
		}

		extensionTargets = append(extensionTargets, extensionTarget{
			gemName: gemName,
			destDir: destDir,
		})

		report.Installed++
	}

	buildPendingExtensions(ctx, extBuilder, engine, extensionTargets, &report, extConfig, "", vendorDir)

	return InstallReport{
		Installed:        report.Installed,
		Skipped:          report.Skipped,
		ExtensionsBuilt:  report.ExtensionsBuilt,
		ExtensionsFailed: report.ExtensionsFailed,
	}, nil
}

func buildPendingExtensions(ctx context.Context, extBuilder *extensions.Builder, engine ruby.Engine, targets []extensionTarget, report *InstallReport, extConfig *extensions.BuildConfig, cacheDir, vendorDir string) {
	if extConfig == nil || extConfig.SkipExtensions {
		return
	}

	for _, target := range targets {
		extResult, err := extBuilder.BuildExtensions(ctx, target.destDir, target.gemName, engine)

		if (err != nil || !extResult.Success) && extResult != nil && len(extResult.MissingDependencies) > 0 {
			if extConfig.Verbose {
				fmt.Printf("Extension build for %s requires: %v\n", target.gemName, extResult.MissingDependencies)
			}

			actualCacheDir := cacheDir
			if actualCacheDir == "" {
				var configErr error
				actualCacheDir, configErr = config.DefaultCacheDir(nil)
				if configErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to determine cache directory: %v\n", configErr)
					report.ExtensionsFailed++
					continue
				}
			}

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

			if extConfig.Verbose {
				fmt.Printf("Retrying extension build for %s...\n", target.gemName)
			}
			extResult, err = extBuilder.BuildExtensions(ctx, target.destDir, target.gemName, engine)
		}

		if err != nil || (extResult != nil && !extResult.Success) {
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

func installBuildDependency(ctx context.Context, gemName, cacheDir, vendorDir string, verbose bool) error {
	client, err := registry.NewClient("https://rubygems.org", registry.ProtocolRubygems)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	versions, err := client.GetGemVersions(ctx, gemName)
	if err != nil {
		return fmt.Errorf("failed to get versions for %s: %w", gemName, err)
	}
	if len(versions) == 0 {
		return fmt.Errorf("no versions found for %s", gemName)
	}
	targetVersion := versions[0]

	gemFileName := fmt.Sprintf("%s-%s.gem", gemName, targetVersion)
	cachedPath := filepath.Join(cacheDir, gemFileName)

	if _, err := os.Stat(cachedPath); err != nil {
		if verbose {
			fmt.Printf("📦 Fetching build dependency %s-%s...\n", gemName, targetVersion)
		}

		sourceManager := sources.NewManager([]SourceConfig{
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

	gemSpec := lockfile.GemSpec{
		Name:    gemName,
		Version: targetVersion,
	}
	destDir := filepath.Join(vendorDir, "gems", gemSpec.FullName())

	metadata, err := geminstall.ExtractGemContents(cachedPath, destDir)
	if err != nil {
		return fmt.Errorf("failed to extract %s: %w", gemName, err)
	}

	if len(metadata) > 0 {
		if err := geminstall.WriteGemSpecification(vendorDir, gemSpec, metadata); err != nil {
			return fmt.Errorf("failed to write gemspec for %s: %w", gemName, err)
		}
	}

	if err := geminstall.LinkGemBinaries(destDir, filepath.Join(vendorDir, "bin")); err != nil {
		return fmt.Errorf("failed to link binaries for %s: %w", gemName, err)
	}

	if verbose {
		fmt.Printf("✓ Installed build dependency %s-%s\n", gemName, targetVersion)
	}

	return nil
}

func findGemInCaches(primaryCache string, gem lockfile.GemSpec) string {
	fileName := gemFileName(gem)

	path := filepath.Join(primaryCache, fileName)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	if gemPaths := tryGetGemPaths(makeDetectRubyVersion(nil)); len(gemPaths) > 0 {
		for _, gemPath := range gemPaths {
			systemCache := filepath.Join(gemPath, "cache", fileName)
			if _, err := os.Stat(systemCache); err == nil {
				return systemCache
			}
		}
	}

	return ""
}

func cloneGitGem(spec lockfile.GitGemSpec, destDir string) error {
	gitSource, err := resolver.NewGitSource(spec.Remote, spec.Branch, spec.Tag, spec.Revision)
	if err != nil {
		return fmt.Errorf("failed to create git source: %w", err)
	}

	if err := gitSource.CloneAtRevision(spec.Revision, destDir); err != nil {
		return fmt.Errorf("failed to clone at revision %s: %w", spec.Revision, err)
	}

	return nil
}

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

// DefaultVendorDir returns the default vendor directory for the given config
func DefaultVendorDir(cfg *Config) string {
	return config.DefaultVendorDir(configAdapter(cfg), makeDetectRubyVersion(cfg), func() string {
		return getSystemGemDir(cfg)
	})
}

func getSystemGemDir(cfg *Config) string {
	return ruby.GetSystemGemDir(makeDetectRubyVersion(cfg))
}
