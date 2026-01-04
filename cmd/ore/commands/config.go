package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/contriboss/ore-light/internal/config"
	"github.com/contriboss/ore-light/internal/sources"
	"gopkg.in/yaml.v3"
)

// RunConfig implements the ore config command
func RunConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	local := fs.Bool("local", false, "Set local config (project-level)")
	global := fs.Bool("global", false, "Set global config (user-level)")
	unset := fs.Bool("unset", false, "Unset a configuration value")
	list := fs.Bool("list", false, "List all configuration settings")
	show := fs.Bool("show", false, "Show effective ore config")
	explain := fs.Bool("explain", false, "Explain effective ore config sources")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// If no flags, show usage
	if !*local && !*global && !*unset && !*list && !*show && !*explain && len(fs.Args()) == 0 {
		return showConfigUsage()
	}

	if *show || *explain {
		return showEffectiveConfig(*explain)
	}

	// List all configs
	if *list {
		return listConfigs(*local, *global)
	}

	// Get/Set/Unset config
	configArgs := fs.Args()

	// Determine scope (default to local if neither specified)
	scope := "local"
	if *global {
		scope = "global"
	}

	// Unset a config value
	if *unset {
		if len(configArgs) != 1 {
			return fmt.Errorf("usage: ore config --unset [--local|--global] <key>")
		}
		return unsetConfig(scope, configArgs[0])
	}

	// Get a config value
	if len(configArgs) == 1 {
		return getConfig(scope, configArgs[0])
	}

	// Set a config value
	if len(configArgs) == 2 {
		return setConfig(scope, configArgs[0], configArgs[1])
	}

	return fmt.Errorf("usage: ore config [--local|--global] <key> [<value>]")
}

func showConfigUsage() error {
	fmt.Print(`Usage: ore config [options] <key> [<value>]

Options:
  --local     Use local config (.bundle/config)
  --global    Use global config (~/.bundle/config)
  --list      List all configuration
  --show      Show effective ore config
  --explain   Show effective ore config with sources
  --unset     Remove configuration value

Examples:
  ore config --local path vendor/bundle    # Set local install path
  ore config path                          # Get install path
  ore config --list                        # List all settings
  ore config --show                        # Show effective ore config
  ore config --unset --local path          # Remove local path setting

Supported keys:
  path        Installation directory for gems
`)
	return nil
}

func listConfigs(localOnly, globalOnly bool) error {
	configs := make(map[string]string)

	// Load global config
	if !localOnly {
		globalConfig := getConfigPath("global")
		if data, err := os.ReadFile(globalConfig); err == nil {
			var config map[string]interface{}
			if err := yaml.Unmarshal(data, &config); err == nil {
				for k, v := range config {
					if str, ok := v.(string); ok {
						configs[k+" (global)"] = str
					}
				}
			}
		}
	}

	// Load local config
	if !globalOnly {
		localConfig := getConfigPath("local")
		if data, err := os.ReadFile(localConfig); err == nil {
			var config map[string]interface{}
			if err := yaml.Unmarshal(data, &config); err == nil {
				for k, v := range config {
					if str, ok := v.(string); ok {
						configs[k+" (local)"] = str
					}
				}
			}
		}
	}

	if len(configs) == 0 {
		fmt.Println("No configuration set")
		return nil
	}

	for k, v := range configs {
		fmt.Printf("%s: %s\n", k, v)
	}

	return nil
}

func getConfig(scope, key string) error {
	configPath := getConfigPath(scope)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("no value set for %s", key)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Convert key to Bundler format (e.g., "path" -> "BUNDLE_PATH")
	bundleKey := toBundleKey(key)

	if value, ok := config[bundleKey].(string); ok {
		fmt.Println(value)
		return nil
	}

	return fmt.Errorf("no value set for %s", key)
}

