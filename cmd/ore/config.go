package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	VendorDir string `toml:"vendor_dir"`
	CacheDir  string `toml:"cache_dir"`
	GemMirror string `toml:"gem_mirror"`
	Gemfile   string `toml:"gemfile"`
}

var appConfig = loadConfig()

func loadConfig() *Config {
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

	return cfg
}

func (c *Config) merge(other Config) {
	if other.VendorDir != "" {
		c.VendorDir = other.VendorDir
	}
	if other.CacheDir != "" {
		c.CacheDir = other.CacheDir
	}
	if other.GemMirror != "" {
		c.GemMirror = other.GemMirror
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
