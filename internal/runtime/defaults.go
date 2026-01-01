package runtime

import (
	"github.com/contriboss/ore-light/cmd/ore/commands"
)

// RunInstallDefault runs install with default ore-light behavior
func RunInstallDefault(args []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	callbacks := commands.InstallCallbacks{
		GetDownloadManager: func(w int) (commands.DownloadManager, error) {
			return NewDownloadManager(cfg, w)
		},
		GetDefaultVendorDir: func() string {
			return DefaultVendorDir(cfg)
		},
		InstallFromCache: InstallFromCache,
		InstallGitGems:   InstallGitGems,
		InstallPathGems:  InstallPathGems,
	}
	return commands.RunInstall(args, callbacks)
}

// RunExecDefault runs exec with default ore-light behavior
func RunExecDefault(args []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	return commands.RunExec(args, NewBuildEnv(cfg))
}

// RunSearchDefault runs search with default ore-light behavior
func RunSearchDefault(args []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	return commands.RunSearch(args, NewSearchSources(cfg))
}
