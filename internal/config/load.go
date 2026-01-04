package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Load reads ore-light configuration from user and project config files.
// Priority: user config first, then project config overrides.
func Load() *Config {
	cfg := &Config{}

	merge := func(path string) {
		if path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return
			}
			fmt.Fprintf(os.Stderr, "warning: unable to read config %s: %v\n", path, err)
			return
		}

		var fileCfg Config
		if err := toml.Unmarshal(data, &fileCfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: unable to parse config %s: %v\n", path, err)
			return
		}

		cfg.merge(fileCfg)
	}

	merge(UserConfigPath())
	merge(ProjectConfigPath())

	return cfg
}

func (c *Config) merge(other Config) {
	if other.VendorDir != "" {
		c.VendorDir = other.VendorDir
	}
	if other.CacheDir != "" {
		c.CacheDir = other.CacheDir
	}
	if len(other.GemSources) > 0 {
		c.GemSources = other.GemSources
	}
	if other.Gemfile != "" {
		c.Gemfile = other.Gemfile
	}
	if other.DownloadWorkers > 0 {
		c.DownloadWorkers = other.DownloadWorkers
	}
}

// UserConfigPath returns the user-level config path.
func UserConfigPath() string {
	if path := os.Getenv("ORE_CONFIG"); path != "" {
		return path
	}

	var base string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		base = xdg
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}

	return filepath.Join(base, "ore", "config.toml")
}

// ProjectConfigPath returns the project-level config path.
func ProjectConfigPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, ".ore.toml")
}
