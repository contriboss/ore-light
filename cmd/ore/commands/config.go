package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RunConfig implements the ore config command
func RunConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	local := fs.Bool("local", false, "Set local config (project-level)")
	global := fs.Bool("global", false, "Set global config (user-level)")
	unset := fs.Bool("unset", false, "Unset a configuration value")
	list := fs.Bool("list", false, "List all configuration settings")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// If no flags, show usage
	if !*local && !*global && !*unset && !*list && len(fs.Args()) == 0 {
		return showConfigUsage()
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
  --unset     Remove configuration value

Examples:
  ore config --local path vendor/bundle    # Set local install path
  ore config path                          # Get install path
  ore config --list                        # List all settings
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
