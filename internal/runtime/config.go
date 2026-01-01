package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/contriboss/ore-light/internal/sources"
	toml "github.com/pelletier/go-toml/v2"
)

type SourceConfig = sources.SourceConfig

// Config holds ore-light runtime configuration
type Config struct {
	VendorDir  string         `toml:"vendor_dir"`
	CacheDir   string         `toml:"cache_dir"`
	GemSources []SourceConfig `toml:"gem_sources"`
	Gemfile    string         `toml:"gemfile"`
}

// LoadConfig loads ore-light configuration from user and project config files
// Priority: project .ore.toml > user ~/.config/ore/config.toml
func LoadConfig() (*Config, error) {
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

	merge(userConfigPath())
	merge(projectConfigPath())

	return cfg, nil
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
}

func userConfigPath() string {
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

func projectConfigPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, ".ore.toml")
}
