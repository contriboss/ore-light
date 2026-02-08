package commands

import (
	"context"

	"github.com/contriboss/gemfile-go/lockfile"
	"github.com/contriboss/ore-light/internal/extensions"
	"github.com/contriboss/ore-light/internal/runtime"
)

// downloadManagerAdapter adapts runtime.DownloadManager to commands.DownloadManager interface
type downloadManagerAdapter struct {
	dm *runtime.DownloadManager
}

func (a *downloadManagerAdapter) CheckSourceHealth(ctx context.Context) {
	a.dm.CheckSourceHealth(ctx)
}

func (a *downloadManagerAdapter) DownloadAll(ctx context.Context, gems []lockfile.GemSpec, force bool) (DownloadReport, error) {
	report, err := a.dm.DownloadAll(ctx, gems, force)
	if err != nil {
		return DownloadReport{}, err
	}
	return DownloadReport{
		Downloaded: report.Downloaded,
		Skipped:    report.Skipped,
	}, nil
}

func (a *downloadManagerAdapter) CacheDir() string {
	return a.dm.CacheDir()
}

// RunInstallDefault runs install with default ore-light behavior
func RunInstallDefault(args []string) error {
	cfg, err := runtime.LoadConfig()
	if err != nil {
		return err
	}
	callbacks := InstallCallbacks{
		GetDownloadManager: func(w int) (DownloadManager, error) {
			dm, err := runtime.NewDownloadManager(cfg, w)
			if err != nil {
				return nil, err
			}
			return &downloadManagerAdapter{dm: dm}, nil
		},
		GetDefaultVendorDir: func() string {
			return runtime.DefaultVendorDir(cfg)
		},
		InstallFromCache: func(ctx context.Context, cacheDir, vendorDir string, gems []lockfile.GemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error) {
			report, err := runtime.InstallFromCache(ctx, cacheDir, vendorDir, gems, force, buildExtensions, extConfig)
			if err != nil {
				return InstallReport{}, err
			}
			return InstallReport{
				Installed:        report.Installed,
				Skipped:          report.Skipped,
				ExtensionsBuilt:  report.ExtensionsBuilt,
				ExtensionsFailed: report.ExtensionsFailed,
			}, nil
		},
		InstallGitGems: func(ctx context.Context, vendorDir, rubyScope string, gitSpecs []lockfile.GitGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig) (InstallReport, error) {
			report, err := runtime.InstallGitGems(ctx, vendorDir, rubyScope, gitSpecs, force, buildExtensions, extConfig)
			if err != nil {
				return InstallReport{}, err
			}
			return InstallReport{
				Installed:        report.Installed,
				Skipped:          report.Skipped,
				ExtensionsBuilt:  report.ExtensionsBuilt,
				ExtensionsFailed: report.ExtensionsFailed,
			}, nil
		},
		InstallPathGems: func(ctx context.Context, vendorDir, rubyScope string, pathSpecs []lockfile.PathGemSpec, force bool, buildExtensions bool, extConfig *extensions.BuildConfig, lockfileDir string) (InstallReport, error) {
			report, err := runtime.InstallPathGems(ctx, vendorDir, rubyScope, pathSpecs, force, buildExtensions, extConfig, lockfileDir)
			if err != nil {
				return InstallReport{}, err
			}
			return InstallReport{
				Installed:        report.Installed,
				Skipped:          report.Skipped,
				ExtensionsBuilt:  report.ExtensionsBuilt,
				ExtensionsFailed: report.ExtensionsFailed,
			}, nil
		},
	}
	return RunInstall(args, callbacks)
}

// RunExecDefault runs exec with default ore-light behavior
func RunExecDefault(args []string) error {
	cfg, err := runtime.LoadConfig()
	if err != nil {
		return err
	}
	return RunExec(args, runtime.NewBuildEnv(cfg))
}

// RunSearchDefault runs search with default ore-light behavior
func RunSearchDefault(args []string) error {
	cfg, err := runtime.LoadConfig()
	if err != nil {
		return err
	}
	return RunSearch(args, runtime.NewSearchSources(cfg))
}