func setConfig(scope, key, value string) error {
	configPath := getConfigPath(scope)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load existing config or create new
	config := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		_ = yaml.Unmarshal(data, &config)
	}

	// Convert key to Bundler format (e.g., "path" -> "BUNDLE_PATH")
	bundleKey := toBundleKey(key)
	config[bundleKey] = value

	// Write config
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("Set %s to %s (%s)\n", key, value, scope)
	return nil
}

func unsetConfig(scope, key string) error {
	configPath := getConfigPath(scope)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("no config file found")
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	bundleKey := toBundleKey(key)
	if _, ok := config[bundleKey]; !ok {
		return fmt.Errorf("no value set for %s", key)
	}

	delete(config, bundleKey)

	// Write updated config
	data, err = yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("Unset %s (%s)\n", key, scope)
	return nil
}

func getConfigPath(scope string) string {
	if scope == "global" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".bundle", "config")
	}
	return filepath.Join(".bundle", "config")
}

// toBundleKey converts user-friendly keys to Bundler config keys
func toBundleKey(key string) string {
	// Common keys
	switch strings.ToLower(key) {
	case "path":
		return "BUNDLE_PATH"
	case "jobs":
		return "BUNDLE_JOBS"
	case "retry":
		return "BUNDLE_RETRY"
	default:
		// If already in BUNDLE_ format, use as-is
		if strings.HasPrefix(strings.ToUpper(key), "BUNDLE_") {
			return strings.ToUpper(key)
		}
		// Otherwise, prefix with BUNDLE_
		return "BUNDLE_" + strings.ToUpper(key)
	}
}

func showEffectiveConfig(explain bool) error {
	cfg := config.Load()
	vendorDir, vendorSource, _ := config.ResolveVendorDir(cfg, getRubyVersion, getSystemGemDir)
	cacheDir, cacheSource, cacheErr := config.ResolveCacheDir(cfg)
	gemfile, gemfileSource, _ := config.ResolveGemfilePath(cfg)
	gemSources, gemSourcesSource := config.ResolveGemSources(cfg)

	fmt.Println("Effective ore config:")
	if explain {
		fmt.Printf("  vendor_dir = %s (%s)\n", vendorDir, vendorSource)
		if cacheErr != nil {
			fmt.Printf("  cache_dir = <error: %v> (%s)\n", cacheErr, cacheSource)
		} else {
			fmt.Printf("  cache_dir = %s (%s)\n", cacheDir, cacheSource)
		}
		fmt.Printf("  gemfile = %s (%s)\n", gemfile, gemfileSource)
		fmt.Printf("  gem_sources = %s (%s)\n", formatSources(gemSources), gemSourcesSource)
	} else {
		fmt.Printf("  vendor_dir = %s\n", vendorDir)
		if cacheErr != nil {
			fmt.Printf("  cache_dir = <error: %v>\n", cacheErr)
		} else {
			fmt.Printf("  cache_dir = %s\n", cacheDir)
		}
		fmt.Printf("  gemfile = %s\n", gemfile)
		fmt.Printf("  gem_sources = %s\n", formatSources(gemSources))
	}

	if explain {
		fmt.Println("")
		fmt.Println("Config files:")
		printConfigPath("  user", config.UserConfigPath())
		printConfigPath("  project", config.ProjectConfigPath())
	}

	return nil
}

func printConfigPath(label, path string) {
	if path == "" {
		fmt.Printf("%s: <unavailable>\n", label)
		return
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("%s: %s (loaded)\n", label, path)
		return
	}
	fmt.Printf("%s: %s (missing)\n", label, path)
}

func formatSources(sources []sources.SourceConfig) string {
	if len(sources) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(sources))
	for _, src := range sources {
		if src.Fallback != "" {
			parts = append(parts, fmt.Sprintf("%s (fallback %s)", src.URL, src.Fallback))
		} else {
			parts = append(parts, src.URL)
		}
	}
	return strings.Join(parts, ", ")
}
